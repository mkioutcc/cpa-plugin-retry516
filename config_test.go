package main

import "testing"

func TestDecodeConfigDefaultsAndOverrides(t *testing.T) {
	cfg, errDecode := decodeConfig([]byte(`
source_formats:
  - codex
model_patterns:
  - gpt-5.5*
trigger_reasoning_tokens: 516
max_retries: 2
retry_stream: false
`))
	if errDecode != nil {
		t.Fatalf("decodeConfig() error = %v", errDecode)
	}
	if !cfg.Enabled {
		t.Fatal("Enabled = false, want default true")
	}
	if cfg.MaxRetries != 2 {
		t.Fatalf("MaxRetries = %d, want 2", cfg.MaxRetries)
	}
	if cfg.RetryStream {
		t.Fatal("RetryStream = true, want false")
	}
	if !sourceFormatAllowed(cfg, "codex") || sourceFormatAllowed(cfg, "claude") {
		t.Fatalf("source format allowlist mismatch: %+v", cfg.SourceFormats)
	}
	if !modelAllowed(cfg, "gpt-5.5-codex") || modelAllowed(cfg, "claude-sonnet") {
		t.Fatalf("model pattern mismatch: %+v", cfg.ModelPatterns)
	}
}

func TestDecodeConfigCapsRetries(t *testing.T) {
	cfg, errDecode := decodeConfig([]byte("max_retries: 99\n"))
	if errDecode != nil {
		t.Fatalf("decodeConfig() error = %v", errDecode)
	}
	if cfg.MaxRetries != 3 {
		t.Fatalf("MaxRetries = %d, want cap 3", cfg.MaxRetries)
	}
}

func TestNormalizeFormatAliases(t *testing.T) {
	if got := normalizeFormatName("responses"); got != "openai-response" {
		t.Fatalf("normalize responses = %q", got)
	}
	if got := normalizeFormatName("chat-completions"); got != "openai" {
		t.Fatalf("normalize chat-completions = %q", got)
	}
}
