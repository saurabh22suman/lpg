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

type boundaryUpstream struct {
	last proxy.ForwardRequest
}

func (u *boundaryUpstream) ChatCompletions(ctx context.Context, req proxy.ForwardRequest) (proxy.ForwardResponse, error) {
	u.last = req
	return proxy.ForwardResponse{Content: "ok"}, nil
}

func TestTVROUTE001BoundaryCorrectness(t *testing.T) {
	cases := []struct {
		name             string
		input            string
		expectedCategory risk.Category
		expectedRoute    router.Route
		expectedStatus   int
	}{
		{
			name:             "low no detections",
			input:            "hello world",
			expectedCategory: risk.CategoryLow,
			expectedRoute:    router.RouteSanitizedForward,
			expectedStatus:   http.StatusOK,
		},
		{
			name:             "medium one detection",
			input:            "reach me at alice@example.com",
			expectedCategory: risk.CategoryMedium,
			expectedRoute:    router.RouteSanitizedForward,
			expectedStatus:   http.StatusOK,
		},
		{
			name:             "high two detections",
			input:            "alice@example.com and 555-123-4567",
			expectedCategory: risk.CategoryHigh,
			expectedRoute:    router.RouteHighAbstraction,
			expectedStatus:   http.StatusOK,
		},
		{
			name:             "critical four detections",
			input:            "a@example.com b@example.com 555-123-4567 123-45-6789",
			expectedCategory: risk.CategoryCritical,
			expectedRoute:    router.RouteCriticalBlocked,
			expectedStatus:   http.StatusForbidden,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := &boundaryUpstream{}
			h := proxy.NewHandler(proxy.HandlerConfig{
				Sanitizer:  sanitizer.NewDefault(),
				Scorer:     risk.NewScorer(0.70),
				Router:     router.NewEngine(false),
				Upstream:   upstream,
				Abstractor: proxy.PassthroughAbstractor{},
			})

			body := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"` + tc.input + `"}]}`)
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
			rec := httptest.NewRecorder()

			h.HandleChatCompletions(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d", tc.expectedStatus, rec.Code)
			}

			if tc.expectedStatus == http.StatusOK {
				if upstream.last.RiskCategory != tc.expectedCategory {
					t.Fatalf("expected category %s, got %s", tc.expectedCategory, upstream.last.RiskCategory)
				}
				if upstream.last.Route != tc.expectedRoute {
					t.Fatalf("expected route %s, got %s", tc.expectedRoute, upstream.last.Route)
				}
			} else {
				if upstream.last.Route != "" {
					t.Fatalf("expected no upstream call for blocked request, got route %s", upstream.last.Route)
				}
			}
		})
	}
}
