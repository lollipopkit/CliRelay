package management

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/backup"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	log "github.com/sirupsen/logrus"
)

func (h *Handler) ensureBackupManager() *backup.Manager {
	if h.backupMgr != nil {
		return h.backupMgr
	}
	configDir := filepath.Dir(h.configFilePath)
	dbPath := filepath.Join(configDir, "data", "usage.db")
	authDir := resolveAuthDir(h.cfg)
	mgr, err := backup.NewManager(configDir, dbPath, authDir, h.cfg.Backup)
	if err != nil {
		log.Errorf("backup: init manager: %v", err)
		return nil
	}
	mgr.SetDBOptions(h.cfg.RequestLogStorage, config.ApplyTimeZone(h.cfg.Timezone))
	h.backupMgr = mgr
	return h.backupMgr
}

func (h *Handler) PostBackup(c *gin.Context) {
	mgr := h.ensureBackupManager()
	if mgr == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "backup manager unavailable"})
		return
	}
	var req backup.CreateBackupRequest
	includeAuth := h.cfg.Backup.IncludeAuthFiles
	if err := c.ShouldBindJSON(&req); err == nil && req.IncludeAuthFiles {
		includeAuth = true
	}
	name, err := mgr.CreateBackup(c.Request.Context(), includeAuth)
	if err != nil {
		log.Errorf("backup: create failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("backup failed: %v", err)})
		return
	}
	backups, _ := mgr.ListBackups()
	var info backup.BackupInfo
	for _, b := range backups {
		if b.Name == name {
			info = b
			break
		}
	}
	c.JSON(http.StatusOK, info)
}

func (h *Handler) GetBackups(c *gin.Context) {
	mgr := h.ensureBackupManager()
	if mgr == nil {
		c.JSON(http.StatusOK, backup.ListBackupsResponse{Backups: nil})
		return
	}
	backups, err := mgr.ListBackups()
	if err != nil {
		log.Errorf("backup: list failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("list failed: %v", err)})
		return
	}
	if backups == nil {
		backups = []backup.BackupInfo{}
	}
	c.JSON(http.StatusOK, backup.ListBackupsResponse{Backups: backups})
}

func (h *Handler) DownloadBackup(c *gin.Context) {
	mgr := h.ensureBackupManager()
	if mgr == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "backup manager unavailable"})
		return
	}
	name := c.Param("name")
	path, err := mgr.BackupPath(name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid backup name"})
		return
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "backup not found"})
		return
	}
	c.FileAttachment(path, name)
}

func (h *Handler) RestoreBackup(c *gin.Context) {
	mgr := h.ensureBackupManager()
	if mgr == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "backup manager unavailable"})
		return
	}
	name := c.Param("name")
	var req struct {
		RestoreConfig bool `json:"restore_config"`
		Confirm       bool `json:"confirm"`
	}
	_ = c.ShouldBindJSON(&req)
	if !req.Confirm {
		c.JSON(http.StatusBadRequest, gin.H{"error": "restore confirmation required"})
		return
	}
	if err := mgr.RestoreFromBackup(name, req.RestoreConfig); err != nil {
		log.Errorf("backup: restore failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("restore failed: %v", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "backup restored successfully — please restart the service"})
}

func (h *Handler) UploadAndRestore(c *gin.Context) {
	mgr := h.ensureBackupManager()
	if mgr == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "backup manager unavailable"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing or invalid file upload"})
		return
	}
	defer file.Close()

	if !strings.HasSuffix(header.Filename, ".tar.gz") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only .tar.gz backup files are accepted"})
		return
	}

	if c.PostForm("confirm") != "true" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "restore confirmation required"})
		return
	}

	// Save uploaded file to backup directory using a sanitized generated name.
	storedName, err := mgr.SaveUploadedBackup(header.Filename, file)
	if err != nil {
		log.Errorf("backup: upload save failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("save upload failed: %v", err)})
		return
	}

	restoreConfig := c.PostForm("restore_config") == "true"
	if err := mgr.RestoreFromBackup(storedName, restoreConfig); err != nil {
		_ = mgr.DeleteBackup(storedName) // cleanup uploaded file on failure
		log.Errorf("backup: upload restore failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("restore failed: %v", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "backup uploaded and restored successfully — please restart the service"})
}

func (h *Handler) DeleteBackup(c *gin.Context) {
	mgr := h.ensureBackupManager()
	if mgr == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "backup manager unavailable"})
		return
	}
	name := c.Param("name")
	if err := mgr.DeleteBackup(name); err != nil {
		log.Errorf("backup: delete failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("delete failed: %v", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "backup deleted"})
}

func (h *Handler) StartBackupScheduler() {
	if h.cfg == nil || !h.cfg.Backup.Enable {
		if h.backupScheduler != nil {
			h.backupScheduler.Stop()
			h.backupScheduler = nil
		}
		return
	}
	go func() {
		mgr := h.ensureBackupManager()
		if mgr == nil {
			return
		}
		sched := backup.NewScheduler(mgr)
		if h.backupScheduler != nil {
			h.backupScheduler.Stop()
		}
		if err := sched.Start(h.cfg.Backup.Cron); err != nil {
			log.Errorf("backup: scheduler start failed: %v", err)
		} else {
			h.backupScheduler = sched
		}
	}()
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
