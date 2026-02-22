package proxy

import (
	"context"
	"fmt"
	"strings"
)

const mimoChatCompletionsPath = "/chat/completions"

type MimoUpstream struct {
	client       *providerHTTPClient
	defaultModel string
}

func NewMimoUpstream(baseURL, apiKey, defaultModel string) (*MimoUpstream, error) {
	client, err := newProviderHTTPClientWithConfig(providerHTTPConfig{
		BaseURL:      baseURL,
		APIKey:       apiKey,
		APIKeyHeader: "api-key",
		APIKeyPrefix: "",
		ChatPath:     mimoChatCompletionsPath,
	})
	if err != nil {
		return nil, err
	}
	return &MimoUpstream{
		client:       client,
		defaultModel: defaultModel,
	}, nil
}

func (u *MimoUpstream) ChatCompletions(ctx context.Context, req ForwardRequest) (ForwardResponse, error) {
	model := strings.TrimSpace(u.defaultModel)
	if model == "" {
		model = strings.TrimSpace(req.Model)
	}
	if model == "" {
		return ForwardResponse{}, fmt.Errorf("model is required")
	}
	return u.client.chatCompletions(ctx, model, req.SanitizedPrompt, req.IdempotencyKey)
}
