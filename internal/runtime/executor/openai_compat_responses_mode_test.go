package executor

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
)

func TestOpenAICompatResponsesModeNativeUsesResponsesEndpoint(t *testing.T) {
	var gotPath string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_1","object":"response","created_at":1,"status":"completed","model":"model-a","usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`))
	}))
	defer server.Close()

	executor := NewOpenAICompatExecutor("compat", &config.Config{})
	auth := &cliproxyauth.Auth{Attributes: map[string]string{
		"base_url":       server.URL + "/v1",
		"api_key":        "test",
		"responses_mode": "native",
	}}
	payload := []byte(`{"model":"model-a","input":"hi"}`)
	resp, err := executor.Execute(context.Background(), auth, cliproxyexecutor.Request{Model: "model-a", Payload: payload}, cliproxyexecutor.Options{SourceFormat: sdktranslator.FromString("openai-response")})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if gotPath != "/v1/responses" {
		t.Fatalf("path = %q, want /v1/responses", gotPath)
	}
	if string(gotBody) != string(payload) {
		t.Fatalf("body = %s, want %s", string(gotBody), string(payload))
	}
	if string(resp.Payload) == "" {
		t.Fatal("expected response payload")
	}
}

func TestOpenAICompatResponsesModeAutoFallsBackToBridge(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/responses" {
			http.Error(w, `{"error":"responses unsupported"}`, http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"id":"chatcmpl_1","object":"chat.completion","created":1,"model":"model-a","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`))
	}))
	defer server.Close()

	executor := NewOpenAICompatExecutor("compat", &config.Config{})
	auth := &cliproxyauth.Auth{Attributes: map[string]string{
		"base_url":       server.URL + "/v1",
		"api_key":        "test",
		"responses_mode": "auto",
	}}
	_, err := executor.Execute(context.Background(), auth, cliproxyexecutor.Request{Model: "model-a", Payload: []byte(`{"model":"model-a","input":"hi"}`)}, cliproxyexecutor.Options{SourceFormat: sdktranslator.FromString("openai-response")})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if len(paths) != 2 || paths[0] != "/v1/responses" || paths[1] != "/v1/chat/completions" {
		t.Fatalf("paths = %#v, want native then bridge", paths)
	}
}
