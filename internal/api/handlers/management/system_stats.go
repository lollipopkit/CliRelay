package management

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/api/middleware"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	log "github.com/sirupsen/logrus"
)

// SystemStats is the JSON payload pushed via WebSocket and returned by HTTP.
type SystemStats struct {
	// Database
	DBSizeBytes int64 `json:"db_size_bytes"`

	// Request log body storage inside SQLite.
	LogContentStoreBytes int64 `json:"log_content_store_bytes"`

	// Log directory size on disk.
	LogDirSizeBytes int64 `json:"log_dir_size_bytes"`

	// Deprecated alias retained for older panels that still read log_size_bytes.
	LogSizeBytes int64 `json:"log_size_bytes"`

	// Process-level runtime metrics
	GoRoutines  int    `json:"go_routines"`
	GoHeapBytes uint64 `json:"go_heap_bytes"`

	// Uptime
	UptimeSeconds int64  `json:"uptime_seconds"`
	StartTime     string `json:"start_time"`

	// Channel latency
	ChannelLatency []usage.ChannelLatency `json:"channel_latency"`

	// Concurrency
	ActiveConcurrency []middleware.ConcurrencySnapshot `json:"active_concurrency"`
	TotalInFlight     int64                            `json:"total_in_flight"`
	TotalRPM          int                              `json:"total_rpm"`
	TotalTPM          int64                            `json:"total_tpm"`
}

func (h *Handler) collectSystemStats() SystemStats {
	stats := SystemStats{
		GoRoutines:    runtime.NumGoroutine(),
		StartTime:     h.startTime.Format(time.RFC3339),
		UptimeSeconds: int64(time.Since(h.startTime).Seconds()),
	}

	// ── Go runtime memory ──
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	stats.GoHeapBytes = m.HeapAlloc

	// ── DB file size ──
	dbPath := usage.GetDBPath()
	if dbPath != "" {
		if info, err := os.Stat(dbPath); err == nil {
			stats.DBSizeBytes = info.Size()
		}
		// Also check WAL and SHM files
		for _, suffix := range []string{"-wal", "-shm"} {
			if info, err := os.Stat(dbPath + suffix); err == nil {
				stats.DBSizeBytes += info.Size()
			}
		}
	}
	if contentBytes, err := usage.GetRequestLogStorageBytes(); err == nil {
		stats.LogContentStoreBytes = contentBytes
	} else {
		log.Warnf("system-stats: failed to query request log storage bytes: %v", err)
	}

	// ── Log directory size ──
	if h.logDir != "" {
		stats.LogDirSizeBytes = dirSize(h.logDir)
		stats.LogSizeBytes = stats.LogDirSizeBytes
	}

	// ── Channel latency (from DB) ──
	if cl, err := usage.GetChannelAvgLatency(7); err == nil {
		stats.ChannelLatency = cl
	}

	// ── Concurrency snapshot ──
	stats.ActiveConcurrency, stats.TotalInFlight = middleware.GetConcurrencySnapshot()

	// Compute system-wide RPM and TPM totals
	var sysRPM int
	var sysTPM int64
	for _, snap := range stats.ActiveConcurrency {
		sysRPM += snap.RPM
		sysTPM += snap.TPM
	}
	stats.TotalRPM = sysRPM
	stats.TotalTPM = sysTPM

	return stats
}

// GetSystemStats handles GET /v0/management/system-stats
func (h *Handler) GetSystemStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.collectSystemStats())
}

// dirSize calculates the total size of all files in a directory tree.
func dirSize(path string) int64 {
	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		size += info.Size()
		return nil
	})
	return size
}
