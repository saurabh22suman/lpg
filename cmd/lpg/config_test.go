package main

import (
	"testing"
	"time"

	"github.com/soloengine/lpg/internal/proxy"
)

func TestLoadStartupConfigFromEnvDefaults(t *testing.T) {
	t.Setenv("LPG_AUDIT_PATH", "")
	t.Setenv("LPG_PROVIDER", "")
	t.Setenv("LPG_PROVIDER_TIMEOUT", "")
	t.Setenv("LPG_VLLM_BASE_URL", "")
	t.Setenv("LPG_VLLM_MODEL", "")
	t.Setenv("LPG_MIMO_BASE_URL", "")
	t.Setenv("LPG_MIMO_API_KEY", "")
	t.Setenv("LPG_MIMO_MODEL", "")
	t.Setenv("LPG_UPSTREAM_BASE_URL", "")
	t.Setenv("LPG_UPSTREAM_API_KEY", "")
	t.Setenv("LPG_UPSTREAM_MODEL", "")
	t.Setenv("LPG_LOCAL_ABSTRACTION_BASE_URL", "")
	t.Setenv("LPG_LOCAL_ABSTRACTION_API_KEY", "")
	t.Setenv("LPG_LOCAL_ABSTRACTION_MODEL", "")

	cfg, err := loadStartupConfigFromEnv()
	if err != nil {
		t.Fatalf("loadStartupConfigFromEnv returned error: %v", err)
	}

	if cfg.AuditPath != defaultAuditPath {
		t.Fatalf("expected default audit path %q, got %q", defaultAuditPath, cfg.AuditPath)
	}
	if cfg.Provider != providerStub {
		t.Fatalf("expected default provider %q, got %q", providerStub, cfg.Provider)
	}
	if cfg.ProviderTimeout != defaultProviderTimeout {
		t.Fatalf("expected default timeout %s, got %s", defaultProviderTimeout, cfg.ProviderTimeout)
	}
	if cfg.MimoModel != defaultMimoModel {
		t.Fatalf("expected default mimo model %q, got %q", defaultMimoModel, cfg.MimoModel)
	}
	if cfg.UpstreamAPIKeyHeader != defaultUpstreamAPIKeyHeader {
		t.Fatalf("expected default upstream api key header %q, got %q", defaultUpstreamAPIKeyHeader, cfg.UpstreamAPIKeyHeader)
	}
	if cfg.UpstreamAPIKeyPrefix != defaultUpstreamAPIKeyPrefix {
		t.Fatalf("expected default upstream api key prefix %q, got %q", defaultUpstreamAPIKeyPrefix, cfg.UpstreamAPIKeyPrefix)
	}
	if cfg.UpstreamChatPath != defaultUpstreamChatPath {
		t.Fatalf("expected default upstream path %q, got %q", defaultUpstreamChatPath, cfg.UpstreamChatPath)
	}
	if cfg.LocalAbstractionAPIKeyHeader != defaultLocalAbstractionAPIKeyHeader {
		t.Fatalf("expected default local abstraction api key header %q, got %q", defaultLocalAbstractionAPIKeyHeader, cfg.LocalAbstractionAPIKeyHeader)
	}
	if cfg.LocalAbstractionAPIKeyPrefix != defaultLocalAbstractionAPIKeyPrefix {
		t.Fatalf("expected default local abstraction api key prefix %q, got %q", defaultLocalAbstractionAPIKeyPrefix, cfg.LocalAbstractionAPIKeyPrefix)
	}
	if cfg.LocalAbstractionChatPath != defaultLocalAbstractionChatPath {
		t.Fatalf("expected default local abstraction path %q, got %q", defaultLocalAbstractionChatPath, cfg.LocalAbstractionChatPath)
	}
	if cfg.AllowRawForwarding {
		t.Fatal("expected raw forwarding to default false")
	}
	if cfg.CriticalLocalOnly {
		t.Fatal("expected critical local-only to default false")
	}
}

