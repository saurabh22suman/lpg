package proxy

import (
	"context"
	"fmt"
	"strings"
)

type VLLMUpstream struct {
	client       *providerHTTPClient
	defaultModel string
}

func NewVLLMUpstream(baseURL, defaultModel string) (*VLLMUpstream, error) {
	client, err := newProviderHTTPClient(baseURL, "")
	if err != nil {
		return nil, err
	}
	return &VLLMUpstream{
		client:       client,
		defaultModel: defaultModel,
	}, nil
}

func (u *VLLMUpstream) ChatCompletions(ctx context.Context, req ForwardRequest) (ForwardResponse, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = strings.TrimSpace(u.defaultModel)
	}
	if model == "" {
		return ForwardResponse{}, fmt.Errorf("model is required")
	}
	return u.client.chatCompletions(ctx, model, req.SanitizedPrompt, req.IdempotencyKey)
}
