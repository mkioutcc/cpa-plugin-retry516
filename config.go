package main

import (
	"encoding/json"
	"path"
	"strings"
	"sync/atomic"

	"gopkg.in/yaml.v3"
)

type lifecycleRequest struct {
	ConfigYAML []byte `json:"config_yaml"`
}

type pluginConfig struct {
	Enabled                 bool     `yaml:"enabled"`
	SourceFormats           []string `yaml:"source_formats"`
	ModelPatterns           []string `yaml:"model_patterns"`
	TriggerReasoningTokens  int64    `yaml:"trigger_reasoning_tokens"`
	MaxRetries              int      `yaml:"max_retries"`
	RetryStream             bool     `yaml:"retry_stream"`
	StreamMode              string   `yaml:"stream_mode"`
}

var currentConfig atomic.Value

func init() {
	currentConfig.Store(defaultPluginConfig())
}

func configure(raw []byte) error {
	var req lifecycleRequest
	if len(raw) > 0 {
		if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
			return errUnmarshal
		}
	}
	cfg := defaultPluginConfig()
	if len(req.ConfigYAML) > 0 {
		decoded, errDecode := decodeConfig(req.ConfigYAML)
		if errDecode != nil {
			return errDecode
		}
		cfg = decoded
	}
	currentConfig.Store(cfg)
	return nil
}

func defaultPluginConfig() pluginConfig {
	return pluginConfig{
		Enabled:                true,
		SourceFormats:          []string{"codex", "openai", "openai-response"},
		ModelPatterns:          []string{"gpt-5*"},
		TriggerReasoningTokens: 516,
		MaxRetries:             1,
		RetryStream:            true,
		StreamMode:             "buffer_then_replay",
	}
}

func decodeConfig(raw []byte) (pluginConfig, error) {
	cfg := defaultPluginConfig()
	if errUnmarshal := yaml.Unmarshal(raw, &cfg); errUnmarshal != nil {
		return pluginConfig{}, errUnmarshal
	}
	cfg.SourceFormats = normalizeStringList(cfg.SourceFormats)
	cfg.ModelPatterns = normalizeStringList(cfg.ModelPatterns)
	cfg.StreamMode = strings.ToLower(strings.TrimSpace(cfg.StreamMode))
	if cfg.StreamMode == "" {
		cfg.StreamMode = "buffer_then_replay"
	}
	if cfg.TriggerReasoningTokens < 0 {
		cfg.TriggerReasoningTokens = 0
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if cfg.MaxRetries > 3 {
		cfg.MaxRetries = 3
	}
	return cfg, nil
}

func loadedConfig() pluginConfig {
	raw := currentConfig.Load()
	if cfg, ok := raw.(pluginConfig); ok {
		return cfg
	}
	return defaultPluginConfig()
}

func normalizeStringList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func sourceFormatAllowed(cfg pluginConfig, source string) bool {
	if len(cfg.SourceFormats) == 0 {
		return true
	}
	source = normalizeFormatName(source)
	for _, allowed := range cfg.SourceFormats {
		if normalizeFormatName(allowed) == source {
			return true
		}
	}
	return false
}

func modelAllowed(cfg pluginConfig, model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return false
	}
	if len(cfg.ModelPatterns) == 0 {
		return true
	}
	for _, pattern := range cfg.ModelPatterns {
		if wildcardMatch(pattern, model) {
			return true
		}
	}
	return false
}

func wildcardMatch(pattern, value string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	value = strings.ToLower(strings.TrimSpace(value))
	if pattern == "" {
		return false
	}
	if pattern == "*" || pattern == value {
		return true
	}
	matched, errMatch := path.Match(pattern, value)
	if errMatch == nil {
		return matched
	}
	if strings.HasSuffix(pattern, "*") && !strings.ContainsAny(strings.TrimSuffix(pattern, "*"), "*?") {
		return strings.HasPrefix(value, strings.TrimSuffix(pattern, "*"))
	}
	return false
}

func normalizeFormatName(format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case "chat-completions", "chat_completions":
		return "openai"
	case "responses", "openai-responses", "openai_responses":
		return "openai-response"
	default:
		return format
	}
}
