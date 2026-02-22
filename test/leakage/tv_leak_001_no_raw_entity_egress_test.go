package leakage_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/soloengine/lpg/internal/proxy"
	"github.com/soloengine/lpg/internal/risk"
	"github.com/soloengine/lpg/internal/router"
	"github.com/soloengine/lpg/internal/sanitizer"
)

type capturingUpstream struct {
	last proxy.ForwardRequest
}

func (c *capturingUpstream) ChatCompletions(ctx context.Context, req proxy.ForwardRequest) (proxy.ForwardResponse, error) {
	c.last = req
	return proxy.ForwardResponse{Content: "ok"}, nil
}

func TestTVLEAK001NoForbiddenRawEntitiesCrossOutboundBoundary(t *testing.T) {
	upstream := &capturingUpstream{}
	h := proxy.NewHandler(proxy.HandlerConfig{
		Sanitizer: sanitizer.NewDefault(),
		Scorer:    risk.NewScorer(0.70),
		Router:    router.NewEngine(false),
		Upstream:  upstream,
	})

	body := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"email alice@example.com"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if strings.Contains(upstream.last.SanitizedPrompt, "alice@example.com") {
		t.Fatalf("raw email leaked to outbound payload: %q", upstream.last.SanitizedPrompt)
	}
	if !strings.Contains(upstream.last.SanitizedPrompt, "person1@example.net") {
		t.Fatalf("expected surrogate email in outbound payload: %q", upstream.last.SanitizedPrompt)
	}
}
