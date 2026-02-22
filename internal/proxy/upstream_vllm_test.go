package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestVLLMUpstreamSendsExpectedRequestAndUsesRequestModel(t *testing.T) {
	type capturedRequest struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}

	var captured capturedRequest
	var idempotencyKey string
	var authHeader string
	var path string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		idempotencyKey = r.Header.Get("Idempotency-Key")
		authHeader = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	upstream, err := NewVLLMUpstream(srv.URL, "fallback-model")
	if err != nil {
		t.Fatalf("NewVLLMUpstream failed: %v", err)
	}

	resp, err := upstream.ChatCompletions(context.Background(), ForwardRequest{
		Model:           "request-model",
		SanitizedPrompt: "hello world",
		IdempotencyKey:  "idem-1",
	})
	if err != nil {
		t.Fatalf("ChatCompletions failed: %v", err)
	}

	if resp.Content != "ok" {
		t.Fatalf("expected content %q, got %q", "ok", resp.Content)
	}
	if path != defaultProviderChatCompletionsPath {
		t.Fatalf("expected default path %q, got %q", defaultProviderChatCompletionsPath, path)
	}
	if captured.Model != "request-model" {
		t.Fatalf("expected request model to be used, got %q", captured.Model)
	}
	if len(captured.Messages) != 1 || captured.Messages[0].Role != "user" || captured.Messages[0].Content != "hello world" {
		t.Fatalf("unexpected messages payload: %+v", captured.Messages)
	}
	if idempotencyKey != "idem-1" {
		t.Fatalf("expected idempotency key to be forwarded, got %q", idempotencyKey)
	}
	if authHeader != "" {
		t.Fatalf("did not expect authorization header for vLLM, got %q", authHeader)
	}
}

func TestVLLMUpstreamUsesFallbackModelWhenRequestModelMissing(t *testing.T) {
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

	upstream, err := NewVLLMUpstream(srv.URL, "fallback-model")
	if err != nil {
		t.Fatalf("NewVLLMUpstream failed: %v", err)
	}

	_, err = upstream.ChatCompletions(context.Background(), ForwardRequest{SanitizedPrompt: "ping"})
	if err != nil {
		t.Fatalf("ChatCompletions failed: %v", err)
	}
	if capturedModel != "fallback-model" {
		t.Fatalf("expected fallback model, got %q", capturedModel)
	}
}

func TestVLLMUpstreamReturnsErrorOnProviderNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer srv.Close()

	upstream, err := NewVLLMUpstream(srv.URL, "fallback-model")
	if err != nil {
		t.Fatalf("NewVLLMUpstream failed: %v", err)
	}

	if _, err := upstream.ChatCompletions(context.Background(), ForwardRequest{SanitizedPrompt: "ping"}); err == nil {
		t.Fatal("expected error on non-2xx response")
	}
}

func TestVLLMUpstreamReturnsErrorOnInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer srv.Close()

	upstream, err := NewVLLMUpstream(srv.URL, "fallback-model")
	if err != nil {
		t.Fatalf("NewVLLMUpstream failed: %v", err)
	}

	if _, err := upstream.ChatCompletions(context.Background(), ForwardRequest{SanitizedPrompt: "ping"}); err == nil {
		t.Fatal("expected error on invalid JSON response")
	}
}

func TestVLLMUpstreamReturnsErrorOnContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	upstream, err := NewVLLMUpstream(srv.URL, "fallback-model")
	if err != nil {
		t.Fatalf("NewVLLMUpstream failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	if _, err := upstream.ChatCompletions(ctx, ForwardRequest{SanitizedPrompt: "ping"}); err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestVLLMUpstreamReturnsErrorWhenModelMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called when model is missing")
	}))
	defer srv.Close()

	upstream, err := NewVLLMUpstream(srv.URL, "")
	if err != nil {
		t.Fatalf("NewVLLMUpstream failed: %v", err)
	}

	if _, err := upstream.ChatCompletions(context.Background(), ForwardRequest{SanitizedPrompt: "ping"}); err == nil {
		t.Fatal("expected error when request and fallback model are both missing")
	}
}
