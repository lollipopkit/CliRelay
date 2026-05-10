package management

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestInferAutoUpdateChannel(t *testing.T) {
	tests := []struct {
		name    string
		version string
		env     string
		want    string
	}{
		{name: "explicit dev version", version: "dev-a35756e", want: "dev"},
		{name: "explicit main version", version: "main-a35756e", want: "main"},
		{name: "release tag defaults main", version: "v1.2.3", want: "main"},
		{name: "environment overrides version", version: "main-a35756e", env: "dev", want: "dev"},
		{name: "unknown environment ignored", version: "main-a35756e", env: "staging", want: "main"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inferAutoUpdateChannel(tt.version, tt.env); got != tt.want {
				t.Fatalf("inferAutoUpdateChannel(%q, %q) = %q, want %q", tt.version, tt.env, got, tt.want)
			}
		})
	}
}

func TestAutoUpdateAvailableFromCommit(t *testing.T) {
	tests := []struct {
		name          string
		currentCommit string
		latestCommit  string
		want          bool
	}{
		{name: "same full commit", currentCommit: "abcdef123456", latestCommit: "abcdef123456", want: false},
		{name: "current short commit matches latest", currentCommit: "abcdef1", latestCommit: "abcdef123456", want: false},
		{name: "different commit", currentCommit: "1111111", latestCommit: "abcdef123456", want: true},
		{name: "missing latest commit", currentCommit: "1111111", latestCommit: "", want: false},
		{name: "missing current commit", currentCommit: "", latestCommit: "abcdef123456", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := autoUpdateAvailableFromCommit(tt.currentCommit, tt.latestCommit); got != tt.want {
				t.Fatalf("autoUpdateAvailableFromCommit(%q, %q) = %v, want %v", tt.currentCommit, tt.latestCommit, got, tt.want)
			}
		})
	}
}

