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

type rawForwardCapturingUpstream struct {
	last proxy.ForwardRequest
}

func (u *rawForwardCapturingUpstream) ChatCompletions(ctx context.Context, req proxy.ForwardRequest) (proxy.ForwardResponse, error) {
	u.last = req
	return proxy.ForwardResponse{Content: "ok"}, nil
}

type rawForwardTestSanitizer struct{}

func (rawForwardTestSanitizer) Sanitize(input string) (sanitizer.Result, error) {
	return sanitizer.Result{Sanitized: "sanitized-value", Mappings: nil}, nil
}

func TestTVROUTE003RawForwardUsesRawPromptPayload(t *testing.T) {
	upstream := &rawForwardCapturingUpstream{}
	h := proxy.NewHandler(proxy.HandlerConfig{
		Sanitizer: rawForwardTestSanitizer{},
		Scorer:    risk.NewScorer(0.70),
		Router:    router.NewEngine(true),
		Upstream:  upstream,
	})

	rawInput := "raw prompt should be forwarded"
	body := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"` + rawInput + `"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if upstream.last.Route != router.RouteRawForward {
		t.Fatalf("expected route %s, got %s", router.RouteRawForward, upstream.last.Route)
	}
	if upstream.last.SanitizedPrompt != rawInput {
		t.Fatalf("expected raw prompt to be forwarded on raw route, got %q", upstream.last.SanitizedPrompt)
	}
}
