package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAICompatibleUpstreamSupportsCustomHeadersAndPath(t *testing.T) {
	type capturedRequest struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}

	var captured capturedRequest
	var authHeader string
	var idempotencyKey string
	var path string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		authHeader = r.Header.Get("X-API-Key")
		idempotencyKey = r.Header.Get("Idempotency-Key")
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"generic-ok"}}]}`))
	}))
	defer srv.Close()

	upstream, err := NewOpenAICompatibleUpstream(OpenAICompatibleConfig{
		BaseURL:      srv.URL,
		APIKey:       "secret-key",
		Model:        "fallback-model",
		APIKeyHeader: "X-API-Key",
		APIKeyPrefix: "Token",
		ChatPath:     "/custom/chat",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleUpstream failed: %v", err)
	}

	resp, err := upstream.ChatCompletions(context.Background(), ForwardRequest{
		SanitizedPrompt: "hello generic",
		IdempotencyKey:  "idem-generic",
	})
	if err != nil {
		t.Fatalf("ChatCompletions failed: %v", err)
	}

	if resp.Content != "generic-ok" {
		t.Fatalf("expected generic response content, got %q", resp.Content)
	}
	if path != "/custom/chat" {
		t.Fatalf("expected custom path, got %q", path)
	}
	if authHeader != "Token secret-key" {
		t.Fatalf("expected custom auth header value, got %q", authHeader)
	}
	if idempotencyKey != "idem-generic" {
		t.Fatalf("expected idempotency key header, got %q", idempotencyKey)
	}
	if captured.Model != "fallback-model" {
		t.Fatalf("expected fallback model, got %q", captured.Model)
	}
}

func TestOpenAICompatibleUpstreamSupportsRawAPIKeyHeaderWithoutPrefix(t *testing.T) {
	var authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("X-API-Key")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	upstream, err := NewOpenAICompatibleUpstream(OpenAICompatibleConfig{
		BaseURL:      srv.URL,
		APIKey:       "secret-key",
		Model:        "fallback-model",
		APIKeyHeader: "X-API-Key",
		APIKeyPrefix: "",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleUpstream failed: %v", err)
	}

	if _, err := upstream.ChatCompletions(context.Background(), ForwardRequest{SanitizedPrompt: "hi"}); err != nil {
		t.Fatalf("ChatCompletions failed: %v", err)
	}
	if authHeader != "secret-key" {
		t.Fatalf("expected raw api key header value, got %q", authHeader)
	}
}

func TestOpenAICompatibleUpstreamNormalizesChatPath(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	upstream, err := NewOpenAICompatibleUpstream(OpenAICompatibleConfig{
		BaseURL:  srv.URL,
		Model:    "fallback-model",
		ChatPath: "relative/chat",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleUpstream failed: %v", err)
	}

	if _, err := upstream.ChatCompletions(context.Background(), ForwardRequest{SanitizedPrompt: "hi"}); err != nil {
		t.Fatalf("ChatCompletions failed: %v", err)
	}
	if path != "/relative/chat" {
		t.Fatalf("expected normalized path, got %q", path)
	}
}