func TestLoadStartupConfigFromEnvParsesTimeoutAndProvider(t *testing.T) {
	t.Setenv("LPG_PROVIDER", "vLLM_local")
	t.Setenv("LPG_PROVIDER_TIMEOUT", "1500ms")
	t.Setenv("LPG_VLLM_BASE_URL", "http://127.0.0.1:8000")
	t.Setenv("LPG_VLLM_MODEL", "local-model")

	cfg, err := loadStartupConfigFromEnv()
	if err != nil {
		t.Fatalf("loadStartupConfigFromEnv returned error: %v", err)
	}

	if cfg.Provider != providerVLLMLocal {
		t.Fatalf("expected provider %q, got %q", providerVLLMLocal, cfg.Provider)
	}
	if cfg.ProviderTimeout != 1500*time.Millisecond {
		t.Fatalf("expected timeout %s, got %s", 1500*time.Millisecond, cfg.ProviderTimeout)
	}
	if cfg.VLLMBaseURL != "http://127.0.0.1:8000" {
		t.Fatalf("unexpected vLLM base URL %q", cfg.VLLMBaseURL)
	}
	if cfg.VLLMModel != "local-model" {
		t.Fatalf("unexpected vLLM model %q", cfg.VLLMModel)
	}
}

func TestLoadStartupConfigFromEnvParsesRoutingFlags(t *testing.T) {
	t.Setenv("LPG_ALLOW_RAW_FORWARDING", "true")
	t.Setenv("LPG_CRITICAL_LOCAL_ONLY", "true")

	cfg, err := loadStartupConfigFromEnv()
	if err != nil {
		t.Fatalf("loadStartupConfigFromEnv returned error: %v", err)
	}
	if !cfg.AllowRawForwarding {
		t.Fatal("expected raw forwarding to be enabled")
	}
	if !cfg.CriticalLocalOnly {
		t.Fatal("expected critical local-only to be enabled")
	}
}

func TestLoadStartupConfigFromEnvRejectsInvalidRoutingFlags(t *testing.T) {
	t.Setenv("LPG_ALLOW_RAW_FORWARDING", "not-bool")
	if _, err := loadStartupConfigFromEnv(); err == nil {
		t.Fatal("expected error for invalid LPG_ALLOW_RAW_FORWARDING")
	}

	t.Setenv("LPG_ALLOW_RAW_FORWARDING", "")
	t.Setenv("LPG_CRITICAL_LOCAL_ONLY", "also-bad")
	if _, err := loadStartupConfigFromEnv(); err == nil {
		t.Fatal("expected error for invalid LPG_CRITICAL_LOCAL_ONLY")
	}
}

func TestLoadStartupConfigFromEnvRejectsInvalidProvider(t *testing.T) {
	t.Setenv("LPG_PROVIDER", "not-a-provider")

	if _, err := loadStartupConfigFromEnv(); err == nil {
		t.Fatal("expected error for invalid LPG_PROVIDER")
	}
}

func TestLoadStartupConfigFromEnvRejectsInvalidTimeout(t *testing.T) {
	t.Setenv("LPG_PROVIDER_TIMEOUT", "abc")

	if _, err := loadStartupConfigFromEnv(); err == nil {
		t.Fatal("expected error for invalid LPG_PROVIDER_TIMEOUT")
	}
}

func TestLoadStartupConfigFromEnvRejectsNonPositiveTimeout(t *testing.T) {
	t.Setenv("LPG_PROVIDER_TIMEOUT", "0s")

	if _, err := loadStartupConfigFromEnv(); err == nil {
		t.Fatal("expected error for non-positive LPG_PROVIDER_TIMEOUT")
	}
}

func TestLoadStartupConfigFromEnvRequiresVLLMBaseURL(t *testing.T) {
	t.Setenv("LPG_PROVIDER", string(providerVLLMLocal))
	t.Setenv("LPG_VLLM_BASE_URL", "")

	if _, err := loadStartupConfigFromEnv(); err == nil {
		t.Fatal("expected error when vLLM base URL is missing")
	}
}

func TestLoadStartupConfigFromEnvRequiresMimoSettings(t *testing.T) {
	t.Setenv("LPG_PROVIDER", string(providerMimoOnline))
	t.Setenv("LPG_MIMO_BASE_URL", "")
	t.Setenv("LPG_MIMO_API_KEY", "")

	if _, err := loadStartupConfigFromEnv(); err == nil {
		t.Fatal("expected error when mimo settings are missing")
	}
}

