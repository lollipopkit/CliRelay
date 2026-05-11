package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	log "github.com/sirupsen/logrus"
)

const (
	backupNamePrefix = "clirelay-backup-"
	backupTimeLayout = "20060102T150405"
	backupExt        = ".tar.gz"
	backupIndexName  = "backups.json"
)

var backupNameRE = regexp.MustCompile(`^clirelay-backup-\d{8}T\d{6}\.tar\.gz$`)

type Manager struct {
	mu         sync.Mutex
	configDir  string
	dbPath     string
	authDir    string // resolved auth directory (empty if not configured)
	backupDir  string
	cfg        config.BackupConfig
	storageCfg config.RequestLogStorageConfig
	loc        *time.Location
}

// NewManager creates a backup manager. authDir is the resolved auth directory path
// (may be empty if auth-dir is not configured or include-auth-files is not needed).
func NewManager(configDir, dbPath, authDir string, cfg config.BackupConfig) (*Manager, error) {
	backupDir := cfg.Dir
	if backupDir == "" {
		backupDir = "backups"
	}
	if !filepath.IsAbs(backupDir) {
		backupDir = filepath.Join(configDir, backupDir)
	}
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return nil, fmt.Errorf("backup: create dir %s: %w", backupDir, err)
	}
	return &Manager{configDir: configDir, dbPath: dbPath, authDir: authDir, backupDir: backupDir, cfg: cfg}, nil
}

func (m *Manager) BackupDir() string { return m.backupDir }

func (m *Manager) SetDBOptions(storageCfg config.RequestLogStorageConfig, loc *time.Location) {
	m.storageCfg = storageCfg
	m.loc = loc
}

func (m *Manager) BackupPath(name string) (string, error) {
	if err := validateBackupName(name); err != nil {
		return "", err
	}
	return filepath.Join(m.backupDir, name), nil
}

func (m *Manager) CreateBackup(ctx context.Context, includeAuth bool) (string, error) {
	return m.CreateBackupWithSource(ctx, includeAuth, "manual")
}

func (m *Manager) CreateBackupWithSource(ctx context.Context, includeAuth bool, source string) (string, error) {
	if source != "cron" {
		source = "manual"
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ensureDB(); err != nil {
		return "", err
	}
	now := time.Now()
	name := backupNamePrefix + now.Format(backupTimeLayout) + backupExt
	dstPath := filepath.Join(m.backupDir, name)
	for i := 0; i < 3; i++ {
		if _, err := os.Stat(dstPath); os.IsNotExist(err) {
			break
		}
		time.Sleep(time.Second)
		now = time.Now()
		name = backupNamePrefix + now.Format(backupTimeLayout) + backupExt
		dstPath = filepath.Join(m.backupDir, name)
	}
	if _, err := os.Stat(dstPath); err == nil {
		return "", fmt.Errorf("backup: file %s already exists", dstPath)
	}
	log.Info("backup: running WAL checkpoint")
	if err := m.walCheckpoint(ctx); err != nil {
		log.Warnf("backup: WAL checkpoint failed (continuing): %v", err)
	}
	tmpDir, err := os.MkdirTemp("", "clirelay-backup-*")
	if err != nil {
		return "", fmt.Errorf("backup: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	tmpDB := filepath.Join(tmpDir, "usage.db")
	log.Info("backup: running VACUUM INTO for SQLite safe copy")
	if err := m.vacuumIntoDB(ctx, tmpDB); err != nil {
		return "", fmt.Errorf("backup: vacuum into: %w", err)
	}
	configSrc := filepath.Join(m.configDir, "config.yaml")
	tmpConfig := filepath.Join(tmpDir, "config.yaml")
	if err := copyFile(configSrc, tmpConfig); err != nil {
		log.Warnf("backup: copy config.yaml failed (continuing): %v", err)
	}
	// Include auth directory files if requested and configured.
	if includeAuth && m.authDir != "" {
		authDst := filepath.Join(tmpDir, "auth")
		if err := copyDir(m.authDir, authDst); err != nil {
			log.Warnf("backup: copy auth-dir failed (continuing): %v", err)
		} else {
			log.Infof("backup: included auth-dir from %s", m.authDir)
		}
	}
	log.Infof("backup: creating archive %s", dstPath)
	if err := tarGzDir(tmpDir, dstPath); err != nil {
		return "", fmt.Errorf("backup: tar.gz: %w", err)
	}
	fi, err := os.Stat(dstPath)
	if err != nil {
		return "", fmt.Errorf("backup: stat archive: %w", err)
	}
	log.Infof("backup: created %s (%d bytes)", name, fi.Size())
	if err := m.recordSource(name, source); err != nil {
		log.Warnf("backup: failed to update backup index: %v", err)
	}
	if err := m.pruneRetention(); err != nil {
		log.Warnf("backup: retention prune failed: %v", err)
	}
	return name, nil
}

func (m *Manager) ListBackups() ([]BackupInfo, error) {
	entries, err := os.ReadDir(m.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("backup: read dir: %w", err)
	}
	index := m.loadIndex()
	var backups []BackupInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, backupNamePrefix) || !strings.HasSuffix(name, backupExt) {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		createdAt, source := parseBackupName(name)
		if meta, ok := index[name]; ok && meta.Source != "" {
			source = meta.Source
		}
		backups = append(backups, BackupInfo{
			Name: name, Path: filepath.Join(m.backupDir, name),
			Size: fi.Size(), CreatedAt: createdAt, Source: source,
		})
	}
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})
	return backups, nil
}

