package main

import "testing"

func TestReasoningTriggerFromChatCompletionsUsage(t *testing.T) {
	cfg := defaultPluginConfig()
	matched, token, found := reasoningTriggerFromPayload([]byte(`{
		"usage": {
			"completion_tokens_details": {"reasoning_tokens": 516}
		}
	}`), cfg)
	if !matched || !found || token != 516 {
		t.Fatalf("trigger = (%v, %d, %v), want match 516", matched, token, found)
	}
}

func TestReasoningTriggerFromResponsesUsage(t *testing.T) {
	cfg := defaultPluginConfig()
	matched, token, found := reasoningTriggerFromPayload([]byte(`{
		"response": {
			"usage": {
				"output_tokens_details": {"reasoning_tokens": 516}
			}
		}
	}`), cfg)
	if !matched || !found || token != 516 {
		t.Fatalf("trigger = (%v, %d, %v), want match 516", matched, token, found)
	}
}

func TestReasoningTriggerFromSSE(t *testing.T) {
	cfg := defaultPluginConfig()
	chunk := []byte("event: response.completed\n" +
		"data: {\"response\":{\"usage\":{\"output_tokens_details\":{\"reasoning_tokens\":516}}}}\n\n")
	matched, token, found := reasoningTriggerFromChunks([][]byte{chunk}, cfg)
	if !matched || !found || token != 516 {
		t.Fatalf("trigger = (%v, %d, %v), want match 516", matched, token, found)
	}
}

func TestReasoningTriggerIgnoresOtherTokenCounts(t *testing.T) {
	cfg := defaultPluginConfig()
	matched, token, found := reasoningTriggerFromPayload([]byte(`{
		"usage": {
			"completion_tokens_details": {"reasoning_tokens": 900}
		}
	}`), cfg)
	if matched || !found || token != 900 {
		t.Fatalf("trigger = (%v, %d, %v), want no match with token 900", matched, token, found)
	}
}

func TestReasoningTriggerReportsMissingUsage(t *testing.T) {
	cfg := defaultPluginConfig()
	matched, token, found := reasoningTriggerFromPayload([]byte(`{"choices":[]}`), cfg)
	if matched || found || token != 0 {
		t.Fatalf("trigger = (%v, %d, %v), want missing usage", matched, token, found)
	}
}