func TestLoadStartupConfigFromEnvAcceptsMimoDefaultsAndOverrides(t *testing.T) {
	t.Setenv("LPG_PROVIDER", string(providerMimoOnline))
	t.Setenv("LPG_MIMO_BASE_URL", "https://example.invalid")
	t.Setenv("LPG_MIMO_API_KEY", "secret")
	t.Setenv("LPG_MIMO_MODEL", "")

	cfg, err := loadStartupConfigFromEnv()
	if err != nil {
		t.Fatalf("loadStartupConfigFromEnv returned error: %v", err)
	}
	if cfg.MimoModel != defaultMimoModel {
		t.Fatalf("expected default mimo model %q, got %q", defaultMimoModel, cfg.MimoModel)
	}

	t.Setenv("LPG_MIMO_MODEL", "mimo-custom")
	cfg, err = loadStartupConfigFromEnv()
	if err != nil {
		t.Fatalf("loadStartupConfigFromEnv returned error: %v", err)
	}
	if cfg.MimoModel != "mimo-custom" {
		t.Fatalf("expected overridden mimo model, got %q", cfg.MimoModel)
	}
}

func TestLoadStartupConfigFromEnvSupportsOpenAICompatibleProvider(t *testing.T) {
	t.Setenv("LPG_PROVIDER", string(providerOpenAICompatible))
	t.Setenv("LPG_UPSTREAM_BASE_URL", "https://example.invalid")
	t.Setenv("LPG_UPSTREAM_API_KEY", "secret")
	t.Setenv("LPG_UPSTREAM_MODEL", "generic-model")
	t.Setenv("LPG_UPSTREAM_API_KEY_HEADER", "X-API-Key")
	t.Setenv("LPG_UPSTREAM_API_KEY_PREFIX", "Token")
	t.Setenv("LPG_UPSTREAM_CHAT_PATH", "/custom/chat")

	cfg, err := loadStartupConfigFromEnv()
	if err != nil {
		t.Fatalf("loadStartupConfigFromEnv returned error: %v", err)
	}

	if cfg.Provider != providerOpenAICompatible {
		t.Fatalf("expected provider %q, got %q", providerOpenAICompatible, cfg.Provider)
	}
	if cfg.UpstreamBaseURL != "https://example.invalid" {
		t.Fatalf("unexpected upstream base url %q", cfg.UpstreamBaseURL)
	}
	if cfg.UpstreamAPIKey != "secret" {
		t.Fatalf("unexpected upstream api key %q", cfg.UpstreamAPIKey)
	}
	if cfg.UpstreamModel != "generic-model" {
		t.Fatalf("unexpected upstream model %q", cfg.UpstreamModel)
	}
	if cfg.UpstreamAPIKeyHeader != "X-API-Key" {
		t.Fatalf("unexpected upstream api key header %q", cfg.UpstreamAPIKeyHeader)
	}
	if cfg.UpstreamAPIKeyPrefix != "Token" {
		t.Fatalf("unexpected upstream api key prefix %q", cfg.UpstreamAPIKeyPrefix)
	}
	if cfg.UpstreamChatPath != "/custom/chat" {
		t.Fatalf("unexpected upstream chat path %q", cfg.UpstreamChatPath)
	}
}

func TestLoadStartupConfigFromEnvRequiresOpenAICompatibleBaseURL(t *testing.T) {
	t.Setenv("LPG_PROVIDER", string(providerOpenAICompatible))
	t.Setenv("LPG_UPSTREAM_BASE_URL", "")

	if _, err := loadStartupConfigFromEnv(); err == nil {
		t.Fatal("expected error when openai compatible base URL is missing")
	}
}

func TestLoadStartupConfigFromEnvSupportsProviderAliases(t *testing.T) {
	aliases := []string{"generic", "openai", "custom", "llamacpp_local"}
	for _, alias := range aliases {
		t.Run(alias, func(t *testing.T) {
			t.Setenv("LPG_PROVIDER", alias)
			t.Setenv("LPG_UPSTREAM_BASE_URL", "https://example.invalid")

			cfg, err := loadStartupConfigFromEnv()
			if err != nil {
				t.Fatalf("loadStartupConfigFromEnv returned error: %v", err)
			}
			if cfg.Provider != providerOpenAICompatible {
				t.Fatalf("expected alias %q to map to %q, got %q", alias, providerOpenAICompatible, cfg.Provider)
			}
		})
	}
}