func (m *Manager) DeleteBackup(name string) error {
	if err := validateBackupName(name); err != nil {
		return err
	}
	p := filepath.Join(m.backupDir, name)
	if err := os.Remove(p); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("backup: %s not found", name)
		}
		return fmt.Errorf("backup: remove: %w", err)
	}
	_ = m.removeFromIndex(name)
	log.Infof("backup: deleted %s", name)
	return nil
}

// SaveUploadedBackup saves an uploaded backup file to the backup directory and
// returns its sanitized storage name. Client-supplied filenames are never used
// as paths to avoid path traversal.
func (m *Manager) SaveUploadedBackup(filename string, src io.Reader) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !strings.HasSuffix(strings.ToLower(filename), ".tar.gz") {
		return "", fmt.Errorf("backup: invalid upload filename %q", filename)
	}
	var name, dstPath string
	var f *os.File
	var err error
	for i := 0; i < 3; i++ {
		name = backupNamePrefix + time.Now().Format(backupTimeLayout) + backupExt
		dstPath = filepath.Join(m.backupDir, name)
		f, err = os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			break
		}
		if !os.IsExist(err) {
			return "", fmt.Errorf("backup: create file: %w", err)
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		return "", fmt.Errorf("backup: create file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, src); err != nil {
		return "", fmt.Errorf("backup: write file: %w", err)
	}
	_ = m.recordSource(name, "manual")
	log.Infof("backup: uploaded %s as %s", filepath.Base(filename), name)
	return name, nil
}

func (m *Manager) RestoreFromBackup(name string, restoreConfig bool) error {
	if err := validateBackupName(name); err != nil {
		return err
	}
	srcPath := filepath.Join(m.backupDir, name)
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return fmt.Errorf("backup: %s not found", name)
	}
	log.Info("backup: creating pre-restore safety backup")
	if _, err := m.CreateBackup(context.Background(), false); err != nil {
		log.Warnf("backup: pre-restore safety backup failed (continuing): %v", err)
	}
	tmpDir, err := os.MkdirTemp("", "clirelay-restore-*")
	if err != nil {
		return fmt.Errorf("backup: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	log.Infof("backup: extracting %s to %s", name, tmpDir)
	if err := unTarGz(srcPath, tmpDir); err != nil {
		return fmt.Errorf("backup: extract: %w", err)
	}
	log.Info("backup: closing current SQLite connection")
	usage.CloseDB()
	restoredDB := filepath.Join(tmpDir, "usage.db")
	restoredConfig := filepath.Join(tmpDir, "config.yaml")
	if _, err := os.Stat(restoredDB); err == nil {
		for _, suffix := range []string{"", "-wal", "-shm"} {
			_ = os.Remove(m.dbPath + suffix)
		}
		if err := copyFile(restoredDB, m.dbPath); err != nil {
			return fmt.Errorf("backup: restore db: %w", err)
		}
		log.Infof("backup: restored %s", m.dbPath)
	} else {
		log.Warn("backup: no usage.db found in backup archive")
	}
	if restoreConfig {
		if _, err := os.Stat(restoredConfig); err == nil {
			configDst := filepath.Join(m.configDir, "config.yaml")
			if err := copyFile(restoredConfig, configDst); err != nil {
				return fmt.Errorf("backup: restore config: %w", err)
			}
			log.Infof("backup: restored %s", configDst)
		} else {
			log.Warn("backup: no config.yaml found in backup archive")
		}
	}
	// Restore auth directory if present in backup.
	restoredAuth := filepath.Join(tmpDir, "auth")
	if fi, err := os.Stat(restoredAuth); err == nil && fi.IsDir() {
		if m.authDir != "" {
			if err := os.RemoveAll(m.authDir); err != nil {
				log.Warnf("backup: remove old auth-dir failed: %v", err)
			}
			if err := copyDir(restoredAuth, m.authDir); err != nil {
				return fmt.Errorf("backup: restore auth-dir: %w", err)
			}
			log.Infof("backup: restored auth-dir to %s", m.authDir)
		} else {
			log.Warn("backup: auth files found in archive but auth-dir not configured, skipping")
		}
	}
	if err := usage.InitDB(m.dbPath, m.storageCfg, m.loc); err != nil {
		log.Warnf("backup: reopen database failed; service restart required: %v", err)
		return fmt.Errorf("backup: reopen database: %w", err)
	}
	log.Info("backup: database reopened after restore; service restart is still recommended")
	return nil
}

func (m *Manager) ensureDB() error {
	if usage.GetDB() != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(m.dbPath), 0755); err != nil {
		return fmt.Errorf("backup: create data dir: %w", err)
	}
	if err := usage.InitDB(m.dbPath, m.storageCfg, m.loc); err != nil {
		return fmt.Errorf("backup: initialize database: %w", err)
	}
	return nil
}

