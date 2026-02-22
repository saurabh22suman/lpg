package proxy

import (
	"context"
	"fmt"
	"strings"

	"github.com/soloengine/lpg/internal/router"
)

type PassthroughAbstractor struct{}

func (PassthroughAbstractor) Abstract(ctx context.Context, req AbstractRequest) (string, error) {
	return req.SanitizedPrompt, nil
}

type OpenAICompatibleAbstractor struct {
	client *providerHTTPClient
	model  string
}

func NewOpenAICompatibleAbstractor(cfg OpenAICompatibleConfig) (*OpenAICompatibleAbstractor, error) {
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		return nil, fmt.Errorf("model is required")
	}

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

	return &OpenAICompatibleAbstractor{
		client: client,
		model:  model,
	}, nil
}

func (a *OpenAICompatibleAbstractor) Abstract(ctx context.Context, req AbstractRequest) (string, error) {
	prompt := req.SanitizedPrompt
	if req.Route == router.RouteHighAbstraction {
		prompt = "Rewrite the sanitized text by jumbling word order while preserving intent. Keep surrogate entities unchanged.\n\n" + req.SanitizedPrompt
	}

	resp, err := a.client.chatCompletions(ctx, a.model, prompt, "")
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}
