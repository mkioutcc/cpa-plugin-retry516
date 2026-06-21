package main

import (
	"encoding/json"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func TestRouteModelHandlesMatchingRequest(t *testing.T) {
	currentConfig.Store(defaultPluginConfig())
	raw, errRoute := routeModel([]byte(`{"SourceFormat":"codex","RequestedModel":"gpt-5.5","Stream":false}`))
	if errRoute != nil {
		t.Fatalf("routeModel() error = %v", errRoute)
	}
	resp := decodeRouteResponse(t, raw)
	if !resp.Handled || resp.TargetKind != pluginapi.ModelRouteTargetSelf {
		t.Fatalf("route response = %+v, want handled self", resp)
	}
}

func TestRouteModelDeclinesNonMatchingModel(t *testing.T) {
	currentConfig.Store(defaultPluginConfig())
	raw, errRoute := routeModel([]byte(`{"SourceFormat":"codex","RequestedModel":"claude-sonnet-4-6","Stream":false}`))
	if errRoute != nil {
		t.Fatalf("routeModel() error = %v", errRoute)
	}
	resp := decodeRouteResponse(t, raw)
	if resp.Handled {
		t.Fatalf("route response = %+v, want declined", resp)
	}
}

func TestRouteModelDeclinesStreamingWhenDisabled(t *testing.T) {
	cfg := defaultPluginConfig()
	cfg.RetryStream = false
	currentConfig.Store(cfg)
	raw, errRoute := routeModel([]byte(`{"SourceFormat":"codex","RequestedModel":"gpt-5.5","Stream":true}`))
	if errRoute != nil {
		t.Fatalf("routeModel() error = %v", errRoute)
	}
	resp := decodeRouteResponse(t, raw)
	if resp.Handled {
		t.Fatalf("route response = %+v, want declined", resp)
	}
}

func decodeRouteResponse(t *testing.T, raw []byte) pluginapi.ModelRouteResponse {
	t.Helper()
	var env envelope
	if errDecode := json.Unmarshal(raw, &env); errDecode != nil {
		t.Fatalf("decode envelope: %v", errDecode)
	}
	if !env.OK {
		t.Fatalf("envelope error: %+v", env.Error)
	}
	var resp pluginapi.ModelRouteResponse
	if errDecode := json.Unmarshal(env.Result, &resp); errDecode != nil {
		t.Fatalf("decode route response: %v", errDecode)
	}
	return resp
}
