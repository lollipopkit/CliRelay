package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigDefaultsDisableControlPanel(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("port: 8317\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.RemoteManagement.DisableControlPanel {
		t.Fatalf("DisableControlPanel = true, want false by default")
	}
}

func TestSanitizeRoutingPreservesChannelGroupStrategy(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Routing: RoutingConfig{
			Strategy: "fill-first",
			ChannelGroups: []RoutingChannelGroup{
				{
					Name:     " Team ",
					Strategy: "round-robin",
					Match: ChannelGroupMatch{
						Channels: []string{"Team Channel"},
					},
				},
				{
					Name:     " Cache ",
					Strategy: "ff",
					Match: ChannelGroupMatch{
						Channels: []string{"Cache Channel"},
					},
				},
			},
		},
	}

	cfg.SanitizeRouting()

	if got := cfg.Routing.ChannelGroups[0].Strategy; got != "round-robin" {
		t.Fatalf("group strategy = %q, want round-robin", got)
	}
	if got := cfg.Routing.ChannelGroups[1].Strategy; got != "fill-first" {
		t.Fatalf("group strategy alias = %q, want fill-first", got)
	}
}

func TestLoadConfigAllowsAuthPathEnvOverride(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("auth-dir: /root/.cli-proxy-api\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("AUTH_PATH", "/CLIProxyAPI/auths")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.AuthDir != "/CLIProxyAPI/auths" {
		t.Fatalf("AuthDir = %q, want AUTH_PATH override", cfg.AuthDir)
	}
}

func TestSaveConfigPreserveCommentsOmitsDisableControlPanelWhenDefaultFalse(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("port: 8317\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := &Config{
		Port: 8317,
		RemoteManagement: RemoteManagement{
			DisableControlPanel: false,
		},
	}

	if err := SaveConfigPreserveComments(configPath, cfg); err != nil {
		t.Fatalf("SaveConfigPreserveComments returned error: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	rendered := string(data)
	if strings.Contains(rendered, "disable-control-panel:") {
		t.Fatalf("saved config unexpectedly persisted default disable-control-panel=false:\n%s", rendered)
	}
}

func TestSaveConfigPreserveCommentsKeepsDisableControlPanelTrue(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("port: 8317\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := &Config{
		Port: 8317,
		RemoteManagement: RemoteManagement{
			DisableControlPanel: true,
		},
	}

	if err := SaveConfigPreserveComments(configPath, cfg); err != nil {
		t.Fatalf("SaveConfigPreserveComments returned error: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	rendered := string(data)
	if !strings.Contains(rendered, "disable-control-panel: true") {
		t.Fatalf("saved config missing explicit true override:\n%s", rendered)
	}
}

func TestNormalizeOpenAICompatibilityResponsesMode(t *testing.T) {
	tests := map[string]string{
		"":        "bridge",
		"bridge":  "bridge",
		"BRIDGE":  "bridge",
		"native":  "native",
		" auto ":  "auto",
		"invalid": "bridge",
	}
	for in, want := range tests {
		if got := NormalizeOpenAICompatibilityResponsesMode(in); got != want {
			t.Fatalf("NormalizeOpenAICompatibilityResponsesMode(%q) = %q, want %q", in, got, want)
		}
	}
}
