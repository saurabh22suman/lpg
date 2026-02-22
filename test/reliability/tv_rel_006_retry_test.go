package reliability_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/soloengine/lpg/internal/proxy"
	"github.com/soloengine/lpg/internal/risk"
	"github.com/soloengine/lpg/internal/router"
	"github.com/soloengine/lpg/internal/sanitizer"
)

type failOnceThenOKUpstream struct {
	calls int
	last  proxy.ForwardRequest
}

func (u *failOnceThenOKUpstream) ChatCompletions(ctx context.Context, req proxy.ForwardRequest) (proxy.ForwardResponse, error) {
	u.calls++
	u.last = req
	if u.calls == 1 {
		return proxy.ForwardResponse{}, errors.New("transient upstream failure")
	}
	return proxy.ForwardResponse{Content: "ok"}, nil
}

type failAlwaysUpstream struct {
	calls int
	last  proxy.ForwardRequest
}

func (u *failAlwaysUpstream) ChatCompletions(ctx context.Context, req proxy.ForwardRequest) (proxy.ForwardResponse, error) {
	u.calls++
	u.last = req
	return proxy.ForwardResponse{}, errors.New("upstream failure")
}

func TestTVREL006IdempotentLowMediumRetryExactlyOnceThenSucceeds(t *testing.T) {
	cases := []struct {
		name   string
		router *router.Engine
		body   string
	}{
		{
			name:   "low route retries once",
			router: router.NewEngine(true),
			body:   `{"model":"gpt-test","messages":[{"role":"user","content":"hello world"}]}`,
		},
		{
			name:   "medium route retries once",
			router: router.NewEngine(false),
			body:   `{"model":"gpt-test","messages":[{"role":"user","content":"email alice@example.com"}]}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := &failOnceThenOKUpstream{}
			h := proxy.NewHandler(proxy.HandlerConfig{
				Sanitizer: sanitizer.NewDefault(),
				Scorer:    risk.NewScorer(0.70),
				Router:    tc.router,
				Upstream:  upstream,
			})

			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(tc.body))
			req.Header.Set("Idempotency-Key", "idem-123")
			rec := httptest.NewRecorder()

			h.HandleChatCompletions(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
			}
			if upstream.calls != 2 {
				t.Fatalf("expected exactly 2 upstream calls (1 retry), got %d", upstream.calls)
			}
		})
	}
}

func TestTVREL006LowMediumWithoutIdempotencyKeyDoesNotRetry(t *testing.T) {
	cases := []struct {
		name   string
		router *router.Engine
		body   string
	}{
		{
			name:   "low route no retry without key",
			router: router.NewEngine(true),
			body:   `{"model":"gpt-test","messages":[{"role":"user","content":"hello world"}]}`,
		},
		{
			name:   "medium route no retry without key",
			router: router.NewEngine(false),
			body:   `{"model":"gpt-test","messages":[{"role":"user","content":"email alice@example.com"}]}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := &failAlwaysUpstream{}
			h := proxy.NewHandler(proxy.HandlerConfig{
				Sanitizer: sanitizer.NewDefault(),
				Scorer:    risk.NewScorer(0.70),
				Router:    tc.router,
				Upstream:  upstream,
			})

			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(tc.body))
			rec := httptest.NewRecorder()

			h.HandleChatCompletions(rec, req)

			if rec.Code != http.StatusBadGateway {
				t.Fatalf("expected status %d, got %d", http.StatusBadGateway, rec.Code)
			}
			if upstream.calls != 1 {
				t.Fatalf("expected 1 upstream call without idempotency key, got %d", upstream.calls)
			}

			var payload struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if payload.Error.Code != "ERR_PROVIDER_FAILURE" {
				t.Fatalf("expected ERR_PROVIDER_FAILURE, got %q", payload.Error.Code)
			}
		})
	}
}

func TestTVREL006HighRouteDoesNotRetryEvenWithIdempotencyKey(t *testing.T) {
	upstream := &failAlwaysUpstream{}
	h := proxy.NewHandler(proxy.HandlerConfig{
		Sanitizer:  sanitizer.NewDefault(),
		Scorer:     risk.NewScorer(0.70),
		Router:     router.NewEngine(false),
		Upstream:   upstream,
		Abstractor: proxy.PassthroughAbstractor{},
	})

	body := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"alice@example.com and 555-123-4567"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Idempotency-Key", "idem-high")
	rec := httptest.NewRecorder()

	h.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d", http.StatusBadGateway, rec.Code)
	}
	if upstream.calls != 1 {
		t.Fatalf("expected no retry for high route, got %d upstream calls", upstream.calls)
	}
	if upstream.last.Route != router.RouteHighAbstraction {
		t.Fatalf("expected high abstraction route, got %s", upstream.last.Route)
	}
}