func TestLoadStartupConfigFromEnvSupportsOptionalLocalAbstractionSettings(t *testing.T) {
	t.Setenv("LPG_LOCAL_ABSTRACTION_BASE_URL", "http://127.0.0.1:8081")
	t.Setenv("LPG_LOCAL_ABSTRACTION_API_KEY", "local-secret")
	t.Setenv("LPG_LOCAL_ABSTRACTION_MODEL", "qwen2.5:3b")
	t.Setenv("LPG_LOCAL_ABSTRACTION_API_KEY_HEADER", "X-Local-Key")
	t.Setenv("LPG_LOCAL_ABSTRACTION_API_KEY_PREFIX", "Token")
	t.Setenv("LPG_LOCAL_ABSTRACTION_CHAT_PATH", "/custom/local/chat")

	cfg, err := loadStartupConfigFromEnv()
	if err != nil {
		t.Fatalf("loadStartupConfigFromEnv returned error: %v", err)
	}
	if cfg.LocalAbstractionBaseURL != "http://127.0.0.1:8081" {
		t.Fatalf("unexpected local abstraction base url %q", cfg.LocalAbstractionBaseURL)
	}
	if cfg.LocalAbstractionAPIKey != "local-secret" {
		t.Fatalf("unexpected local abstraction api key %q", cfg.LocalAbstractionAPIKey)
	}
	if cfg.LocalAbstractionModel != "qwen2.5:3b" {
		t.Fatalf("unexpected local abstraction model %q", cfg.LocalAbstractionModel)
	}
	if cfg.LocalAbstractionAPIKeyHeader != "X-Local-Key" {
		t.Fatalf("unexpected local abstraction api key header %q", cfg.LocalAbstractionAPIKeyHeader)
	}
	if cfg.LocalAbstractionAPIKeyPrefix != "Token" {
		t.Fatalf("unexpected local abstraction api key prefix %q", cfg.LocalAbstractionAPIKeyPrefix)
	}
	if cfg.LocalAbstractionChatPath != "/custom/local/chat" {
		t.Fatalf("unexpected local abstraction chat path %q", cfg.LocalAbstractionChatPath)
	}
}

func TestLoadStartupConfigFromEnvRequiresLocalAbstractionModelWhenBaseURLSet(t *testing.T) {
	t.Setenv("LPG_LOCAL_ABSTRACTION_BASE_URL", "http://127.0.0.1:8081")
	t.Setenv("LPG_LOCAL_ABSTRACTION_MODEL", "")

	if _, err := loadStartupConfigFromEnv(); err == nil {
		t.Fatal("expected error when local abstraction base URL is set without model")
	}
}

func TestLoadStartupConfigFromEnvRequiresLocalAbstractionBaseURLWhenModelSet(t *testing.T) {
	t.Setenv("LPG_LOCAL_ABSTRACTION_BASE_URL", "")
	t.Setenv("LPG_LOCAL_ABSTRACTION_MODEL", "qwen2.5:3b")

	if _, err := loadStartupConfigFromEnv(); err == nil {
		t.Fatal("expected error when local abstraction model is set without base URL")
	}
}

