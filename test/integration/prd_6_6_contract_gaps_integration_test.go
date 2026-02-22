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

type prd66Upstream struct{}

func (prd66Upstream) ChatCompletions(ctx context.Context, req proxy.ForwardRequest) (proxy.ForwardResponse, error) {
	return proxy.ForwardResponse{Content: "ok"}, nil
}

type prd66ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	RequestID string `json:"request_id"`
}

func newPRD66Handler() *proxy.Handler {
	return proxy.NewHandler(proxy.HandlerConfig{
		Sanitizer: sanitizer.NewDefault(),
		Scorer:    risk.NewScorer(0.70),
		Router:    router.NewEngine(false),
		Upstream:  prd66Upstream{},
	})
}

func TestPRD66MalformedJSONReturnsValidationErrorWithRequestID(t *testing.T) {
	h := newPRD66Handler()

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"gpt-test","messages":[`))
	rec := httptest.NewRecorder()

	h.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	requestID := rec.Header().Get("x-lpg-request-id")
	if requestID == "" {
		t.Fatal("missing x-lpg-request-id header")
	}

	var payload prd66ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}

	if payload.Error.Code != "ERR_VALIDATION" {
		t.Fatalf("expected error code ERR_VALIDATION, got %q", payload.Error.Code)
	}
	if payload.Error.Message != "invalid JSON payload" {
		t.Fatalf("expected message %q, got %q", "invalid JSON payload", payload.Error.Message)
	}
	if payload.RequestID != requestID {
		t.Fatalf("expected response request_id %q to match header %q", payload.RequestID, requestID)
	}
}

func TestPRD66MissingRequiredFieldsReturnsValidationErrorWithRequestID(t *testing.T) {
	h := newPRD66Handler()

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()

	h.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	requestID := rec.Header().Get("x-lpg-request-id")
	if requestID == "" {
		t.Fatal("missing x-lpg-request-id header")
	}

	var payload prd66ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}

	if payload.Error.Code != "ERR_VALIDATION" {
		t.Fatalf("expected error code ERR_VALIDATION, got %q", payload.Error.Code)
	}
	if payload.Error.Message != "model is required" {
		t.Fatalf("expected message %q, got %q", "model is required", payload.Error.Message)
	}
	if payload.RequestID != requestID {
		t.Fatalf("expected response request_id %q to match header %q", payload.RequestID, requestID)
	}
}

func TestPRD66RequestIDPresentAcrossSuccessAndBlockedResponses(t *testing.T) {
	h := newPRD66Handler()

	t.Run("success includes request id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"gpt-test","messages":[{"role":"user","content":"hello world"}]}`))
		rec := httptest.NewRecorder()

		h.HandleChatCompletions(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		if rec.Header().Get("x-lpg-request-id") == "" {
			t.Fatal("missing x-lpg-request-id header on success response")
		}
	})

	t.Run("blocked includes request id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"gpt-test","messages":[{"role":"user","content":"a@example.com b@example.com 555-123-4567 123-45-6789"}]}`))
		rec := httptest.NewRecorder()

		h.HandleChatCompletions(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
		}

		if rec.Header().Get("x-lpg-request-id") == "" {
			t.Fatal("missing x-lpg-request-id header on blocked response")
		}
	})
}

func TestPRD66RequestIDsUniqueAcrossMultipleRequests(t *testing.T) {
	h := newPRD66Handler()

	const count = 12
	seen := make(map[string]struct{}, count)

	for i := 0; i < count; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"gpt-test","messages":[{"role":"user","content":"hello world"}]}`))
		rec := httptest.NewRecorder()

		h.HandleChatCompletions(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected status %d, got %d", i, http.StatusOK, rec.Code)
		}

		requestID := rec.Header().Get("x-lpg-request-id")
		if requestID == "" {
			t.Fatalf("request %d: missing x-lpg-request-id header", i)
		}
		if _, exists := seen[requestID]; exists {
			t.Fatalf("request %d: duplicate request id %q", i, requestID)
		}
		seen[requestID] = struct{}{}
	}

	if len(seen) != count {
		t.Fatalf("expected %d unique request IDs, got %d", count, len(seen))
	}
}
