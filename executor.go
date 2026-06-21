package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

type rpcExecutorRequest struct {
	pluginapi.ExecutorRequest
	StreamID       string `json:"stream_id,omitempty"`
	HostCallbackID string `json:"host_callback_id,omitempty"`
}

type hostModelExecutionRequest struct {
	pluginapi.HostModelExecutionRequest
	HostCallbackID string `json:"host_callback_id,omitempty"`
}

type rpcExecutorStreamResponse struct {
	Headers http.Header                     `json:"headers,omitempty"`
	Chunks  []pluginapi.ExecutorStreamChunk `json:"chunks,omitempty"`
}

type rpcStreamEmitRequest struct {
	StreamID string `json:"stream_id"`
	Payload  []byte `json:"payload,omitempty"`
	Error    string `json:"error,omitempty"`
}

type rpcStreamCloseRequest struct {
	StreamID string `json:"stream_id"`
	Error    string `json:"error,omitempty"`
}

type hostLogRequest struct {
	HostCallbackID string         `json:"host_callback_id,omitempty"`
	Level          string         `json:"level,omitempty"`
	Message        string         `json:"message,omitempty"`
	Fields         map[string]any `json:"fields,omitempty"`
}

func execute(raw []byte) ([]byte, error) {
	var req rpcExecutorRequest
	if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
		return nil, errUnmarshal
	}
	body, headers, errRun := runNonStreamWithRetry(req.ExecutorRequest, req.HostCallbackID)
	if errRun != nil {
		return errorEnvelope("executor_error", errRun.Error()), nil
	}
	return okEnvelope(pluginapi.ExecutorResponse{Payload: body, Headers: headers})
}

func executeStream(raw []byte) ([]byte, error) {
	var req rpcExecutorRequest
	if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
		return nil, errUnmarshal
	}
	streamID := strings.TrimSpace(req.StreamID)
	if streamID == "" {
		return errorEnvelope("executor_error", "stream_id is required for executor.execute_stream"), nil
	}
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				closePluginStream(streamID, fmt.Sprintf("retry516 stream panic: %v", recovered))
			}
		}()
		if errRun := runStreamWithRetry(req.ExecutorRequest, req.HostCallbackID, streamID); errRun != nil {
			closePluginStream(streamID, errRun.Error())
			return
		}
		closePluginStream(streamID, "")
	}()
	return okEnvelope(rpcExecutorStreamResponse{Headers: streamResponseHeaders()})
}

func runNonStreamWithRetry(exec pluginapi.ExecutorRequest, hostCallbackID string) ([]byte, http.Header, error) {
	cfg := loadedConfig()
	maxRetries := cfg.MaxRetries
	for attempt := 0; ; attempt++ {
		body, headers, errExecute := hostModelExecute(exec, hostCallbackID)
		if errExecute != nil {
			return nil, nil, errExecute
		}
		matched, token, found := reasoningTriggerFromPayload(body, cfg)
		if matched && attempt < maxRetries {
			logRetry(hostCallbackID, exec, attempt+1, token, false)
			continue
		}
		if matched && found {
			logFinal516(hostCallbackID, exec, attempt, token, false)
		}
		return body, headers, nil
	}
}

func runStreamWithRetry(exec pluginapi.ExecutorRequest, hostCallbackID, pluginStreamID string) error {
	cfg := loadedConfig()
	maxRetries := cfg.MaxRetries
	var chunks [][]byte
	for attempt := 0; ; attempt++ {
		attemptChunks, errCollect := hostModelStreamCollect(exec, hostCallbackID)
		if errCollect != nil {
			return errCollect
		}
		matched, token, found := reasoningTriggerFromChunks(attemptChunks, cfg)
		if matched && attempt < maxRetries {
			logRetry(hostCallbackID, exec, attempt+1, token, true)
			continue
		}
		if matched && found {
			logFinal516(hostCallbackID, exec, attempt, token, true)
		}
		chunks = attemptChunks
		break
	}
	for _, chunk := range chunks {
		if len(chunk) == 0 {
			continue
		}
		if errEmit := emitPluginStreamChunk(pluginStreamID, chunk); errEmit != nil {
			return errEmit
		}
	}
	return nil
}

func hostModelExecute(exec pluginapi.ExecutorRequest, hostCallbackID string) ([]byte, http.Header, error) {
	entryProtocol, exitProtocol := executionProtocols(exec)
	raw, errCall := callHost(pluginabi.MethodHostModelExecute, hostModelExecutionRequest{
		HostModelExecutionRequest: pluginapi.HostModelExecutionRequest{
			EntryProtocol: entryProtocol,
			ExitProtocol:  exitProtocol,
			Model:         strings.TrimSpace(exec.Model),
			Stream:        false,
			Body:          executorRequestBody(exec),
			Headers:       cloneHeader(exec.Headers),
			Query:         cloneValues(exec.Query),
			Alt:           exec.Alt,
		},
		HostCallbackID: hostCallbackID,
	})
	if errCall != nil {
		return nil, nil, errCall
	}
	var resp pluginapi.HostModelExecutionResponse
	if errDecode := json.Unmarshal(raw, &resp); errDecode != nil {
		return nil, nil, errDecode
	}
	if resp.StatusCode >= 400 {
		return nil, nil, fmt.Errorf("host model status %d", resp.StatusCode)
	}
	return bytes.Clone(resp.Body), cloneHeader(resp.Headers), nil
}

