package proxy

import (
	"context"
	"fmt"
	"strings"
)

type OpenAICompatibleUpstream struct {
	client       *providerHTTPClient
	defaultModel string
}

type OpenAICompatibleConfig struct {
	BaseURL      string
	APIKey       string
	Model        string
	APIKeyHeader string
	APIKeyPrefix string
	ChatPath     string
}

func NewOpenAICompatibleUpstream(cfg OpenAICompatibleConfig) (*OpenAICompatibleUpstream, error) {
	client, err := newProviderHTTPClientWithConfig(providerHTTPConfig{
		BaseURL:      cfg.BaseURL,
		APIKey:       cfg.APIKey,
		APIKeyHeader: cfg.APIKeyHeader,
		APIKeyPrefix: cfg.APIKeyPrefix,
		ChatPath:     cfg.ChatPath,
	})
	if err != nil {
		return nil, err
	}

	return &OpenAICompatibleUpstream{
		client:       client,
		defaultModel: strings.TrimSpace(cfg.Model),
	}, nil
}

func (u *OpenAICompatibleUpstream) ChatCompletions(ctx context.Context, req ForwardRequest) (ForwardResponse, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = strings.TrimSpace(u.defaultModel)
	}
	if model == "" {
		return ForwardResponse{}, fmt.Errorf("model is required")
	}
	return u.client.chatCompletions(ctx, model, req.SanitizedPrompt, req.IdempotencyKey)
}
