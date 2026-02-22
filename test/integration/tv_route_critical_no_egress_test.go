package integration_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/soloengine/lpg/internal/proxy"
	"github.com/soloengine/lpg/internal/risk"
	"github.com/soloengine/lpg/internal/router"
	"github.com/soloengine/lpg/internal/sanitizer"
)

type countingUpstream struct {
	calls int
}

func (u *countingUpstream) ChatCompletions(ctx context.Context, req proxy.ForwardRequest) (proxy.ForwardResponse, error) {
	u.calls++
	return proxy.ForwardResponse{Content: "unexpected"}, nil
}

func TestCriticalRouteNoEgress(t *testing.T) {
	upstream := &countingUpstream{}
	h := proxy.NewHandler(proxy.HandlerConfig{
		Sanitizer: sanitizer.NewDefault(),
		Scorer:    risk.NewScorer(0.70),
		Router:    router.NewEngine(false),
		Upstream:  upstream,
	})

	// 4 detections => score 100 => Critical route
	body := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"a@example.com b@example.com 555-123-4567 123-45-6789"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}

	if upstream.calls != 0 {
		t.Fatalf("expected no outbound calls for critical route, got %d", upstream.calls)
	}
}

func TestCriticalRouteLocalOnlyNoRemoteCall(t *testing.T) {
	upstream := &countingUpstream{}
	h := proxy.NewHandler(proxy.HandlerConfig{
		Sanitizer:  sanitizer.NewDefault(),
		Scorer:     risk.NewScorer(0.70),
		Router:     router.NewEngineWithCriticalLocalOnly(false, true),
		Upstream:   upstream,
		Abstractor: proxy.PassthroughAbstractor{},
	})

	body := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"a@example.com b@example.com 555-123-4567 123-45-6789"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if upstream.calls != 0 {
		t.Fatalf("expected no outbound calls for critical local-only route, got %d", upstream.calls)
	}
}