func (m *Manager) walCheckpoint(ctx context.Context) error {
	db := usage.GetDB()
	if db == nil {
		return fmt.Errorf("backup: database not initialized")
	}
	if _, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("backup: wal_checkpoint: %w", err)
	}
	return nil
}

func (m *Manager) vacuumIntoDB(ctx context.Context, dstPath string) error {
	db := usage.GetDB()
	if db == nil {
		return fmt.Errorf("backup: database not initialized")
	}
	query := fmt.Sprintf("VACUUM INTO '%s'", strings.ReplaceAll(dstPath, "'", "''"))
	if _, err := db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("backup: VACUUM INTO: %w", err)
	}
	return nil
}

func (m *Manager) pruneRetention() error {
	backups, err := m.ListBackups()
	if err != nil {
		return err
	}
	if m.cfg.MaxAgeDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -m.cfg.MaxAgeDays)
		for _, b := range backups {
			if b.CreatedAt.Before(cutoff) {
				log.Infof("backup: pruning expired backup %s", b.Name)
				_ = os.Remove(b.Path)
				_ = m.removeFromIndex(b.Name)
			}
		}
	}
	if m.cfg.MaxKeep > 0 {
		backups, err = m.ListBackups()
		if err != nil {
			return err
		}
		for i := m.cfg.MaxKeep; i < len(backups); i++ {
			log.Infof("backup: pruning excess backup %s", backups[i].Name)
			_ = os.Remove(backups[i].Path)
			_ = m.removeFromIndex(backups[i].Name)
		}
	}
	return nil
}

type backupIndexEntry struct {
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
}

func validateBackupName(name string) error {
	if filepath.Base(name) != name || !backupNameRE.MatchString(name) {
		return fmt.Errorf("backup: invalid backup name %q", name)
	}
	return nil
}

func (m *Manager) indexPath() string { return filepath.Join(m.backupDir, backupIndexName) }

func (m *Manager) loadIndex() map[string]backupIndexEntry {
	entries := map[string]backupIndexEntry{}
	data, err := os.ReadFile(m.indexPath())
	if err != nil {
		return entries
	}
	_ = json.Unmarshal(data, &entries)
	return entries
}

func (m *Manager) saveIndex(entries map[string]backupIndexEntry) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.indexPath(), append(data, '\n'), 0600)
}

func (m *Manager) recordSource(name, source string) error {
	if err := validateBackupName(name); err != nil {
		return err
	}
	if source != "cron" {
		source = "manual"
	}
	createdAt, _ := parseBackupName(name)
	entries := m.loadIndex()
	entries[name] = backupIndexEntry{Source: source, CreatedAt: createdAt}
	return m.saveIndex(entries)
}

func (m *Manager) removeFromIndex(name string) error {
	entries := m.loadIndex()
	delete(entries, name)
	return m.saveIndex(entries)
}

func parseBackupName(name string) (time.Time, string) {
	trimmed := strings.TrimPrefix(name, backupNamePrefix)
	trimmed = strings.TrimSuffix(trimmed, backupExt)
	t, err := time.Parse(backupTimeLayout, trimmed)
	if err != nil {
		return time.Time{}, "manual"
	}
	return t, "manual"
}

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()
	if _, err := io.Copy(d, s); err != nil {
		return err
	}
	return d.Sync()
}

// copyDir recursively copies the src directory tree to dst.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target)
	})
}

func tarGzDir(srcDir, dstPath string) error {
	f, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("backup: rel path: %w", err)
		}
		if rel == "." {
			return nil
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("backup: header: %w", err)
		}
		header.Name = rel
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("backup: write header: %w", err)
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("backup: open: %w", err)
		}
		defer file.Close()
		if _, err := io.Copy(tw, file); err != nil {
			return fmt.Errorf("backup: copy to tar: %w", err)
		}
		return nil
	})
}

func unTarGz(srcPath, dstDir string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("backup: open archive: %w", err)
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("backup: gzip reader: %w", err)
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("backup: tar read: %w", err)
		}
		if filepath.IsAbs(header.Name) {
			return fmt.Errorf("backup: illegal absolute path in archive: %s", header.Name)
		}
		target := filepath.Join(dstDir, header.Name)
		rel, err := filepath.Rel(dstDir, target)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("backup: illegal path in archive: %s", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		default:
			return fmt.Errorf("backup: unsupported archive entry type %d for %s", header.Typeflag, header.Name)
		}
	}
	return nil
}
