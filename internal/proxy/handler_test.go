package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/soloengine/lpg/internal/risk"
	"github.com/soloengine/lpg/internal/router"
	"github.com/soloengine/lpg/internal/sanitizer"
)

type countingUpstreamAdapter struct {
	calls int
}

func (u *countingUpstreamAdapter) ChatCompletions(ctx context.Context, req ForwardRequest) (ForwardResponse, error) {
	u.calls++
	return ForwardResponse{Content: "unexpected"}, nil
}

type fixedSanitizer struct {
	result sanitizer.Result
}

func (s fixedSanitizer) Sanitize(input string) (sanitizer.Result, error) {
	return s.result, nil
}

func TestCriticalLocalOnlyReturnsAbstractionWithoutRemoteEgress(t *testing.T) {
	upstream := &countingUpstreamAdapter{}
	h := NewHandler(HandlerConfig{
		Sanitizer: fixedSanitizer{result: sanitizer.Result{
			Sanitized: "person1@example.net person2@example.net 555-010-0001 900-00-0001",
			Mappings: []sanitizer.Mapping{
				{Placeholder: "person1@example.net", OriginalValue: "a@example.com", EntityType: "EMAIL", ConfidenceScore: 0.99},
				{Placeholder: "person2@example.net", OriginalValue: "b@example.com", EntityType: "EMAIL", ConfidenceScore: 0.99},
				{Placeholder: "555-010-0001", OriginalValue: "555-123-4567", EntityType: "PHONE", ConfidenceScore: 0.99},
				{Placeholder: "900-00-0001", OriginalValue: "123-45-6789", EntityType: "SSN", ConfidenceScore: 0.99},
			},
		}},
		Scorer:     risk.NewScorer(0.70),
		Router:     router.NewEngineWithCriticalLocalOnly(false, true),
		Upstream:   upstream,
		Abstractor: PassthroughAbstractor{},
	})

	body := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"test"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if upstream.calls != 0 {
		t.Fatalf("expected no upstream calls, got %d", upstream.calls)
	}

	var payload ChatCompletionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(payload.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(payload.Choices))
	}
	if payload.Choices[0].Message.Content != "person1@example.net person2@example.net 555-010-0001 900-00-0001" {
		t.Fatalf("unexpected local-only output %q", payload.Choices[0].Message.Content)
	}
}

func TestCriticalLocalOnlyFailsWhenAbstractionMissing(t *testing.T) {
	h := NewHandler(HandlerConfig{
		Sanitizer: fixedSanitizer{result: sanitizer.Result{
			Sanitized: "person1@example.net person2@example.net 555-010-0001 900-00-0001",
			Mappings: []sanitizer.Mapping{
				{Placeholder: "person1@example.net", OriginalValue: "a@example.com", EntityType: "EMAIL", ConfidenceScore: 0.99},
				{Placeholder: "person2@example.net", OriginalValue: "b@example.com", EntityType: "EMAIL", ConfidenceScore: 0.99},
				{Placeholder: "555-010-0001", OriginalValue: "555-123-4567", EntityType: "PHONE", ConfidenceScore: 0.99},
				{Placeholder: "900-00-0001", OriginalValue: "123-45-6789", EntityType: "SSN", ConfidenceScore: 0.99},
			},
		}},
		Scorer: risk.NewScorer(0.70),
		Router: router.NewEngineWithCriticalLocalOnly(false, true),
	})

	body := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"test"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}

	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}
	if payload.Error.Code != "ERR_ABSTRACTION_UNAVAILABLE" {
		t.Fatalf("expected ERR_ABSTRACTION_UNAVAILABLE, got %q", payload.Error.Code)
	}
}

func TestHandleDebugExplainReturnsRoutingExplanation(t *testing.T) {
	h := NewHandler(HandlerConfig{
		Sanitizer: sanitizer.NewDefault(),
		Scorer:    risk.NewScorer(0.70),
		Router:    router.NewEngineWithCriticalLocalOnly(false, true),
	})

	body := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"alice@example.com and 555-123-4567"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/debug/explain", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleDebugExplain(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload ExplainResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to unmarshal explain response: %v", err)
	}

	if payload.Model != "gpt-test" {
		t.Fatalf("expected model gpt-test, got %q", payload.Model)
	}
	if payload.RiskCategory != risk.CategoryHigh {
		t.Fatalf("expected category %s, got %s", risk.CategoryHigh, payload.RiskCategory)
	}
	if payload.Route != router.RouteHighAbstraction {
		t.Fatalf("expected route %s, got %s", router.RouteHighAbstraction, payload.Route)
	}
	if !payload.Egress {
		t.Fatal("expected egress true for high abstraction route")
	}
	if payload.Detections != 2 {
		t.Fatalf("expected 2 detections, got %d", payload.Detections)
	}
	if len(payload.Mappings) != 2 {
		t.Fatalf("expected 2 mappings, got %d", len(payload.Mappings))
	}
	if payload.SanitizedInput == "" {
		t.Fatal("expected sanitized input to be populated")
	}
}

func TestHandleDebugExplainRejectsInvalidMethod(t *testing.T) {
	h := NewHandler(HandlerConfig{})
	req := httptest.NewRequest(http.MethodGet, "/v1/debug/explain", nil)
	rec := httptest.NewRecorder()

	h.HandleDebugExplain(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}
