package main

import "github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"

const (
	pluginID   = "retry516"
	pluginName = "Retry 516"
	pluginRepo = "https://github.com/mkioutcc/cpa-plugin-retry516"
)

var pluginVersion = "0.1.1"

func pluginRegistration() registration {
	return registration{
		SchemaVersion: schemaVersion,
		Metadata: pluginapi.Metadata{
			Name:             pluginName,
			Version:          pluginVersion,
			Author:           "mkioutcc",
			GitHubRepository: pluginRepo,
			ConfigFields: []pluginapi.ConfigField{
				{Name: "enabled", Type: pluginapi.ConfigFieldTypeBoolean, Description: "When false, the plugin declines all matching requests."},
				{Name: "source_formats", Type: pluginapi.ConfigFieldTypeArray, Description: "Inbound protocol formats to wrap, such as codex, openai, or openai-response."},
				{Name: "model_patterns", Type: pluginapi.ConfigFieldTypeArray, Description: "Case-insensitive model glob patterns to wrap, such as gpt-5*."},
				{Name: "trigger_reasoning_tokens", Type: pluginapi.ConfigFieldTypeInteger, Description: "Reasoning token count that triggers a retry. Default: 516."},
				{Name: "max_retries", Type: pluginapi.ConfigFieldTypeInteger, Description: "Maximum automatic retries per request. Default: 1."},
				{Name: "retry_stream", Type: pluginapi.ConfigFieldTypeBoolean, Description: "When true, streaming requests are buffered, checked, and replayed after retry decisions."},
				{Name: "stream_mode", Type: pluginapi.ConfigFieldTypeEnum, EnumValues: []string{"buffer_then_replay"}, Description: "Streaming retry mode. Only buffer_then_replay is supported."},
			},
		},
		Capabilities: registrationCapabilities{
			ModelRouter:           true,
			Executor:              true,
			ExecutorModelScope:    string(pluginapi.ExecutorModelScopeStatic),
			ExecutorInputFormats:  executorFormats(),
			ExecutorOutputFormats: executorFormats(),
		},
	}
}

func executorFormats() []string {
	return []string{"codex", "openai", "openai-response"}
}
