package proxy

import "context"

type StubUpstream struct{}

func (StubUpstream) ChatCompletions(ctx context.Context, req ForwardRequest) (ForwardResponse, error) {
	select {
	case <-ctx.Done():
		return ForwardResponse{}, ctx.Err()
	default:
	}

	return ForwardResponse{Content: "stub completion"}, nil
}
