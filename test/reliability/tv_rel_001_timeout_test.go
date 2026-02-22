package reliability_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/soloengine/lpg/internal/audit"
	"github.com/soloengine/lpg/internal/proxy"
	"github.com/soloengine/lpg/internal/risk"
	"github.com/soloengine/lpg/internal/router"
	"github.com/soloengine/lpg/internal/sanitizer"
)

type timeoutUpstream struct{}

func (timeoutUpstream) ChatCompletions(ctx context.Context, req proxy.ForwardRequest) (proxy.ForwardResponse, error) {
	<-ctx.Done()
	return proxy.ForwardResponse{}, ctx.Err()
}

type capturingUpstream struct {
	last proxy.ForwardRequest
}

func (u *capturingUpstream) ChatCompletions(ctx context.Context, req proxy.ForwardRequest) (proxy.ForwardResponse, error) {
	u.last = req
	return proxy.ForwardResponse{Content: "ok"}, nil
}

func TestTVREL001ProviderTimeoutSafeHandling(t *testing.T) {
	h := proxy.NewHandler(proxy.HandlerConfig{
		Sanitizer:       sanitizer.NewDefault(),
		Scorer:          risk.NewScorer(0.70),
		Router:          router.NewEngine(false),
		Upstream:        timeoutUpstream{},
		ProviderTimeout: 10 * time.Millisecond,
	})

	body := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"contact alice@example.com"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}

	if rec.Header().Get("x-lpg-request-id") == "" {
		t.Fatal("x-lpg-request-id header was not set")
	}

	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if payload.Error.Code != "ERR_PROVIDER_TIMEOUT" {
		t.Fatalf("expected ERR_PROVIDER_TIMEOUT, got %q", payload.Error.Code)
	}
}

type failingAuditWriter struct {
	err error
}

func (f failingAuditWriter) Append(event audit.Event) (audit.Record, error) {
	return audit.Record{}, f.err
}

func TestTVREL002AuditFailureStrictModeBlocks(t *testing.T) {
	h := proxy.NewHandler(proxy.HandlerConfig{
		Sanitizer:     sanitizer.NewDefault(),
		Scorer:        risk.NewScorer(0.70),
		Router:        router.NewEngine(false),
		Upstream:      proxy.StubUpstream{},
		Audit:         failingAuditWriter{err: errors.New("audit down")},
		StrictAudit:   true,
		PolicyVersion: "v2.1",
	})

	body := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if payload.Error.Code != "ERR_AUDIT_FAILURE" {
		t.Fatalf("expected ERR_AUDIT_FAILURE, got %q", payload.Error.Code)
	}
}

func TestTVREL003AuditFailureNonStrictContinues(t *testing.T) {
	h := proxy.NewHandler(proxy.HandlerConfig{
		Sanitizer:     sanitizer.NewDefault(),
		Scorer:        risk.NewScorer(0.70),
		Router:        router.NewEngine(false),
		Upstream:      proxy.StubUpstream{},
		Audit:         failingAuditWriter{err: errors.New("audit down")},
		StrictAudit:   false,
		PolicyVersion: "v2.1",
	})

	body := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestTVREL004IdempotencyKeyAcceptedForSafeRetryPath(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "audit.log")
	chainWriter, err := audit.NewChainWriter(auditPath)
	if err != nil {
		t.Fatalf("NewChainWriter failed: %v", err)
	}

	upstream := &capturingUpstream{}
	h := proxy.NewHandler(proxy.HandlerConfig{
		Sanitizer:     sanitizer.NewDefault(),
		Scorer:        risk.NewScorer(0.70),
		Router:        router.NewEngine(false),
		Upstream:      upstream,
		Audit:         chainWriter,
		StrictAudit:   true,
		PolicyVersion: "v2.1",
	})

	body := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Idempotency-Key", "abc-123")
	rec := httptest.NewRecorder()

	h.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if rec.Header().Get("x-lpg-request-id") == "" {
		t.Fatal("x-lpg-request-id header was not set")
	}

	if upstream.last.IdempotencyKey != "abc-123" {
		t.Fatalf("expected idempotency key %q, got %q", "abc-123", upstream.last.IdempotencyKey)
	}
}

func TestTVREL005IdempotencyKeyTrimmedBeforeForwarding(t *testing.T) {
	upstream := &capturingUpstream{}
	h := proxy.NewHandler(proxy.HandlerConfig{
		Sanitizer: sanitizer.NewDefault(),
		Scorer:    risk.NewScorer(0.70),
		Router:    router.NewEngine(false),
		Upstream:  upstream,
	})

	body := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Idempotency-Key", "  key-with-space  ")
	rec := httptest.NewRecorder()

	h.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if upstream.last.IdempotencyKey != "key-with-space" {
		t.Fatalf("expected trimmed idempotency key, got %q", upstream.last.IdempotencyKey)
	}
}
