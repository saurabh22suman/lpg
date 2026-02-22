package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMimoUpstreamAddsAuthorizationHeaderAndUsesFallbackModel(t *testing.T) {
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
		authHeader = r.Header.Get("api-key")
		idempotencyKey = r.Header.Get("Idempotency-Key")
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"mimo-ok"}}]}`))
	}))
	defer srv.Close()

	upstream, err := NewMimoUpstream(srv.URL, "secret-key", "mimo-v2-flash")
	if err != nil {
		t.Fatalf("NewMimoUpstream failed: %v", err)
	}

	resp, err := upstream.ChatCompletions(context.Background(), ForwardRequest{
		SanitizedPrompt: "hello mimo",
		IdempotencyKey:  "idem-mimo",
	})
	if err != nil {
		t.Fatalf("ChatCompletions failed: %v", err)
	}

	if resp.Content != "mimo-ok" {
		t.Fatalf("expected mimo response content, got %q", resp.Content)
	}
	if authHeader != "secret-key" {
		t.Fatalf("expected raw api-key auth header, got %q", authHeader)
	}
	if idempotencyKey != "idem-mimo" {
		t.Fatalf("expected idempotency key header, got %q", idempotencyKey)
	}
	if captured.Model != "mimo-v2-flash" {
		t.Fatalf("expected fallback model, got %q", captured.Model)
	}
	if path != mimoChatCompletionsPath {
		t.Fatalf("expected mimo path %q, got %q", mimoChatCompletionsPath, path)
	}
}

func TestMimoUpstreamUsesConfiguredModelEvenWhenRequestModelProvided(t *testing.T) {
	var capturedModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		capturedModel = req.Model
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	upstream, err := NewMimoUpstream(srv.URL, "secret-key", "mimo-v2-flash")
	if err != nil {
		t.Fatalf("NewMimoUpstream failed: %v", err)
	}

	_, err = upstream.ChatCompletions(context.Background(), ForwardRequest{Model: "qwen2.5:3b", SanitizedPrompt: "hi"})
	if err != nil {
		t.Fatalf("ChatCompletions failed: %v", err)
	}
	if capturedModel != "mimo-v2-flash" {
		t.Fatalf("expected configured Mimo model, got %q", capturedModel)
	}
}

func TestMimoUpstreamUsesRequestModelWhenConfiguredModelMissing(t *testing.T) {
	var capturedModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		capturedModel = req.Model
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	upstream, err := NewMimoUpstream(srv.URL, "secret-key", "")
	if err != nil {
		t.Fatalf("NewMimoUpstream failed: %v", err)
	}

	_, err = upstream.ChatCompletions(context.Background(), ForwardRequest{Model: "request-model", SanitizedPrompt: "hi"})
	if err != nil {
		t.Fatalf("ChatCompletions failed: %v", err)
	}
	if capturedModel != "request-model" {
		t.Fatalf("expected request model when configured Mimo model is empty, got %q", capturedModel)
	}
}

func TestMimoUpstreamReturnsErrorOnProviderFailures(t *testing.T) {
	t.Run("non-2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}))
		defer srv.Close()

		upstream, err := NewMimoUpstream(srv.URL, "secret-key", "fallback-model")
		if err != nil {
			t.Fatalf("NewMimoUpstream failed: %v", err)
		}

		if _, err := upstream.ChatCompletions(context.Background(), ForwardRequest{SanitizedPrompt: "hi"}); err == nil {
			t.Fatal("expected error on non-2xx")
		}
	})

	t.Run("invalid-json", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"choices":`))
		}))
		defer srv.Close()

		upstream, err := NewMimoUpstream(srv.URL, "secret-key", "fallback-model")
		if err != nil {
			t.Fatalf("NewMimoUpstream failed: %v", err)
		}

		if _, err := upstream.ChatCompletions(context.Background(), ForwardRequest{SanitizedPrompt: "hi"}); err == nil {
			t.Fatal("expected error on invalid JSON")
		}
	})
}

func TestMimoUpstreamReturnsErrorOnContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	upstream, err := NewMimoUpstream(srv.URL, "secret-key", "fallback-model")
	if err != nil {
		t.Fatalf("NewMimoUpstream failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	if _, err := upstream.ChatCompletions(ctx, ForwardRequest{SanitizedPrompt: "hi"}); err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestMimoUpstreamReturnsErrorWhenModelMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called when model is missing")
	}))
	defer srv.Close()

	upstream, err := NewMimoUpstream(srv.URL, "secret-key", "")
	if err != nil {
		t.Fatalf("NewMimoUpstream failed: %v", err)
	}

	if _, err := upstream.ChatCompletions(context.Background(), ForwardRequest{SanitizedPrompt: "hi"}); err == nil {
		t.Fatal("expected error when request and fallback model are both missing")
	}
}
