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

type routeCapturingUpstream struct {
	last proxy.ForwardRequest
}

func (u *routeCapturingUpstream) ChatCompletions(ctx context.Context, req proxy.ForwardRequest) (proxy.ForwardResponse, error) {
	u.last = req
	return proxy.ForwardResponse{Content: "ok"}, nil
}

func TestTVROUTE002ConfidenceEscalationUsesHighRoute(t *testing.T) {
	upstream := &routeCapturingUpstream{}

	h := proxy.NewHandler(proxy.HandlerConfig{
		Sanitizer:  &lowConfidenceSanitizer{},
		Scorer:     risk.NewScorer(0.70),
		Router:     router.NewEngine(false),
		Upstream:   upstream,
		Abstractor: proxy.PassthroughAbstractor{},
	})

	body := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if upstream.last.Route != router.RouteHighAbstraction {
		t.Fatalf("expected route %s, got %s", router.RouteHighAbstraction, upstream.last.Route)
	}
	if upstream.last.RiskCategory != risk.CategoryHigh {
		t.Fatalf("expected category %s, got %s", risk.CategoryHigh, upstream.last.RiskCategory)
	}
}

type lowConfidenceSanitizer struct{}

func (lowConfidenceSanitizer) Sanitize(input string) (sanitizer.Result, error) {
	return sanitizer.Result{
		Sanitized: "person1@example.net",
		Mappings: []sanitizer.Mapping{
			{
				Placeholder:     "person1@example.net",
				OriginalValue:   "alice@example.com",
				EntityType:      "EMAIL",
				ConfidenceScore: 0.60,
			},
		},
	}, nil
}
