package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

const defaultProviderChatCompletionsPath = "/v1/chat/completions"

type providerHTTPClient struct {
	baseURL      string
	apiKey       string
	apiKeyHeader string
	apiKeyPrefix string
	chatPath     string
	client       *http.Client
}

type ProviderHTTPStatusError struct {
	StatusCode      int
	BodySnippet     string
	ResponseHeaders map[string]string
}

var redactionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)"api[-_]?key"\s*:\s*"[^"]+"`),
	regexp.MustCompile(`(?i)"access[-_]?token"\s*:\s*"[^"]+"`),
	regexp.MustCompile(`(?i)"token"\s*:\s*"[^"]+"`),
	regexp.MustCompile(`(?i)"authorization"\s*:\s*"[^"]+"`),
	regexp.MustCompile(`(?i)(?:sk|rk)-[a-z0-9]{8,}`),
	regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`),
	regexp.MustCompile(`\b\d{3}[-.\s]?\d{3}[-.\s]?\d{4}\b`),
	regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
}

func (e *ProviderHTTPStatusError) Error() string {
	if e == nil {
		return "provider returned non-2xx status"
	}
	if strings.TrimSpace(e.BodySnippet) == "" {
		return fmt.Sprintf("provider returned status %d", e.StatusCode)
	}
	return fmt.Sprintf("provider returned status %d: %s", e.StatusCode, e.BodySnippet)
}

type providerHTTPConfig struct {
	BaseURL      string
	APIKey       string
	APIKeyHeader string
	APIKeyPrefix string
	ChatPath     string
}

type providerChatRequest struct {
	Model    string                `json:"model"`
	Messages []providerChatMessage `json:"messages"`
}

type providerChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type providerChatResponse struct {
	Choices []providerChatChoice `json:"choices"`
}

type providerChatChoice struct {
	Message providerChatMessage `json:"message"`
}

func newProviderHTTPClient(baseURL, apiKey string) (*providerHTTPClient, error) {
	return newProviderHTTPClientWithConfig(providerHTTPConfig{
		BaseURL:      baseURL,
		APIKey:       apiKey,
		APIKeyHeader: "Authorization",
		APIKeyPrefix: "Bearer",
		ChatPath:     defaultProviderChatCompletionsPath,
	})
}

func newProviderHTTPClientWithConfig(cfg providerHTTPConfig) (*providerHTTPClient, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	chatPath := strings.TrimSpace(cfg.ChatPath)
	if chatPath == "" {
		chatPath = defaultProviderChatCompletionsPath
	}
	if !strings.HasPrefix(chatPath, "/") {
		chatPath = "/" + chatPath
	}

	apiKeyHeader := strings.TrimSpace(cfg.APIKeyHeader)
	if apiKeyHeader == "" {
		apiKeyHeader = "Authorization"
	}

	apiKeyPrefix := strings.TrimSpace(cfg.APIKeyPrefix)

	return &providerHTTPClient{
		baseURL:      base,
		apiKey:       strings.TrimSpace(cfg.APIKey),
		apiKeyHeader: apiKeyHeader,
		apiKeyPrefix: apiKeyPrefix,
		chatPath:     chatPath,
		client:       &http.Client{},
	}, nil
}

func (c *providerHTTPClient) chatCompletions(ctx context.Context, model, prompt, idempotencyKey string) (ForwardResponse, error) {
	body, err := json.Marshal(providerChatRequest{
		Model: model,
		Messages: []providerChatMessage{
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return ForwardResponse{}, fmt.Errorf("marshal provider request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+c.chatPath, bytes.NewReader(body))
	if err != nil {
		return ForwardResponse{}, fmt.Errorf("create provider request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		httpReq.Header.Set("Idempotency-Key", idempotencyKey)
	}
	if c.apiKey != "" {
		authValue := c.apiKey
		if c.apiKeyPrefix != "" {
			authValue = c.apiKeyPrefix + " " + c.apiKey
		}
		httpReq.Header.Set(c.apiKeyHeader, authValue)
	}

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return ForwardResponse{}, fmt.Errorf("send provider request: %w", err)
	}
	defer func() {
		_ = httpResp.Body.Close()
	}()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return ForwardResponse{}, fmt.Errorf("read provider response: %w", err)
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return ForwardResponse{}, &ProviderHTTPStatusError{
			StatusCode:      httpResp.StatusCode,
			BodySnippet:     safeBodySnippet(respBody),
			ResponseHeaders: sanitizeResponseHeaders(httpResp.Header),
		}
	}

	var parsed providerChatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return ForwardResponse{}, fmt.Errorf("parse provider response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return ForwardResponse{}, fmt.Errorf("provider response missing choices")
	}
	if strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return ForwardResponse{}, fmt.Errorf("provider response missing choice content")
	}

	return ForwardResponse{Content: parsed.Choices[0].Message.Content}, nil
}

func safeBodySnippet(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}
	for _, pattern := range redactionPatterns {
		trimmed = pattern.ReplaceAllString(trimmed, "[REDACTED]")
	}
	if len(trimmed) > 240 {
		return trimmed[:240] + "..."
	}
	return trimmed
}

func sanitizeResponseHeaders(headers http.Header) map[string]string {
	safe := make(map[string]string)
	for _, key := range []string{"content-type", "x-request-id", "request-id", "trace-id", "x-trace-id", "x-mimo-request-id"} {
		if values, ok := headers[http.CanonicalHeaderKey(key)]; ok && len(values) > 0 {
			safe[key] = values[0]
		}
	}
	if values, ok := headers["Retry-After"]; ok && len(values) > 0 {
		safe["retry-after"] = values[0]
	}
	if statusText, ok := headers[":status"]; ok && len(statusText) > 0 {
		safe["status"] = statusText[0]
	}
	return safe
}

func safeProviderDiagnostic(err error) string {
	var statusErr *ProviderHTTPStatusError
	if !errors.As(err, &statusErr) {
		return ""
	}

	parts := []string{"provider_status=" + strconv.Itoa(statusErr.StatusCode)}
	if statusErr.BodySnippet != "" {
		parts = append(parts, "provider_body="+statusErr.BodySnippet)
	}
	if len(statusErr.ResponseHeaders) > 0 {
		headerOrder := []string{"content-type", "x-request-id", "request-id", "trace-id", "x-trace-id", "x-mimo-request-id", "retry-after", "status"}
		headerParts := make([]string, 0, len(statusErr.ResponseHeaders))
		for _, key := range headerOrder {
			if value, ok := statusErr.ResponseHeaders[key]; ok {
				headerParts = append(headerParts, key+"="+value)
			}
		}
		parts = append(parts, "provider_headers="+strings.Join(headerParts, ","))
	}
	return strings.Join(parts, " ")
}