func hostModelStreamCollect(exec pluginapi.ExecutorRequest, hostCallbackID string) ([][]byte, error) {
	entryProtocol, exitProtocol := executionProtocols(exec)
	raw, errCall := callHost(pluginabi.MethodHostModelExecuteStream, hostModelExecutionRequest{
		HostModelExecutionRequest: pluginapi.HostModelExecutionRequest{
			EntryProtocol: entryProtocol,
			ExitProtocol:  exitProtocol,
			Model:         strings.TrimSpace(exec.Model),
			Stream:        true,
			Body:          executorRequestBody(exec),
			Headers:       cloneHeader(exec.Headers),
			Query:         cloneValues(exec.Query),
			Alt:           exec.Alt,
		},
		HostCallbackID: hostCallbackID,
	})
	if errCall != nil {
		return nil, errCall
	}
	var resp pluginapi.HostModelStreamResponse
	if errDecode := json.Unmarshal(raw, &resp); errDecode != nil {
		return nil, errDecode
	}
	if resp.StatusCode >= 400 {
		_ = closeHostModelStream(resp.StreamID)
		return nil, fmt.Errorf("host model status %d", resp.StatusCode)
	}
	if strings.TrimSpace(resp.StreamID) == "" {
		return nil, fmt.Errorf("host model stream: empty stream_id")
	}
	defer func() { _ = closeHostModelStream(resp.StreamID) }()

	var chunks [][]byte
	for {
		chunkRaw, errRead := callHost(pluginabi.MethodHostModelStreamRead, pluginapi.HostModelStreamReadRequest{StreamID: resp.StreamID})
		if errRead != nil {
			return nil, errRead
		}
		var chunk pluginapi.HostModelStreamReadResponse
		if errDecode := json.Unmarshal(chunkRaw, &chunk); errDecode != nil {
			return nil, errDecode
		}
		if strings.TrimSpace(chunk.Error) != "" {
			return nil, fmt.Errorf("%s", chunk.Error)
		}
		if len(chunk.Payload) > 0 {
			chunks = append(chunks, bytes.Clone(chunk.Payload))
		}
		if chunk.Done {
			break
		}
	}
	return chunks, nil
}

func closeHostModelStream(streamID string) error {
	if strings.TrimSpace(streamID) == "" {
		return nil
	}
	_, errCall := callHost(pluginabi.MethodHostModelStreamClose, pluginapi.HostModelStreamCloseRequest{StreamID: streamID})
	return errCall
}

func emitPluginStreamChunk(streamID string, payload []byte) error {
	if strings.TrimSpace(streamID) == "" {
		return fmt.Errorf("plugin stream id is required")
	}
	_, errCall := callHost(pluginabi.MethodHostStreamEmit, rpcStreamEmitRequest{StreamID: streamID, Payload: bytes.Clone(payload)})
	return errCall
}

func closePluginStream(streamID, errMsg string) {
	if strings.TrimSpace(streamID) == "" {
		return
	}
	_, _ = callHost(pluginabi.MethodHostStreamClose, rpcStreamCloseRequest{StreamID: streamID, Error: strings.TrimSpace(errMsg)})
}

func executionProtocols(exec pluginapi.ExecutorRequest) (string, string) {
	entryProtocol := strings.TrimSpace(exec.SourceFormat)
	if entryProtocol == "" {
		entryProtocol = strings.TrimSpace(exec.Format)
	}
	if entryProtocol == "" {
		entryProtocol = "openai"
	}
	exitProtocol := strings.TrimSpace(exec.Format)
	if exitProtocol == "" {
		exitProtocol = entryProtocol
	}
	return entryProtocol, exitProtocol
}

func executorRequestBody(exec pluginapi.ExecutorRequest) []byte {
	if len(exec.Payload) > 0 {
		return bytes.Clone(exec.Payload)
	}
	return bytes.Clone(exec.OriginalRequest)
}

func streamResponseHeaders() http.Header {
	return http.Header{"Content-Type": []string{"text/event-stream"}}
}

func cloneHeader(src http.Header) http.Header {
	if src == nil {
		return nil
	}
	return src.Clone()
}

func cloneValues(src map[string][]string) map[string][]string {
	if src == nil {
		return nil
	}
	out := make(map[string][]string, len(src))
	for key, values := range src {
		out[key] = append([]string(nil), values...)
	}
	return out
}

func logRetry(hostCallbackID string, exec pluginapi.ExecutorRequest, retryNumber int, token int64, stream bool) {
	logHost(hostCallbackID, "warn", "retry516: reasoning token trigger matched; retrying request", map[string]any{
		"model":            exec.Model,
		"source_format":    exec.SourceFormat,
		"response_format":  exec.Format,
		"reasoning_tokens": token,
		"retry_number":     retryNumber,
		"stream":           stream,
	})
}

func logFinal516(hostCallbackID string, exec pluginapi.ExecutorRequest, retriesUsed int, token int64, stream bool) {
	logHost(hostCallbackID, "warn", "retry516: final response still matched reasoning token trigger", map[string]any{
		"model":            exec.Model,
		"source_format":    exec.SourceFormat,
		"response_format":  exec.Format,
		"reasoning_tokens": token,
		"retries_used":     retriesUsed,
		"stream":           stream,
	})
}

func logHost(hostCallbackID, level, message string, fields map[string]any) {
	_, _ = callHost(pluginabi.MethodHostLog, hostLogRequest{
		HostCallbackID: hostCallbackID,
		Level:          level,
		Message:        message,
		Fields:         fields,
	})
}
