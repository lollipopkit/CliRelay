package backup

import "time"

type BackupInfo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	Source    string    `json:"source"`
}

type CreateBackupRequest struct {
	IncludeAuthFiles bool `json:"include_auth_files"`
}

type ListBackupsResponse struct {
	Backups []BackupInfo `json:"backups"`
}
