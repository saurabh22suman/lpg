package proxy

import (
	"errors"
	"net/http"
	"testing"
)

func TestSafeBodySnippetRedactsSensitiveFieldsAndValues(t *testing.T) {
	input := []byte(`{"error":"invalid api key","api_key":"sk-secret-value","email":"alice@example.com","phone":"555-123-4567","ssn":"123-45-6789"}`)

	snippet := safeBodySnippet(input)

	if snippet == "" {
		t.Fatal("expected non-empty snippet")
	}
	for _, forbidden := range []string{"sk-secret-value", "alice@example.com", "555-123-4567", "123-45-6789"} {
		if contains(snippet, forbidden) {
			t.Fatalf("expected snippet to redact %q, got %q", forbidden, snippet)
		}
	}
}

func TestSafeProviderDiagnosticIncludesStatusSnippetAndSafeHeaders(t *testing.T) {
	err := &ProviderHTTPStatusError{
		StatusCode:  401,
		BodySnippet: `{"error":"unauthorized"}`,
		ResponseHeaders: map[string]string{
			"content-type":      "application/json",
			"x-mimo-request-id": "req-123",
			"authorization":     "should-not-appear",
		},
	}

	diagnostic := safeProviderDiagnostic(err)
	if diagnostic == "" {
		t.Fatal("expected diagnostic")
	}
	if !contains(diagnostic, "provider_status=401") {
		t.Fatalf("expected status in diagnostic, got %q", diagnostic)
	}
	if !contains(diagnostic, "provider_body={\"error\":\"unauthorized\"}") {
		t.Fatalf("expected body snippet in diagnostic, got %q", diagnostic)
	}
	if !contains(diagnostic, "x-mimo-request-id=req-123") {
		t.Fatalf("expected safe header in diagnostic, got %q", diagnostic)
	}
	if contains(diagnostic, "authorization=") {
		t.Fatalf("expected unsafe header to be excluded, got %q", diagnostic)
	}
}

func TestSafeProviderDiagnosticReturnsEmptyForNonStatusError(t *testing.T) {
	if got := safeProviderDiagnostic(errors.New("network failed")); got != "" {
		t.Fatalf("expected empty diagnostic for non-status error, got %q", got)
	}
}

func TestSanitizeResponseHeadersKeepsOnlyWhitelistedHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("X-Mimo-Request-Id", "req-abc")
	headers.Set("Authorization", "Bearer secret")

	safe := sanitizeResponseHeaders(headers)
	if safe["content-type"] != "application/json" {
		t.Fatalf("expected content-type to be kept, got %q", safe["content-type"])
	}
	if safe["x-mimo-request-id"] != "req-abc" {
		t.Fatalf("expected x-mimo-request-id to be kept, got %q", safe["x-mimo-request-id"])
	}
	if _, ok := safe["authorization"]; ok {
		t.Fatal("expected authorization header to be excluded")
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) > 0 && (len(haystack) >= len(needle)) && (indexOf(haystack, needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
