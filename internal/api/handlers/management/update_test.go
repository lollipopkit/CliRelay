package management

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo"
)

func TestInferUpdateChannel(t *testing.T) {
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
			if got := inferUpdateChannel(tt.version, tt.env); got != tt.want {
				t.Fatalf("inferUpdateChannel(%q, %q) = %q, want %q", tt.version, tt.env, got, tt.want)
			}
		})
	}
}

func TestUpdateAvailableFromCommit(t *testing.T) {
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
			if got := updateAvailableFromCommit(tt.currentCommit, tt.latestCommit); got != tt.want {
				t.Fatalf("updateAvailableFromCommit(%q, %q) = %v, want %v", tt.currentCommit, tt.latestCommit, got, tt.want)
			}
		})
	}
}

func TestBuildUpdateCheckGracefullyHandlesGitHubFailures(t *testing.T) {
	origFetchBranchCommit := fetchBranchCommitForUpdateCheck
	origFetchLatestRelease := fetchLatestReleaseInfoForUpdateCheck
	origVersion := buildinfo.Version
	t.Cleanup(func() {
		fetchBranchCommitForUpdateCheck = origFetchBranchCommit
		fetchLatestReleaseInfoForUpdateCheck = origFetchLatestRelease
		buildinfo.Version = origVersion
	})

	fetchBranchCommitForUpdateCheck = func(context.Context, *http.Client, string, string) (branchCommitInfo, error) {
		return branchCommitInfo{}, errors.New("github rate limit exceeded")
	}
	fetchLatestReleaseInfoForUpdateCheck = func(context.Context, *http.Client, string) (releaseInfo, error) {
		return releaseInfo{}, errors.New("release unavailable")
	}
	buildinfo.Version = "dev-abcdef1"

	resp, err := (&Handler{}).buildUpdateCheck(context.Background())
	if err != nil {
		t.Fatalf("buildUpdateCheck() error = %v, want nil", err)
	}
	if resp.UpdateAvailable {
		t.Fatal("UpdateAvailable = true, want false when GitHub checks fail")
	}
	if !strings.Contains(strings.ToLower(resp.Message), "github") {
		t.Fatalf("Message = %q, want GitHub failure context", resp.Message)
	}
	if resp.TargetChannel != "dev" {
		t.Fatalf("TargetChannel = %q, want dev", resp.TargetChannel)
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

	fetchBranchCommitForUpdateCheck = func(context.Context, *http.Client, string, string) (branchCommitInfo, error) {
		t.Fatal("buildCurrentUpdateState must not query GitHub branch commits")
		return branchCommitInfo{}, nil
	}
	fetchLatestReleaseInfoForUpdateCheck = func(context.Context, *http.Client, string) (releaseInfo, error) {
		t.Fatal("buildCurrentUpdateState must not query GitHub releases")
		return releaseInfo{}, nil
	}
	buildinfo.Version = "dev-abcdef1"
	buildinfo.Commit = "abcdef123456"
	buildinfo.BuildDate = "2026-04-20T06:14:57Z"
	buildinfo.FrontendVersion = "panel-dev-fedcba9"
	buildinfo.FrontendCommit = "fedcba987654"
	buildinfo.FrontendRef = "dev"

	resp := (&Handler{}).buildCurrentUpdateState(context.Background())
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
			body := `{"sha":"abcdef1234567","html_url":"https://example.com/commit"}`
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
