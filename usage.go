package main

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

func reasoningTriggerFromPayload(body []byte, cfg pluginConfig) (bool, int64, bool) {
	if cfg.TriggerReasoningTokens <= 0 {
		return false, 0, false
	}
	tokens := reasoningTokensFromPayload(body)
	for _, token := range tokens {
		if token == cfg.TriggerReasoningTokens {
			return true, token, true
		}
	}
	if len(tokens) == 0 {
		return false, 0, false
	}
	return false, tokens[len(tokens)-1], true
}

func reasoningTriggerFromChunks(chunks [][]byte, cfg pluginConfig) (bool, int64, bool) {
	if cfg.TriggerReasoningTokens <= 0 {
		return false, 0, false
	}
	var last int64
	found := false
	for _, chunk := range chunks {
		for _, token := range reasoningTokensFromPayload(chunk) {
			found = true
			last = token
			if token == cfg.TriggerReasoningTokens {
				return true, token, true
			}
		}
	}
	return false, last, found
}

func reasoningTokensFromPayload(payload []byte) []int64 {
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 {
		return nil
	}
	if bytes.Contains(payload, []byte("data:")) || bytes.Contains(payload, []byte("event:")) {
		return reasoningTokensFromSSE(payload)
	}
	return reasoningTokensFromJSON(payload)
}

func reasoningTokensFromSSE(payload []byte) []int64 {
	var out []int64
	lines := bytes.Split(payload, []byte{'\n'})
	for _, rawLine := range lines {
		line := bytes.TrimSpace(rawLine)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(data) == 0 || bytes.Equal(data, []byte("[DONE]")) {
			continue
		}
		out = append(out, reasoningTokensFromJSON(data)...)
	}
	if len(out) > 0 {
		return out
	}
	return reasoningTokensFromJSON(payload)
}

func reasoningTokensFromJSON(payload []byte) []int64 {
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 || payload[0] != '{' && payload[0] != '[' {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var value any
	if errDecode := decoder.Decode(&value); errDecode != nil {
		return nil
	}
	var out []int64
	collectReasoningTokens(value, &out)
	return out
}

func collectReasoningTokens(value any, out *[]int64) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if strings.EqualFold(key, "reasoning_tokens") {
				if token, ok := int64Value(child); ok {
					*out = append(*out, token)
				}
			}
			collectReasoningTokens(child, out)
		}
	case []any:
		for _, child := range typed {
			collectReasoningTokens(child, out)
		}
	}
}

func int64Value(value any) (int64, bool) {
	switch typed := value.(type) {
	case json.Number:
		v, errInt := typed.Int64()
		if errInt == nil {
			return v, true
		}
		f, errFloat := strconv.ParseFloat(typed.String(), 64)
		if errFloat != nil {
			return 0, false
		}
		return int64(f), true
	case float64:
		return int64(typed), true
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case string:
		v, errParse := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if errParse != nil {
			return 0, false
		}
		return v, true
	default:
		return 0, false
	}
}
