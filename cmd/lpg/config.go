package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAuditPath            = "./audit.log"
	defaultProviderTimeout      = 2 * time.Second
	defaultMimoModel            = "mimo-v2-flash"
	defaultUpstreamAPIKeyHeader = "Authorization"
	defaultUpstreamAPIKeyPrefix = "Bearer"
	defaultUpstreamChatPath     = "/v1/chat/completions"

	defaultLocalAbstractionAPIKeyHeader = "Authorization"
	defaultLocalAbstractionAPIKeyPrefix = "Bearer"
	defaultLocalAbstractionChatPath     = "/v1/chat/completions"
)

type providerMode string

const (
	providerStub             providerMode = "stub"
	providerVLLMLocal        providerMode = "vllm_local"
	providerMimoOnline       providerMode = "mimo_online"
	providerOpenAICompatible providerMode = "openai_compatible"
)

type startupConfig struct {
	AuditPath       string
	Provider        providerMode
	ProviderTimeout time.Duration

	AllowRawForwarding bool
	CriticalLocalOnly  bool

	VLLMBaseURL string
	VLLMModel   string

	MimoBaseURL string
	MimoAPIKey  string
	MimoModel   string

	UpstreamBaseURL      string
	UpstreamAPIKey       string
	UpstreamModel        string
	UpstreamAPIKeyHeader string
	UpstreamAPIKeyPrefix string
	UpstreamChatPath     string

	LocalAbstractionBaseURL      string
	LocalAbstractionAPIKey       string
	LocalAbstractionModel        string
	LocalAbstractionAPIKeyHeader string
	LocalAbstractionAPIKeyPrefix string
	LocalAbstractionChatPath     string
}

