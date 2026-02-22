package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/soloengine/lpg/internal/audit"
	"github.com/soloengine/lpg/internal/proxy"
	"github.com/soloengine/lpg/internal/risk"
	"github.com/soloengine/lpg/internal/router"
	"github.com/soloengine/lpg/internal/sanitizer"
)

func main() {
	cfg, err := loadStartupConfigFromEnv()
	if err != nil {
		log.Fatalf("invalid startup configuration: %v", err)
	}

	chainWriter, err := audit.NewChainWriter(cfg.AuditPath)
	if err != nil {
		log.Fatalf("failed to initialize audit writer: %v", err)
	}

	upstream, err := upstreamFromConfig(cfg)
	if err != nil {
		log.Fatalf("failed to initialize upstream provider: %v", err)
	}

	abstractor, err := abstractorFromConfig(cfg)
	if err != nil {
		log.Fatalf("failed to initialize local abstraction provider: %v", err)
	}

	handler := proxy.NewHandler(proxy.HandlerConfig{
		Sanitizer:       sanitizer.NewDefault(),
		Scorer:          risk.NewScorer(0.70),
		Router:          router.NewEngineWithCriticalLocalOnly(cfg.AllowRawForwarding, cfg.CriticalLocalOnly),
		Upstream:        upstream,
		Abstractor:      abstractor,
		Audit:           chainWriter,
		PolicyVersion:   "v2.1-phase1",
		ProviderTimeout: cfg.ProviderTimeout,
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", handler.HandleChatCompletions)
	mux.HandleFunc("/v1/debug/explain", handler.HandleDebugExplain)

	addr := "127.0.0.1:8080"
	log.Printf("lpg proxy listening on %s (provider=%s raw_forward=%t critical_local_only=%t local_abstractor=%t)", addr, cfg.Provider, cfg.AllowRawForwarding, cfg.CriticalLocalOnly, cfg.LocalAbstractionBaseURL != "")
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func upstreamFromConfig(cfg startupConfig) (proxy.UpstreamAdapter, error) {
	switch cfg.Provider {
	case providerStub:
		return proxy.StubUpstream{}, nil
	case providerVLLMLocal:
		return proxy.NewVLLMUpstream(cfg.VLLMBaseURL, cfg.VLLMModel)
	case providerMimoOnline:
		return proxy.NewMimoUpstream(cfg.MimoBaseURL, cfg.MimoAPIKey, cfg.MimoModel)
	case providerOpenAICompatible:
		return proxy.NewOpenAICompatibleUpstream(proxy.OpenAICompatibleConfig{
			BaseURL:      cfg.UpstreamBaseURL,
			APIKey:       cfg.UpstreamAPIKey,
			Model:        cfg.UpstreamModel,
			APIKeyHeader: cfg.UpstreamAPIKeyHeader,
			APIKeyPrefix: cfg.UpstreamAPIKeyPrefix,
			ChatPath:     cfg.UpstreamChatPath,
		})
	default:
		return nil, fmt.Errorf("unsupported provider mode %q", cfg.Provider)
	}
}

func abstractorFromConfig(cfg startupConfig) (proxy.Abstractor, error) {
	if cfg.LocalAbstractionBaseURL == "" {
		return proxy.PassthroughAbstractor{}, nil
	}

	return proxy.NewOpenAICompatibleAbstractor(proxy.OpenAICompatibleConfig{
		BaseURL:      cfg.LocalAbstractionBaseURL,
		APIKey:       cfg.LocalAbstractionAPIKey,
		Model:        cfg.LocalAbstractionModel,
		APIKeyHeader: cfg.LocalAbstractionAPIKeyHeader,
		APIKeyPrefix: cfg.LocalAbstractionAPIKeyPrefix,
		ChatPath:     cfg.LocalAbstractionChatPath,
	})
}