func TestUpstreamFromConfigReturnsExpectedAdapter(t *testing.T) {
	vllmCfg := startupConfig{
		Provider:             providerVLLMLocal,
		VLLMBaseURL:          "http://127.0.0.1:8000",
		VLLMModel:            "local-model",
		ProviderTimeout:      defaultProviderTimeout,
		UpstreamAPIKeyHeader: defaultUpstreamAPIKeyHeader,
		UpstreamAPIKeyPrefix: defaultUpstreamAPIKeyPrefix,
		UpstreamChatPath:     defaultUpstreamChatPath,
	}
	upstream, err := upstreamFromConfig(vllmCfg)
	if err != nil {
		t.Fatalf("upstreamFromConfig returned error for vLLM: %v", err)
	}
	if _, ok := upstream.(*proxy.VLLMUpstream); !ok {
		t.Fatalf("expected *proxy.VLLMUpstream, got %T", upstream)
	}

	mimoCfg := startupConfig{
		Provider:             providerMimoOnline,
		MimoBaseURL:          "https://example.invalid",
		MimoAPIKey:           "secret",
		MimoModel:            "mimo-v2-flash",
		ProviderTimeout:      defaultProviderTimeout,
		UpstreamAPIKeyHeader: defaultUpstreamAPIKeyHeader,
		UpstreamAPIKeyPrefix: defaultUpstreamAPIKeyPrefix,
		UpstreamChatPath:     defaultUpstreamChatPath,
	}
	upstream, err = upstreamFromConfig(mimoCfg)
	if err != nil {
		t.Fatalf("upstreamFromConfig returned error for mimo: %v", err)
	}
	if _, ok := upstream.(*proxy.MimoUpstream); !ok {
		t.Fatalf("expected *proxy.MimoUpstream, got %T", upstream)
	}

	openAICfg := startupConfig{
		Provider:             providerOpenAICompatible,
		UpstreamBaseURL:      "https://example.invalid",
		UpstreamAPIKey:       "secret",
		UpstreamModel:        "generic-model",
		UpstreamAPIKeyHeader: "X-API-Key",
		UpstreamAPIKeyPrefix: "Token",
		UpstreamChatPath:     "/custom/chat",
		ProviderTimeout:      defaultProviderTimeout,
	}
	upstream, err = upstreamFromConfig(openAICfg)
	if err != nil {
		t.Fatalf("upstreamFromConfig returned error for openai compatible: %v", err)
	}
	if _, ok := upstream.(*proxy.OpenAICompatibleUpstream); !ok {
		t.Fatalf("expected *proxy.OpenAICompatibleUpstream, got %T", upstream)
	}

	stubCfg := startupConfig{
		Provider:             providerStub,
		ProviderTimeout:      defaultProviderTimeout,
		MimoModel:            defaultMimoModel,
		UpstreamAPIKeyHeader: defaultUpstreamAPIKeyHeader,
		UpstreamAPIKeyPrefix: defaultUpstreamAPIKeyPrefix,
		UpstreamChatPath:     defaultUpstreamChatPath,
	}
	upstream, err = upstreamFromConfig(stubCfg)
	if err != nil {
		t.Fatalf("upstreamFromConfig returned error for stub: %v", err)
	}
	if _, ok := upstream.(proxy.StubUpstream); !ok {
		t.Fatalf("expected proxy.StubUpstream, got %T", upstream)
	}
}

func TestUpstreamFromConfigRejectsUnsupportedProvider(t *testing.T) {
	_, err := upstreamFromConfig(startupConfig{Provider: providerMode("bad")})
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}

func TestAbstractorFromConfigReturnsPassthroughByDefault(t *testing.T) {
	abstractor, err := abstractorFromConfig(startupConfig{})
	if err != nil {
		t.Fatalf("abstractorFromConfig returned error: %v", err)
	}
	if _, ok := abstractor.(proxy.PassthroughAbstractor); !ok {
		t.Fatalf("expected proxy.PassthroughAbstractor, got %T", abstractor)
	}
}

func TestAbstractorFromConfigReturnsOpenAICompatibleAbstractorWhenConfigured(t *testing.T) {
	abstractor, err := abstractorFromConfig(startupConfig{
		LocalAbstractionBaseURL:      "http://127.0.0.1:8081",
		LocalAbstractionAPIKey:       "local-secret",
		LocalAbstractionModel:        "qwen2.5:3b",
		LocalAbstractionAPIKeyHeader: "X-Local-Key",
		LocalAbstractionAPIKeyPrefix: "Token",
		LocalAbstractionChatPath:     "/local/chat",
	})
	if err != nil {
		t.Fatalf("abstractorFromConfig returned error: %v", err)
	}
	if _, ok := abstractor.(*proxy.OpenAICompatibleAbstractor); !ok {
		t.Fatalf("expected *proxy.OpenAICompatibleAbstractor, got %T", abstractor)
	}
}

func TestAbstractorFromConfigReturnsErrorForInvalidLocalAbstractionConfig(t *testing.T) {
	_, err := abstractorFromConfig(startupConfig{
		LocalAbstractionBaseURL: "http://127.0.0.1:8081",
	})
	if err == nil {
		t.Fatal("expected error when local abstraction model is missing")
	}
}
