package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/soloengine/lpg/internal/router"
)

func TestOpenAICompatibleAbstractorUsesConfiguredModelAndReturnsContent(t *testing.T) {
	type capturedRequest struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}

	var captured capturedRequest
	var authHeader string
	var path string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		authHeader = r.Header.Get("X-API-Key")
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"abstracted-output"}}]}`))
	}))
	defer srv.Close()

	abstractor, err := NewOpenAICompatibleAbstractor(OpenAICompatibleConfig{
		BaseURL:      srv.URL,
		APIKey:       "local-secret",
		Model:        "local-abstractor-model",
		APIKeyHeader: "X-API-Key",
		APIKeyPrefix: "Token",
		ChatPath:     "/local/chat",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleAbstractor failed: %v", err)
	}

	output, err := abstractor.Abstract(context.Background(), AbstractRequest{SanitizedPrompt: "sanitize me", Route: router.RouteCriticalLocalOnly})
	if err != nil {
		t.Fatalf("Abstract failed: %v", err)
	}
	if output != "abstracted-output" {
		t.Fatalf("expected abstracted output, got %q", output)
	}
	if path != "/local/chat" {
		t.Fatalf("expected custom chat path, got %q", path)
	}
	if authHeader != "Token local-secret" {
		t.Fatalf("expected custom auth header value, got %q", authHeader)
	}
	if captured.Model != "local-abstractor-model" {
		t.Fatalf("expected configured abstractor model, got %q", captured.Model)
	}
	if len(captured.Messages) != 1 || captured.Messages[0].Content != "sanitize me" {
		t.Fatalf("unexpected messages payload: %+v", captured.Messages)
	}
}

func TestOpenAICompatibleAbstractorRequiresModel(t *testing.T) {
	_, err := NewOpenAICompatibleAbstractor(OpenAICompatibleConfig{BaseURL: "http://example.invalid"})
	if err == nil {
		t.Fatal("expected error when model is missing")
	}
}

func TestOpenAICompatibleAbstractorHighRouteAddsJumbleInstructionAndCallsProvider(t *testing.T) {
	type capturedRequest struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}

	var captured capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"jumbled-output"}}]}`))
	}))
	defer srv.Close()

	abstractor, err := NewOpenAICompatibleAbstractor(OpenAICompatibleConfig{
		BaseURL: srv.URL,
		Model:   "local-model",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleAbstractor failed: %v", err)
	}

	output, err := abstractor.Abstract(context.Background(), AbstractRequest{
		SanitizedPrompt: "alpha beta gamma delta",
		Route:           router.RouteHighAbstraction,
	})
	if err != nil {
		t.Fatalf("Abstract failed: %v", err)
	}
	if output != "jumbled-output" {
		t.Fatalf("expected provider output, got %q", output)
	}
	if len(captured.Messages) != 1 {
		t.Fatalf("expected single message in request, got %d", len(captured.Messages))
	}
	if !stringContains(captured.Messages[0].Content, "Rewrite the sanitized text by jumbling word order") {
		t.Fatalf("expected jumble instruction in prompt, got %q", captured.Messages[0].Content)
	}
	if !stringContains(captured.Messages[0].Content, "alpha beta gamma delta") {
		t.Fatalf("expected original sanitized prompt in request content, got %q", captured.Messages[0].Content)
	}
}

func TestOpenAICompatibleAbstractorReturnsErrorOnProviderFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer srv.Close()

	abstractor, err := NewOpenAICompatibleAbstractor(OpenAICompatibleConfig{
		BaseURL: srv.URL,
		Model:   "local-model",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleAbstractor failed: %v", err)
	}

	if _, err := abstractor.Abstract(context.Background(), AbstractRequest{SanitizedPrompt: "x", Route: router.RouteCriticalLocalOnly}); err == nil {
		t.Fatal("expected error on non-2xx provider response")
	}
}

func stringContains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) > 0 && (len(haystack) >= len(needle)) && (stringIndexOf(haystack, needle) >= 0)
}

func stringIndexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
