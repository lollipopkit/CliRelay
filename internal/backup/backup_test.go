package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
)

func setupTestEnv(t *testing.T) (configDir string, dbPath string, authDir string, cleanup func()) {
	t.Helper()
	tmpRoot, err := os.MkdirTemp("", "clirelay-backup-test-*")
	if err != nil {
		t.Fatalf("create temp root: %v", err)
	}

	configDir = filepath.Join(tmpRoot, "config")
	authDir = filepath.Join(tmpRoot, "auth")
	dbDir := filepath.Join(configDir, "data")

	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatalf("create data dir: %v", err)
	}
	if err := os.MkdirAll(authDir, 0755); err != nil {
		t.Fatalf("create auth dir: %v", err)
	}

	// Write a minimal config.yaml
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("port: 8317\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	// Write a minimal auth file
	if err := os.WriteFile(filepath.Join(authDir, "test-token.json"), []byte(`{"token":"abc"}`), 0600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}

	dbPath = filepath.Join(dbDir, "usage.db")

	cleanup = func() {
		usage.CloseDB()
		os.RemoveAll(tmpRoot)
	}
	return
}

func TestNewManager(t *testing.T) {
	configDir, dbPath, _, cleanup := setupTestEnv(t)
	defer cleanup()

	cfg := config.BackupConfig{Dir: "backups"}
	mgr, err := NewManager(configDir, dbPath, "", cfg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if mgr.BackupDir() == "" {
		t.Fatal("BackupDir should not be empty")
	}
	// Backup dir should exist
	if _, err := os.Stat(mgr.BackupDir()); os.IsNotExist(err) {
		t.Fatal("backup directory was not created")
	}
}

func TestNewManagerAbsoluteDir(t *testing.T) {
	configDir, dbPath, _, cleanup := setupTestEnv(t)
	defer cleanup()

	absBackupDir := filepath.Join(configDir, "custom-backups")
	cfg := config.BackupConfig{Dir: absBackupDir}
	mgr, err := NewManager(configDir, dbPath, "", cfg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if mgr.BackupDir() != absBackupDir {
		t.Fatalf("BackupDir = %q, want %q", mgr.BackupDir(), absBackupDir)
	}
}

func TestCreateAndListBackup(t *testing.T) {
	configDir, dbPath, _, cleanup := setupTestEnv(t)
	defer cleanup()

	cfg := config.BackupConfig{Dir: "backups"}
	mgr, err := NewManager(configDir, dbPath, "", cfg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Create a minimal SQLite DB first (without full InitDB dependency)
	if err := createTestDB(dbPath); err != nil {
		t.Fatalf("create test db: %v", err)
	}

	name, err := mgr.CreateBackup(context.Background(), false)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	if !strings.HasPrefix(name, backupNamePrefix) {
		t.Fatalf("backup name %q should have prefix %q", name, backupNamePrefix)
	}
	if !strings.HasSuffix(name, backupExt) {
		t.Fatalf("backup name %q should have extension %q", name, backupExt)
	}

	backups, err := mgr.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(backups))
	}
	if backups[0].Name != name {
		t.Fatalf("backup name = %q, want %q", backups[0].Name, name)
	}
	if backups[0].Size <= 0 {
		t.Fatal("backup size should be positive")
	}
	if backups[0].Source != "manual" {
		t.Fatalf("source = %q, want 'manual'", backups[0].Source)
	}
}

func TestCreateBackupIncludesAuth(t *testing.T) {
	configDir, dbPath, authDir, cleanup := setupTestEnv(t)
	defer cleanup()

	cfg := config.BackupConfig{Dir: "backups"}
	mgr, err := NewManager(configDir, dbPath, authDir, cfg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := createTestDB(dbPath); err != nil {
		t.Fatalf("create test db: %v", err)
	}

	name, err := mgr.CreateBackup(context.Background(), true)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	// Extract and verify auth files are in the archive
	archivePath := filepath.Join(mgr.BackupDir(), name)
	if err := verifyArchiveContains(archivePath, "auth/test-token.json"); err != nil {
		t.Fatalf("auth file missing from archive: %v", err)
	}
}

func TestCreateBackupExcludesAuth(t *testing.T) {
	configDir, dbPath, authDir, cleanup := setupTestEnv(t)
	defer cleanup()

	cfg := config.BackupConfig{Dir: "backups"}
	mgr, err := NewManager(configDir, dbPath, authDir, cfg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := createTestDB(dbPath); err != nil {
		t.Fatalf("create test db: %v", err)
	}

	name, err := mgr.CreateBackup(context.Background(), false)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	archivePath := filepath.Join(mgr.BackupDir(), name)
	if err := verifyArchiveNotContains(archivePath, "auth/"); err != nil {
		t.Fatalf("auth directory should not be in archive: %v", err)
	}
}

func TestCreateBackupWithCronSource(t *testing.T) {
	configDir, dbPath, _, cleanup := setupTestEnv(t)
	defer cleanup()

	cfg := config.BackupConfig{Dir: "backups"}
	mgr, err := NewManager(configDir, dbPath, "", cfg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := createTestDB(dbPath); err != nil {
		t.Fatalf("create test db: %v", err)
	}

	name, err := mgr.CreateBackupWithSource(context.Background(), false, "cron")
	if err != nil {
		t.Fatalf("CreateBackupWithSource: %v", err)
	}
	backups, err := mgr.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(backups) != 1 || backups[0].Name != name {
		t.Fatalf("unexpected backups: %+v", backups)
	}
	if backups[0].Source != "cron" {
		t.Fatalf("source = %q, want cron", backups[0].Source)
	}
}

func TestDeleteBackup(t *testing.T) {
	configDir, dbPath, _, cleanup := setupTestEnv(t)
	defer cleanup()

	cfg := config.BackupConfig{Dir: "backups"}
	mgr, err := NewManager(configDir, dbPath, "", cfg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := createTestDB(dbPath); err != nil {
		t.Fatalf("create test db: %v", err)
	}

	name, err := mgr.CreateBackup(context.Background(), false)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	if err := mgr.DeleteBackup(name); err != nil {
		t.Fatalf("DeleteBackup: %v", err)
	}

	backups, _ := mgr.ListBackups()
	if len(backups) != 0 {
		t.Fatalf("expected 0 backups after delete, got %d", len(backups))
	}
}

func TestDeleteBackupInvalidName(t *testing.T) {
	configDir, dbPath, _, cleanup := setupTestEnv(t)
	defer cleanup()

	cfg := config.BackupConfig{Dir: "backups"}
	mgr, err := NewManager(configDir, dbPath, "", cfg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := mgr.DeleteBackup("not-a-backup.txt"); err == nil {
		t.Fatal("expected error for invalid backup name")
	}
}

func TestPruneRetentionMaxKeep(t *testing.T) {
	configDir, dbPath, _, cleanup := setupTestEnv(t)
	defer cleanup()

	cfg := config.BackupConfig{Dir: "backups", MaxKeep: 2}
	mgr, err := NewManager(configDir, dbPath, "", cfg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := createTestDB(dbPath); err != nil {
		t.Fatalf("create test db: %v", err)
	}

	// Create 3 backups with small delay to get unique timestamps
	for i := 0; i < 3; i++ {
		_, err := mgr.CreateBackup(context.Background(), false)
		if err != nil {
			t.Fatalf("CreateBackup %d: %v", i, err)
		}
		if i < 2 {
			time.Sleep(1100 * time.Millisecond) // ensure unique timestamp
		}
	}

	backups, _ := mgr.ListBackups()
	if len(backups) > 2 {
		t.Fatalf("expected at most 2 backups after prune, got %d", len(backups))
	}
}

func TestPruneRetentionMaxAgeDays(t *testing.T) {
	configDir, dbPath, _, cleanup := setupTestEnv(t)
	defer cleanup()

	cfg := config.BackupConfig{Dir: "backups", MaxAgeDays: 1}
	mgr, err := NewManager(configDir, dbPath, "", cfg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := createTestDB(dbPath); err != nil {
		t.Fatalf("create test db: %v", err)
	}

	// Create a fresh backup
	_, err = mgr.CreateBackup(context.Background(), false)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	// Manually create an "old" backup file
	oldName := backupNamePrefix + time.Now().AddDate(0, 0, -5).Format(backupTimeLayout) + backupExt
	oldPath := filepath.Join(mgr.BackupDir(), oldName)
	if err := os.WriteFile(oldPath, []byte("old"), 0600); err != nil {
		t.Fatalf("write old backup: %v", err)
	}

	// Trigger prune via another backup creation (which calls pruneRetention)
	_, err = mgr.CreateBackup(context.Background(), false)
	if err != nil {
		t.Fatalf("CreateBackup (prune trigger): %v", err)
	}

	// The old file should have been pruned
	if _, err := os.Stat(oldPath); err == nil {
		t.Fatal("old backup should have been pruned")
	}
}

func TestListBackupsEmpty(t *testing.T) {
	configDir, dbPath, _, cleanup := setupTestEnv(t)
	defer cleanup()

	cfg := config.BackupConfig{Dir: "backups"}
	mgr, err := NewManager(configDir, dbPath, "", cfg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	backups, err := mgr.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(backups) != 0 {
		t.Fatalf("expected 0 backups, got %d", len(backups))
	}
}

func TestParseBackupName(t *testing.T) {
	name := backupNamePrefix + "20260511T030000" + backupExt
	ts, source := parseBackupName(name)
	if source != "manual" {
		t.Fatalf("source = %q, want 'manual'", source)
	}
	if ts.Year() != 2026 {
		t.Fatalf("year = %d, want 2026", ts.Year())
	}
	_, source2 := parseBackupName("garbage")
	if source2 != "manual" {
		t.Fatalf("fallback source = %q, want 'manual'", source2)
	}
}

func TestCopyDir(t *testing.T) {
	src, err := os.MkdirTemp("", "copydir-src-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(src)

	dst, err := os.MkdirTemp("", "copydir-dst-*")
	if err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(dst) // remove so copyDir creates it
	defer os.RemoveAll(dst)

	sub := filepath.Join(src, "nested")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "file.txt"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dst, "nested", "file.txt"))
	if err != nil {
		t.Fatalf("read copied file: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("copied content = %q, want 'hello'", data)
	}
}

// --- helpers ---

func createTestDB(path string) error {
	usage.CloseDB()
	return usage.InitDB(path, config.RequestLogStorageConfig{}, nil)
}

func verifyArchiveContains(archivePath, expectedPrefix string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		if strings.HasPrefix(hdr.Name, expectedPrefix) {
			return nil
		}
	}
	return os.ErrNotExist
}

func verifyArchiveNotContains(archivePath, unexpectedPrefix string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		if strings.HasPrefix(hdr.Name, unexpectedPrefix) {
			return os.ErrExist
		}
	}
	return nil
}