func loadStartupConfigFromEnv() (startupConfig, error) {
	cfg := startupConfig{
		AuditPath:                    defaultAuditPath,
		Provider:                     providerStub,
		ProviderTimeout:              defaultProviderTimeout,
		MimoModel:                    defaultMimoModel,
		UpstreamAPIKeyHeader:         defaultUpstreamAPIKeyHeader,
		UpstreamAPIKeyPrefix:         defaultUpstreamAPIKeyPrefix,
		UpstreamChatPath:             defaultUpstreamChatPath,
		LocalAbstractionAPIKeyHeader: defaultLocalAbstractionAPIKeyHeader,
		LocalAbstractionAPIKeyPrefix: defaultLocalAbstractionAPIKeyPrefix,
		LocalAbstractionChatPath:     defaultLocalAbstractionChatPath,
	}

	if value := strings.TrimSpace(os.Getenv("LPG_AUDIT_PATH")); value != "" {
		cfg.AuditPath = value
	}

	if value := strings.TrimSpace(os.Getenv("LPG_PROVIDER")); value != "" {
		provider, err := parseProviderMode(value)
		if err != nil {
			return startupConfig{}, err
		}
		cfg.Provider = provider
	}

	if value := strings.TrimSpace(os.Getenv("LPG_PROVIDER_TIMEOUT")); value != "" {
		timeout, err := time.ParseDuration(value)
		if err != nil {
			return startupConfig{}, fmt.Errorf("invalid LPG_PROVIDER_TIMEOUT: %w", err)
		}
		if timeout <= 0 {
			return startupConfig{}, fmt.Errorf("invalid LPG_PROVIDER_TIMEOUT: must be > 0")
		}
		cfg.ProviderTimeout = timeout
	}

	if value, ok := envValue("LPG_ALLOW_RAW_FORWARDING"); ok {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return startupConfig{}, fmt.Errorf("invalid LPG_ALLOW_RAW_FORWARDING: %w", err)
		}
		cfg.AllowRawForwarding = parsed
	}

	if value, ok := envValue("LPG_CRITICAL_LOCAL_ONLY"); ok {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return startupConfig{}, fmt.Errorf("invalid LPG_CRITICAL_LOCAL_ONLY: %w", err)
		}
		cfg.CriticalLocalOnly = parsed
	}

	cfg.VLLMBaseURL = strings.TrimSpace(os.Getenv("LPG_VLLM_BASE_URL"))
	cfg.VLLMModel = strings.TrimSpace(os.Getenv("LPG_VLLM_MODEL"))

	cfg.MimoBaseURL = strings.TrimSpace(os.Getenv("LPG_MIMO_BASE_URL"))
	cfg.MimoAPIKey = strings.TrimSpace(os.Getenv("LPG_MIMO_API_KEY"))
	if value := strings.TrimSpace(os.Getenv("LPG_MIMO_MODEL")); value != "" {
		cfg.MimoModel = value
	}

	cfg.UpstreamBaseURL = strings.TrimSpace(os.Getenv("LPG_UPSTREAM_BASE_URL"))
	cfg.UpstreamAPIKey = strings.TrimSpace(os.Getenv("LPG_UPSTREAM_API_KEY"))
	cfg.UpstreamModel = strings.TrimSpace(os.Getenv("LPG_UPSTREAM_MODEL"))
	if value, ok := envValue("LPG_UPSTREAM_API_KEY_HEADER"); ok {
		cfg.UpstreamAPIKeyHeader = value
	}
	if value, ok := envValue("LPG_UPSTREAM_API_KEY_PREFIX"); ok {
		cfg.UpstreamAPIKeyPrefix = value
	}
	if value, ok := envValue("LPG_UPSTREAM_CHAT_PATH"); ok {
		cfg.UpstreamChatPath = value
	}

	cfg.LocalAbstractionBaseURL = strings.TrimSpace(os.Getenv("LPG_LOCAL_ABSTRACTION_BASE_URL"))
	cfg.LocalAbstractionAPIKey = strings.TrimSpace(os.Getenv("LPG_LOCAL_ABSTRACTION_API_KEY"))
	cfg.LocalAbstractionModel = strings.TrimSpace(os.Getenv("LPG_LOCAL_ABSTRACTION_MODEL"))
	if value, ok := envValue("LPG_LOCAL_ABSTRACTION_API_KEY_HEADER"); ok {
		cfg.LocalAbstractionAPIKeyHeader = value
	}
	if value, ok := envValue("LPG_LOCAL_ABSTRACTION_API_KEY_PREFIX"); ok {
		cfg.LocalAbstractionAPIKeyPrefix = value
	}
	if value, ok := envValue("LPG_LOCAL_ABSTRACTION_CHAT_PATH"); ok {
		cfg.LocalAbstractionChatPath = value
	}

	switch cfg.Provider {
	case providerStub:
		// no additional required variables
	case providerVLLMLocal:
		if cfg.VLLMBaseURL == "" {
			return startupConfig{}, fmt.Errorf("LPG_VLLM_BASE_URL is required when LPG_PROVIDER=%q", providerVLLMLocal)
		}
	case providerMimoOnline:
		if cfg.MimoBaseURL == "" {
			return startupConfig{}, fmt.Errorf("LPG_MIMO_BASE_URL is required when LPG_PROVIDER=%q", providerMimoOnline)
		}
		if cfg.MimoAPIKey == "" {
			return startupConfig{}, fmt.Errorf("LPG_MIMO_API_KEY is required when LPG_PROVIDER=%q", providerMimoOnline)
		}
	case providerOpenAICompatible:
		if cfg.UpstreamBaseURL == "" {
			return startupConfig{}, fmt.Errorf("LPG_UPSTREAM_BASE_URL is required when LPG_PROVIDER=%q", providerOpenAICompatible)
		}
	}

	if cfg.LocalAbstractionBaseURL != "" && cfg.LocalAbstractionModel == "" {
		return startupConfig{}, fmt.Errorf("LPG_LOCAL_ABSTRACTION_MODEL is required when LPG_LOCAL_ABSTRACTION_BASE_URL is set")
	}
	if cfg.LocalAbstractionBaseURL == "" && cfg.LocalAbstractionModel != "" {
		return startupConfig{}, fmt.Errorf("LPG_LOCAL_ABSTRACTION_BASE_URL is required when LPG_LOCAL_ABSTRACTION_MODEL is set")
	}

	return cfg, nil
}

func parseProviderMode(raw string) (providerMode, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch providerMode(normalized) {
	case providerStub, providerVLLMLocal, providerMimoOnline, providerOpenAICompatible:
		return providerMode(normalized), nil
	case "generic", "openai", "custom", "llamacpp_local":
		return providerOpenAICompatible, nil
	default:
		return "", fmt.Errorf("invalid LPG_PROVIDER %q: must be one of %q, %q, %q, %q", raw, providerStub, providerVLLMLocal, providerMimoOnline, providerOpenAICompatible)
	}
}

func envValue(key string) (string, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(value), true
}
