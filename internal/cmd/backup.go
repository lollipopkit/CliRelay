package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/backup"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	log "github.com/sirupsen/logrus"
)

func backupManager(cfg *config.Config, configPath string) *backup.Manager {
	configDir := filepath.Dir(configPath)
	dbPath := filepath.Join(configDir, "data", "usage.db")
	authDir := resolveAuthDir(cfg)

	// Ensure DB is initialized so backup can access it.
	if usage.GetDB() == nil {
		log.Info("backup: initializing database for backup operation")
		if err := usage.InitDB(dbPath, cfg.RequestLogStorage, config.ApplyTimeZone(cfg.Timezone)); err != nil {
			log.Errorf("backup: failed to init DB: %v", err)
			return nil
		}
	}

	mgr, err := backup.NewManager(configDir, dbPath, authDir, cfg.Backup)
	if err != nil {
		log.Errorf("backup: failed to create manager: %v", err)
		return nil
	}
	mgr.SetDBOptions(cfg.RequestLogStorage, config.ApplyTimeZone(cfg.Timezone))
	return mgr
}

func DoBackupCreate(cfg *config.Config, configPath string, output string) {
	mgr := backupManager(cfg, configPath)
	if mgr == nil {
		fmt.Fprintln(os.Stderr, "backup: manager unavailable")
		return
	}

	name, err := mgr.CreateBackup(context.Background(), cfg.Backup.IncludeAuthFiles)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backup: create failed: %v\n", err)
		return
	}

	// If --output is specified, copy the backup there.
	if output != "" {
		srcPath := filepath.Join(mgr.BackupDir(), name)
		if err := copyFile(srcPath, output); err != nil {
			fmt.Fprintf(os.Stderr, "backup: copy to output failed: %v\n", err)
			return
		}
		fmt.Printf("Backup created: %s -> %s\n", name, output)
	} else {
		fmt.Printf("Backup created: %s\n", name)
	}
}

func DoBackupList(cfg *config.Config, configPath string) {
	mgr := backupManager(cfg, configPath)
	if mgr == nil {
		fmt.Fprintln(os.Stderr, "backup: manager unavailable")
		return
	}

	backups, err := mgr.ListBackups()
	if err != nil {
		fmt.Fprintf(os.Stderr, "backup: list failed: %v\n", err)
		return
	}

	if len(backups) == 0 {
		fmt.Println("No backups found.")
		return
	}

	fmt.Printf("%-50s %12s %20s %10s\n", "NAME", "SIZE", "CREATED", "SOURCE")
	for _, b := range backups {
		sizeStr := formatSize(b.Size)
		fmt.Printf("%-50s %12s %20s %10s\n",
			b.Name, sizeStr,
			b.CreatedAt.Format("2006-01-02 15:04:05"),
			b.Source)
	}
}

func DoBackupRestore(cfg *config.Config, configPath string, name string, restoreConfig bool) {
	if name == "" {
		fmt.Fprintln(os.Stderr, "backup: missing backup name")
		return
	}

	mgr := backupManager(cfg, configPath)
	if mgr == nil {
		fmt.Fprintln(os.Stderr, "backup: manager unavailable")
		return
	}

	fmt.Printf("Restoring from backup: %s\n", name)
	fmt.Println("WARNING: This will close the current database and replace data files.")
	fmt.Print("Continue? [y/N]: ")

	var answer string
	fmt.Scanln(&answer)
	if answer != "y" && answer != "Y" {
		fmt.Println("Restore cancelled.")
		return
	}

	if err := mgr.RestoreFromBackup(name, restoreConfig); err != nil {
		fmt.Fprintf(os.Stderr, "backup: restore failed: %v\n", err)
		return
	}

	fmt.Println("Restore complete. Please restart the service for changes to take effect.")
}

func DoBackupDelete(cfg *config.Config, configPath string, name string) {
	if name == "" {
		fmt.Fprintln(os.Stderr, "backup: missing backup name")
		return
	}

	mgr := backupManager(cfg, configPath)
	if mgr == nil {
		fmt.Fprintln(os.Stderr, "backup: manager unavailable")
		return
	}

	if err := mgr.DeleteBackup(name); err != nil {
		fmt.Fprintf(os.Stderr, "backup: delete failed: %v\n", err)
		return
	}

	fmt.Printf("Backup deleted: %s\n", name)
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// resolveAuthDir resolves the auth-dir path from config, expanding ~.
func resolveAuthDir(cfg *config.Config) string {
	authDir := strings.TrimSpace(cfg.AuthDir)
	if authDir == "" {
		return ""
	}
	if strings.HasPrefix(authDir, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return authDir
		}
		return filepath.Join(home, strings.TrimPrefix(authDir, "~"))
	}
	return authDir
}

// copyFile copies a file from src to dst.
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
	_, err = io.Copy(d, s)
	return err
}
