package managementasset

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	"github.com/router-for-me/CLIProxyAPI/v6/panel"
)

const PanelMetadataFileName = "panel-meta.json"

// managementAssetName is the legacy single-file panel asset name.
const managementAssetName = "management.html"

// embeddedPanelMetadataPath is the metadata path inside embedded assets.
const embeddedPanelMetadataPath = "dist/panel-meta.json"

// PanelMetadata describes the management panel currently present on disk.
// It lets update checks compare the actual served UI instead of stale binary build info.
type PanelMetadata struct {
	Version    string `json:"version"`
	Ref        string `json:"ref"`
	Commit     string `json:"commit"`
	Repository string `json:"repository"`
	BuildDate  string `json:"build_date"`
}

// ResolvePanelDir returns the directory containing the SPA panel (manage.html + assets/).
func ResolvePanelDir(configFilePath string) string {
	if override := strings.TrimSpace(os.Getenv("MANAGEMENT_PANEL_DIR")); override != "" {
		if info, err := os.Stat(override); err == nil && info.IsDir() {
			return override
		}
	}

	candidates := []string{
		"/home/web/html/cliproxy-panel",
	}
	if staticDir := StaticDir(configFilePath); staticDir != "" {
		candidates = append(candidates, staticDir)
	}

	for _, dir := range candidates {
		manageHTML := filepath.Join(dir, "manage.html")
		if _, err := os.Stat(manageHTML); err == nil {
			return dir
		}
	}
	return ""
}

// StaticDir resolves the directory that stores management assets on disk.
func StaticDir(configFilePath string) string {
	if override := strings.TrimSpace(os.Getenv("MANAGEMENT_STATIC_PATH")); override != "" {
		cleaned := filepath.Clean(override)
		if strings.EqualFold(filepath.Base(cleaned), managementAssetName) {
			return filepath.Dir(cleaned)
		}
		return cleaned
	}

	if writable := util.WritablePath(); writable != "" {
		return filepath.Join(writable, "static")
	}

	configFilePath = strings.TrimSpace(configFilePath)
	if configFilePath == "" {
		return ""
	}

	base := filepath.Dir(configFilePath)
	if fileInfo, err := os.Stat(configFilePath); err == nil && fileInfo.IsDir() {
		base = configFilePath
	}
	return filepath.Join(base, "static")
}

// FilePath resolves the absolute path to the management control panel asset.
func FilePath(configFilePath string) string {
	if override := strings.TrimSpace(os.Getenv("MANAGEMENT_STATIC_PATH")); override != "" {
		cleaned := filepath.Clean(override)
		if strings.EqualFold(filepath.Base(cleaned), managementAssetName) {
			return cleaned
		}
		return filepath.Join(cleaned, managementAssetName)
	}

	dir := StaticDir(configFilePath)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, managementAssetName)
}

func ReadPanelMetadata(panelDir string) (PanelMetadata, bool) {
	panelDir = strings.TrimSpace(panelDir)
	if panelDir == "" {
		return PanelMetadata{}, false
	}

	data, err := os.ReadFile(filepath.Join(panelDir, PanelMetadataFileName))
	if err != nil {
		return PanelMetadata{}, false
	}

	var meta PanelMetadata
	if err = json.Unmarshal(data, &meta); err != nil {
		return PanelMetadata{}, false
	}
	meta.Version = strings.TrimSpace(meta.Version)
	meta.Ref = strings.TrimSpace(meta.Ref)
	meta.Commit = strings.TrimSpace(meta.Commit)
	meta.Repository = strings.TrimSpace(meta.Repository)
	meta.BuildDate = strings.TrimSpace(meta.BuildDate)
	return meta, meta.Version != "" || meta.Commit != ""
}

func ReadEmbeddedPanelMetadata() (PanelMetadata, bool) {
	data, ok := readEmbeddedPanelAsset(embeddedPanelMetadataPath)
	meta, err := parsePanelMetadata(data, ok)
	if err != nil {
		return PanelMetadata{}, false
	}

	return meta, meta.Version != "" || meta.Commit != ""
}

func parsePanelMetadata(data []byte, ok bool) (PanelMetadata, error) {
	if !ok || len(data) == 0 {
		return PanelMetadata{}, os.ErrNotExist
	}

	var meta PanelMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return PanelMetadata{}, err
	}

	meta.Version = strings.TrimSpace(meta.Version)
	meta.Ref = strings.TrimSpace(meta.Ref)
	meta.Commit = strings.TrimSpace(meta.Commit)
	meta.Repository = strings.TrimSpace(meta.Repository)
	meta.BuildDate = strings.TrimSpace(meta.BuildDate)
	return meta, nil
}

// IsEmbeddedPanelAvailable reports whether the repository already contains embedded panel assets.
func IsEmbeddedPanelAvailable() bool {
	_, ok := readEmbeddedPanelAsset("manage.html")
	return ok
}

// ReadEmbeddedPanelAsset reads a single file from embedded panel assets.
// The provided path is panel-root relative (for example: "manage.html", "assets/index.js").
func ReadEmbeddedPanelAsset(pathInPanel string) ([]byte, bool) {
	trimmed := strings.TrimSpace(pathInPanel)
	if trimmed == "" {
		return nil, false
	}

	cleaned := path.Clean(strings.TrimPrefix(pathInPanel, "/"))
	if cleaned == "." || strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "../") {
		return nil, false
	}

	panelPath := path.Join("dist", cleaned)
	return readEmbeddedPanelAsset(panelPath)
}

func readEmbeddedPanelAsset(pathInDist string) ([]byte, bool) {
	data, err := panel.EmbeddedDist.ReadFile(pathInDist)
	if err != nil {
		return nil, false
	}
	return data, true
}

func CurrentPanelMetadata(configFilePath string) (PanelMetadata, bool) {
	if meta, ok := ReadPanelMetadata(ResolvePanelDir(configFilePath)); ok {
		return meta, true
	}
	return ReadEmbeddedPanelMetadata()
}