func TestAutoUpdateAvailable(t *testing.T) {
	tests := []struct {
		name            string
		currentBackend  string
		latestBackend   string
		currentFrontend string
		latestFrontend  string
		want            bool
	}{
		{
			name:            "backend changed",
			currentBackend:  "1111111",
			latestBackend:   "abcdef123456",
			currentFrontend: "panel-main-9477958",
			latestFrontend:  "94779588adb784b1ceff19c662d3ab55155997e1",
			want:            true,
		},
		{
			name:            "frontend changed while backend stays the same",
			currentBackend:  "a0ed5c63a118412d5b4da8d57ec6d049111b7888",
			latestBackend:   "a0ed5c63a118412d5b4da8d57ec6d049111b7888",
			currentFrontend: "1111111",
			latestFrontend:  "94779588adb784b1ceff19c662d3ab55155997e1",
			want:            true,
		},
		{
			name:            "both backend and frontend already match",
			currentBackend:  "a0ed5c63a118412d5b4da8d57ec6d049111b7888",
			latestBackend:   "a0ed5c63a118412d5b4da8d57ec6d049111b7888",
			currentFrontend: "94779588adb784b1ceff19c662d3ab55155997e1",
			latestFrontend:  "94779588adb784b1ceff19c662d3ab55155997e1",
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := autoUpdateAvailable(
				tt.currentBackend,
				tt.latestBackend,
				tt.currentFrontend,
				tt.latestFrontend,
			); got != tt.want {
				t.Fatalf("autoUpdateAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFrontendDisplayVersionsIncludeConcreteCommit(t *testing.T) {
	if got := currentFrontendDisplayVersion("panel-main-9477958", "main", "94779588adb784b1ceff19c662d3ab55155997e1"); got != "panel-main-9477958" {
		t.Fatalf("currentFrontendDisplayVersion() = %q, want panel-main-9477958", got)
	}
	if got := latestFrontendDisplayVersion("main", "94779588adb784b1ceff19c662d3ab55155997e1"); got != "panel-main-9477958" {
		t.Fatalf("latestFrontendDisplayVersion(main) = %q, want panel-main-9477958", got)
	}
	if got := latestFrontendDisplayVersion("dev", "3758025c21de3f0a47a8e1e08cb1b859c73069ba"); got != "panel-dev-3758025" {
		t.Fatalf("latestFrontendDisplayVersion(dev) = %q, want panel-dev-3758025", got)
	}
}

func TestDockerTagForChannel(t *testing.T) {
	if got := dockerTagForChannel("dev", "a35756e"); got != "dev" {
		t.Fatalf("dockerTagForChannel(dev) = %q, want dev", got)
	}
	if got := dockerTagForChannel("main", "a35756e"); got != "latest" {
		t.Fatalf("dockerTagForChannel(main) = %q, want latest", got)
	}
}

func TestUpdateDisplayVersionsIncludeConcreteCommit(t *testing.T) {
	if got := currentUpdateDisplayVersion("dev-d5c2482"); got != "dev-d5c2482" {
		t.Fatalf("currentUpdateDisplayVersion(dev-d5c2482) = %q, want dev-d5c2482", got)
	}
	if got := currentUpdateDisplayVersion("main-d5c2482"); got != "main-d5c2482" {
		t.Fatalf("currentUpdateDisplayVersion(main-d5c2482) = %q, want main-d5c2482", got)
	}
	if got := latestUpdateDisplayVersion("main", "de96948c21de3f0a47a8e1e08cb1b859c73069ba"); got != "main-de96948" {
		t.Fatalf("latestUpdateDisplayVersion(main) = %q, want main-de96948", got)
	}
	if got := latestUpdateDisplayVersion("dev", "3758025c21de3f0a47a8e1e08cb1b859c73069ba"); got != "dev-3758025" {
		t.Fatalf("latestUpdateDisplayVersion(dev) = %q, want dev-3758025", got)
	}
}

func TestAutoUpdateChannelEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.AutoUpdate.Channel = config.DefaultAutoUpdateChannel
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("port: 8317\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	handler := NewHandler(cfg, configPath, nil)

	router := gin.New()
	router.GET("/channel", handler.GetAutoUpdateChannel)
	router.PUT("/channel", handler.PutAutoUpdateChannel)

	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/channel", nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body=%s", getRec.Code, getRec.Body.String())
	}
	if !strings.Contains(getRec.Body.String(), `"channel":"main"`) {
		t.Fatalf("GET body = %s, want channel main", getRec.Body.String())
	}

	putRec := httptest.NewRecorder()
	putReq := httptest.NewRequest(http.MethodPut, "/channel", strings.NewReader(`{"value":"dev"}`))
	putReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body=%s", putRec.Code, putRec.Body.String())
	}
	if cfg.AutoUpdate.Channel != "dev" {
		t.Fatalf("AutoUpdate.Channel = %q, want dev", cfg.AutoUpdate.Channel)
	}
}

func TestBuildUpdateCheckGracefullyHandlesGitHubFailures(t *testing.T) {
	origFetchBranchCommit := fetchBranchCommitForUpdateCheck
	origFetchLatestRelease := fetchLatestReleaseInfoForUpdateCheck
	t.Cleanup(func() {
		fetchBranchCommitForUpdateCheck = origFetchBranchCommit
		fetchLatestReleaseInfoForUpdateCheck = origFetchLatestRelease
	})

	fetchBranchCommitForUpdateCheck = func(ctx context.Context, client *http.Client, repo string, channel string) (branchCommitInfo, error) {
		return branchCommitInfo{}, errors.New("github rate limit exceeded")
	}
	fetchLatestReleaseInfoForUpdateCheck = func(ctx context.Context, client *http.Client, repo string) (releaseInfo, error) {
		return releaseInfo{}, errors.New("release unavailable")
	}

	cfg := &config.Config{}
	cfg.AutoUpdate.Enabled = true
	cfg.AutoUpdate.Channel = "dev"
	cfg.AutoUpdate.Repository = "https://github.com/kittors/CliRelay"

	handler := &Handler{cfg: cfg}
	resp, err := handler.buildUpdateCheck(context.Background())
	if err != nil {
		t.Fatalf("buildUpdateCheck() error = %v, want nil", err)
	}
	if resp == nil {
		t.Fatal("buildUpdateCheck() returned nil response")
	}
	if resp.UpdateAvailable {
		t.Fatalf("UpdateAvailable = true, want false when GitHub checks fail")
	}
	if !strings.Contains(strings.ToLower(resp.Message), "github") {
		t.Fatalf("Message = %q, want GitHub failure context", resp.Message)
	}
	if resp.TargetChannel != "dev" {
		t.Fatalf("TargetChannel = %q, want dev", resp.TargetChannel)
	}
	if resp.LatestVersion != resp.CurrentVersion {
		t.Fatalf("LatestVersion = %q, want fallback current version %q", resp.LatestVersion, resp.CurrentVersion)
	}
	if resp.LatestUIVersion != resp.CurrentUIVersion {
		t.Fatalf("LatestUIVersion = %q, want fallback current UI version %q", resp.LatestUIVersion, resp.CurrentUIVersion)
	}
}

func TestBuildUpdateCheckUsesConfiguredPanelRepository(t *testing.T) {
	origFetchBranchCommit := fetchBranchCommitForUpdateCheck
	origFetchLatestRelease := fetchLatestReleaseInfoForUpdateCheck
	t.Cleanup(func() {
		fetchBranchCommitForUpdateCheck = origFetchBranchCommit
		fetchLatestReleaseInfoForUpdateCheck = origFetchLatestRelease
	})

	var repos []string
	fetchBranchCommitForUpdateCheck = func(ctx context.Context, client *http.Client, repo string, channel string) (branchCommitInfo, error) {
		repos = append(repos, repo)
		return branchCommitInfo{SHA: "abcdef1234567", HTMLURL: "https://example.com/" + repo}, nil
	}
	fetchLatestReleaseInfoForUpdateCheck = func(ctx context.Context, client *http.Client, repo string) (releaseInfo, error) {
		return releaseInfo{}, nil
	}

	cfg := &config.Config{}
	cfg.AutoUpdate.Enabled = true
	cfg.AutoUpdate.Channel = "main"
	cfg.AutoUpdate.Repository = "https://github.com/kittors/CliRelay"

	handler := &Handler{cfg: cfg}
	if _, err := handler.buildUpdateCheck(context.Background()); err != nil {
		t.Fatalf("buildUpdateCheck() error = %v, want nil", err)
	}

	if len(repos) != 2 {
		t.Fatalf("expected 2 branch lookups (backend + frontend), got %d", len(repos))
	}
	if repos[0] != "kittors/CliRelay" || repos[1] != "kittors/CliRelay" {
		t.Fatalf("repos = %v, want backend and frontend to query the same repo", repos)
	}
}

func TestBuildUpdateCheckUsesRuntimePanelMetadata(t *testing.T) {
	origFetchBranchCommit := fetchBranchCommitForUpdateCheck
	origFetchLatestRelease := fetchLatestReleaseInfoForUpdateCheck
	origVersion := buildinfo.Version
	origCommit := buildinfo.Commit
	origBuildDate := buildinfo.BuildDate
	origFrontendVersion := buildinfo.FrontendVersion
	origFrontendCommit := buildinfo.FrontendCommit
	origFrontendRef := buildinfo.FrontendRef
	t.Cleanup(func() {
		fetchBranchCommitForUpdateCheck = origFetchBranchCommit
		fetchLatestReleaseInfoForUpdateCheck = origFetchLatestRelease
		buildinfo.Version = origVersion
		buildinfo.Commit = origCommit
		buildinfo.BuildDate = origBuildDate
		buildinfo.FrontendVersion = origFrontendVersion
		buildinfo.FrontendCommit = origFrontendCommit
		buildinfo.FrontendRef = origFrontendRef
	})

	panelDir := t.TempDir()
	t.Setenv("MANAGEMENT_PANEL_DIR", panelDir)
	if err := os.WriteFile(filepath.Join(panelDir, "manage.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatalf("write manage.html: %v", err)
	}
	runtimeUICommit := "a28920de945ac13611eb88315cf5aff895bb8c78"
	metadata := `{"version":"panel-dev-a28920d","ref":"dev","commit":"` + runtimeUICommit + `","repository":"https://github.com/router-for-me/Cli-Proxy-API-Management-Center"}`
	if err := os.WriteFile(filepath.Join(panelDir, "panel-meta.json"), []byte(metadata), 0o644); err != nil {
		t.Fatalf("write panel metadata: %v", err)
	}

	fetchBranchCommitForUpdateCheck = func(ctx context.Context, client *http.Client, repo string, channel string) (branchCommitInfo, error) {
		switch repo {
		case "kittors/CliRelay":
			return branchCommitInfo{SHA: "1402b1d6970b7ce61eec9430b137e817c448d215"}, nil
		default:
			t.Fatalf("unexpected repo %q", repo)
			return branchCommitInfo{}, nil
		}
	}
	fetchLatestReleaseInfoForUpdateCheck = func(ctx context.Context, client *http.Client, repo string) (releaseInfo, error) {
		return releaseInfo{}, nil
	}

	buildinfo.Version = "dev-1402b1d"
	buildinfo.Commit = "1402b1d6970b7ce61eec9430b137e817c448d215"
	buildinfo.BuildDate = "2026-04-20T07:51:38Z"
	buildinfo.FrontendVersion = "panel-dev-97847f8"
	buildinfo.FrontendCommit = "97847f83ca0e33f3145a3526e9c9e47e0867803c"
	buildinfo.FrontendRef = "dev"

	cfg := &config.Config{}
	cfg.AutoUpdate.Enabled = true
	cfg.AutoUpdate.Channel = "dev"
	cfg.AutoUpdate.Repository = "https://github.com/kittors/CliRelay"

	handler := &Handler{cfg: cfg}
	resp, err := handler.buildUpdateCheck(context.Background())
	if err != nil {
		t.Fatalf("buildUpdateCheck() error = %v, want nil", err)
	}
	if resp.CurrentUIVersion != "panel-dev-a28920d" {
		t.Fatalf("CurrentUIVersion = %q, want panel-dev-a28920d", resp.CurrentUIVersion)
	}
	if resp.CurrentUICommit != runtimeUICommit {
		t.Fatalf("CurrentUICommit = %q, want %q", resp.CurrentUICommit, runtimeUICommit)
	}
	if resp.UpdateAvailable {
		t.Fatalf("UpdateAvailable = true, want false when runtime panel metadata matches target")
	}
}

func TestBuildCurrentUpdateStateDoesNotQueryGitHub(t *testing.T) {
	origFetchBranchCommit := fetchBranchCommitForUpdateCheck
	origFetchLatestRelease := fetchLatestReleaseInfoForUpdateCheck
	origVersion := buildinfo.Version
	origCommit := buildinfo.Commit
	origBuildDate := buildinfo.BuildDate
	origFrontendVersion := buildinfo.FrontendVersion
	origFrontendCommit := buildinfo.FrontendCommit
	origFrontendRef := buildinfo.FrontendRef
	t.Cleanup(func() {
		fetchBranchCommitForUpdateCheck = origFetchBranchCommit
		fetchLatestReleaseInfoForUpdateCheck = origFetchLatestRelease
		buildinfo.Version = origVersion
		buildinfo.Commit = origCommit
		buildinfo.BuildDate = origBuildDate
		buildinfo.FrontendVersion = origFrontendVersion
		buildinfo.FrontendCommit = origFrontendCommit
		buildinfo.FrontendRef = origFrontendRef
	})

	fetchBranchCommitForUpdateCheck = func(ctx context.Context, client *http.Client, repo string, channel string) (branchCommitInfo, error) {
		t.Fatal("buildCurrentUpdateState must not query GitHub branch commits")
		return branchCommitInfo{}, nil
	}
	fetchLatestReleaseInfoForUpdateCheck = func(ctx context.Context, client *http.Client, repo string) (releaseInfo, error) {
		t.Fatal("buildCurrentUpdateState must not query GitHub releases")
		return releaseInfo{}, nil
	}
	buildinfo.Version = "dev-abcdef1"
	buildinfo.Commit = "abcdef123456"
	buildinfo.BuildDate = "2026-04-20T06:14:57Z"
	buildinfo.FrontendVersion = "panel-dev-fedcba9"
	buildinfo.FrontendCommit = "fedcba987654"
	buildinfo.FrontendRef = "dev"

	cfg := &config.Config{}
	cfg.AutoUpdate.Enabled = true
	cfg.AutoUpdate.Channel = "dev"
	handler := &Handler{cfg: cfg}

	resp := handler.buildCurrentUpdateState(context.Background())
	if resp.CurrentVersion != "dev-abcdef1" {
		t.Fatalf("CurrentVersion = %q, want dev-abcdef1", resp.CurrentVersion)
	}
	if resp.CurrentCommit != "abcdef123456" {
		t.Fatalf("CurrentCommit = %q, want abcdef123456", resp.CurrentCommit)
	}
	if resp.CurrentUIVersion != "panel-dev-fedcba9" {
		t.Fatalf("CurrentUIVersion = %q, want panel-dev-fedcba9", resp.CurrentUIVersion)
	}
	if resp.CurrentUICommit != "fedcba987654" {
		t.Fatalf("CurrentUICommit = %q, want fedcba987654", resp.CurrentUICommit)
	}
	if resp.TargetChannel != "dev" {
		t.Fatalf("TargetChannel = %q, want dev", resp.TargetChannel)
	}
	if resp.DockerTag != "dev" {
		t.Fatalf("DockerTag = %q, want dev", resp.DockerTag)
	}
}

func TestGetUpdateProgressReturnsNotImplemented(t *testing.T) {
	router := gin.New()
	handler := NewHandler(&config.Config{}, filepath.Join(t.TempDir(), "config.yaml"), nil)

	router.GET("/update/progress", handler.GetUpdateProgress)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/update/progress", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("GET /update/progress status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestApplyUpdateReturnsNotImplementedWhenNoTargetAvailable(t *testing.T) {
	origFetchBranchCommit := fetchBranchCommitForUpdateCheck
	origFetchLatestRelease := fetchLatestReleaseInfoForUpdateCheck
	origBuildinfoVersion := buildinfo.Version
	origBuildinfoCommit := buildinfo.Commit
	origBuildinfoBuildDate := buildinfo.BuildDate
	origBuildinfoFrontendVersion := buildinfo.FrontendVersion
	origBuildinfoFrontendCommit := buildinfo.FrontendCommit
	origBuildinfoFrontendRef := buildinfo.FrontendRef
	t.Cleanup(func() {
		fetchBranchCommitForUpdateCheck = origFetchBranchCommit
		fetchLatestReleaseInfoForUpdateCheck = origFetchLatestRelease
		buildinfo.Version = origBuildinfoVersion
		buildinfo.Commit = origBuildinfoCommit
		buildinfo.BuildDate = origBuildinfoBuildDate
		buildinfo.FrontendVersion = origBuildinfoFrontendVersion
		buildinfo.FrontendCommit = origBuildinfoFrontendCommit
		buildinfo.FrontendRef = origBuildinfoFrontendRef
	})

	fetchBranchCommitForUpdateCheck = func(ctx context.Context, client *http.Client, repo string, channel string) (branchCommitInfo, error) {
		switch strings.TrimSpace(repo) {
		case "kittors/CliRelay":
			return branchCommitInfo{SHA: "aaaaaaaaaaaaaaaa", HTMLURL: "https://example.com/" + repo}, nil
		default:
			return branchCommitInfo{SHA: buildinfo.FrontendCommit, HTMLURL: "https://example.com/" + repo}, nil
		}
	}
	fetchLatestReleaseInfoForUpdateCheck = func(ctx context.Context, client *http.Client, repo string) (releaseInfo, error) {
		return releaseInfo{}, nil
	}

	router := gin.New()
	cfg := &config.Config{}
	cfg.AutoUpdate.Enabled = true
	cfg.AutoUpdate.Channel = "main"
	cfg.AutoUpdate.Repository = "https://github.com/kittors/CliRelay"
	handler := NewHandler(cfg, filepath.Join(t.TempDir(), "config.yaml"), nil)
	buildinfo.Version = "main-bbbbbbbb"
	buildinfo.Commit = "bbbbbbbb"
	buildinfo.BuildDate = "2026-05-10T00:00:00Z"
	buildinfo.FrontendVersion = "panel-main-bbbbbbb"
	buildinfo.FrontendCommit = "bbbbbbbb"
	buildinfo.FrontendRef = "main"

	router.POST("/update/apply", handler.ApplyUpdate)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/update/apply", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("POST /update/apply status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestFetchBranchCommitUsesGitHubTokenWhenConfigured(t *testing.T) {
	t.Setenv("CLIRELAY_GITHUB_TOKEN", "test-token")

	var gotAuth string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotAuth = req.Header.Get("Authorization")
			body := `{"sha":"abcdef1234567","html_url":"https://example.com/commit","commit":{"message":"ok"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    req,
			}, nil
		}),
	}

	info, err := fetchBranchCommit(context.Background(), client, "kittors/CliRelay", "dev")
	if err != nil {
		t.Fatalf("fetchBranchCommit() error = %v, want nil", err)
	}
	if info.SHA != "abcdef1234567" {
		t.Fatalf("SHA = %q, want abcdef1234567", info.SHA)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want Bearer test-token", gotAuth)
	}
}
