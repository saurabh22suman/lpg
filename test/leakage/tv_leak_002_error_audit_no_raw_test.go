package leakage_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/soloengine/lpg/internal/audit"
	"github.com/soloengine/lpg/internal/proxy"
	"github.com/soloengine/lpg/internal/risk"
	"github.com/soloengine/lpg/internal/router"
	"github.com/soloengine/lpg/internal/sanitizer"
)

type providerFailureUpstream struct{}

func (providerFailureUpstream) ChatCompletions(ctx context.Context, req proxy.ForwardRequest) (proxy.ForwardResponse, error) {
	return proxy.ForwardResponse{}, errors.New("upstream failure")
}

type providerTimeoutUpstream struct{}

func (providerTimeoutUpstream) ChatCompletions(ctx context.Context, req proxy.ForwardRequest) (proxy.ForwardResponse, error) {
	<-ctx.Done()
	return proxy.ForwardResponse{}, ctx.Err()
}

func TestTVLEAK002ErrorResponsesDoNotEchoRawSensitiveValuesOnProviderFailurePaths(t *testing.T) {
	rawEmail := "alice@example.com"
	body := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"contact ` + rawEmail + `"}]}`)

	tests := []struct {
		name         string
		upstream     proxy.UpstreamAdapter
		expectedCode int
		errorCode    string
	}{
		{
			name:         "provider failure returns policy-safe error",
			upstream:     providerFailureUpstream{},
			expectedCode: http.StatusBadGateway,
			errorCode:    "ERR_PROVIDER_FAILURE",
		},
		{
			name:         "provider timeout returns policy-safe error",
			upstream:     providerTimeoutUpstream{},
			expectedCode: http.StatusServiceUnavailable,
			errorCode:    "ERR_PROVIDER_TIMEOUT",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := proxy.NewHandler(proxy.HandlerConfig{
				Sanitizer:       sanitizer.NewDefault(),
				Scorer:          risk.NewScorer(0.70),
				Router:          router.NewEngine(false),
				Upstream:        tc.upstream,
				ProviderTimeout: 10 * time.Millisecond,
			})

			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
			rec := httptest.NewRecorder()

			h.HandleChatCompletions(rec, req)

			if rec.Code != tc.expectedCode {
				t.Fatalf("expected status %d, got %d", tc.expectedCode, rec.Code)
			}

			var payload struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
				RequestID string `json:"request_id"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if payload.Error.Code != tc.errorCode {
				t.Fatalf("expected error code %q, got %q", tc.errorCode, payload.Error.Code)
			}

			responseBody := rec.Body.String()
			assertNoRawSensitive(t, responseBody, rawEmail)
			assertNoRawSensitive(t, payload.Error.Message, rawEmail)
			assertNoRawSensitive(t, payload.RequestID, rawEmail)
		})
	}
}

func TestTVLEAK003AuditRecordsDoNotContainRawSensitiveRequestContentByDefault(t *testing.T) {
	rawEmail := "alice@example.com"
	auditPath := filepath.Join(t.TempDir(), "audit.log")

	chainWriter, err := audit.NewChainWriter(auditPath)
	if err != nil {
		t.Fatalf("NewChainWriter failed: %v", err)
	}

	h := proxy.NewHandler(proxy.HandlerConfig{
		Sanitizer:     sanitizer.NewDefault(),
		Scorer:        risk.NewScorer(0.70),
		Router:        router.NewEngine(false),
		Upstream:      providerFailureUpstream{},
		Audit:         chainWriter,
		StrictAudit:   true,
		PolicyVersion: "v2.1",
	})

	body := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"contact ` + rawEmail + `"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d", http.StatusBadGateway, rec.Code)
	}

	contents, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}
	logText := string(contents)

	assertNoRawSensitive(t, logText, rawEmail)

	scanner := bufio.NewScanner(strings.NewReader(logText))
	if !scanner.Scan() {
		t.Fatal("expected at least one audit record")
	}

	var record audit.Record
	if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
		t.Fatalf("failed to unmarshal audit record: %v", err)
	}

	if record.ActionSummary == "" {
		t.Fatal("expected non-empty action summary")
	}
	if !strings.Contains(record.ActionSummary, "provider-failure") {
		t.Fatalf("expected provider failure action summary, got %q", record.ActionSummary)
	}
	assertNoRawSensitive(t, record.ActionSummary, rawEmail)
}

func assertNoRawSensitive(t *testing.T, haystack string, sensitiveValues ...string) {
	t.Helper()
	for _, value := range sensitiveValues {
		if strings.Contains(haystack, value) {
			t.Fatalf("raw sensitive value leaked: %q in %q", value, haystack)
		}
	}
}
