package management

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/claude"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestStartCallbackForwarderOnAvailablePortFallsBackWhenPreferredBusy(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on busy port: %v", err)
	}
	defer func() { _ = busy.Close() }()

	preferredPort := busy.Addr().(*net.TCPAddr).Port
	forwarder, actualPort, err := startCallbackForwarderOnAvailablePort(preferredPort, "gemini", "http://example.test/google/callback")
	if err != nil {
		t.Fatalf("startCallbackForwarderOnAvailablePort returned error: %v", err)
	}
	defer stopCallbackForwarderInstance(context.Background(), actualPort, forwarder)

	if actualPort == preferredPort {
		t.Fatalf("actualPort = preferredPort = %d, want fallback port", actualPort)
	}

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get("http://127.0.0.1:" + strconv.Itoa(actualPort) + "/oauth2callback?code=abc&state=xyz")
	if err != nil {
		t.Fatalf("GET callback forwarder: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusFound)
	}
	location := resp.Header.Get("Location")
	if !strings.Contains(location, "http://example.test/google/callback") ||
		!strings.Contains(location, "code=abc") ||
		!strings.Contains(location, "state=xyz") {
		_ = resp.Body.Close()
		t.Fatalf("unexpected redirect location: %q", location)
	}
}

func TestRequestAnthropicTokenFallsBackToPlatformCallbackWhenLocalPortBusy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	busy, err := net.Listen("tcp", "127.0.0.1:54545")
	if err != nil {
		t.Skipf("anthropic callback port already unavailable: %v", err)
	}
	defer func() { _ = busy.Close() }()

	previousStore := oauthSessions
	oauthSessions = newOAuthSessionStore(oauthCallbackWaitTimeout)
	t.Cleanup(func() {
		oauthSessions = previousStore
	})

	h := &Handler{
		cfg: &config.Config{
			AuthDir: t.TempDir(),
			Port:    8317,
		},
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/anthropic-auth-url?is_webui=true", nil)
	c.Request = req

	h.RequestAnthropicToken(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var payload struct {
		URL   string `json:"url"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.State == "" {
		t.Fatalf("expected state in response")
	}
	if !strings.Contains(payload.URL, url.QueryEscape(claude.PlatformRedirectURI)) {
		t.Fatalf("auth URL should use platform callback fallback, got %s", payload.URL)
	}
	SetOAuthSessionError(payload.State, "test shutdown")
}
