package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/soloengine/lpg/internal/proxy"
	"github.com/soloengine/lpg/internal/risk"
	"github.com/soloengine/lpg/internal/router"
	"github.com/soloengine/lpg/internal/sanitizer"
)

type okUpstream struct{}

func (okUpstream) ChatCompletions(ctx context.Context, req proxy.ForwardRequest) (proxy.ForwardResponse, error) {
	return proxy.ForwardResponse{Content: "stub completion"}, nil
}

func TestChatCompletionsAddsRequestIDAndReturnsOpenAIStyleEnvelope(t *testing.T) {
	h := proxy.NewHandler(proxy.HandlerConfig{
		Sanitizer: sanitizer.NewDefault(),
		Scorer:    risk.NewScorer(0.70),
		Router:    router.NewEngine(false),
		Upstream:  okUpstream{},
	})

	body := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	requestID := rec.Header().Get("x-lpg-request-id")
	if requestID == "" {
		t.Fatal("missing x-lpg-request-id header")
	}

	var payload proxy.ChatCompletionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if payload.ID != requestID {
		t.Fatalf("expected response ID to match request ID header: id=%q header=%q", payload.ID, requestID)
	}
	if payload.Object != "chat.completion" {
		t.Fatalf("unexpected object type: %q", payload.Object)
	}
	if len(payload.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(payload.Choices))
	}
}
