# Retry 516 Plugin for CLIProxyAPI

`retry516` is a CLIProxyAPI standard dynamic-library plugin that retries a matching request once when the upstream response reports exactly `516` reasoning tokens.

It is designed for the issue discussed in [`router-for-me/CLIProxyAPI#3937`](https://github.com/router-for-me/CLIProxyAPI/discussions/3937), where GPT-5/Codex requests sent through third-party clients sometimes appear to be reasoning-limited and report `reasoning_tokens = 516`.

## How it works

The plugin registers two capabilities:

- `model_router`: routes matching requests to the plugin executor.
- `executor`: calls the normal CLIProxyAPI model execution path through `host.model.*` callbacks.

For non-streaming requests, the plugin:

1. Calls `host.model.execute` with the original protocol, model, headers, query, and body.
2. Parses the response usage fields.
3. Retries once if any `reasoning_tokens` field equals `516`.
4. Returns the retry response, or the original response when no retry is needed.

For streaming requests, the plugin uses `buffer_then_replay`:

1. Calls `host.model.execute_stream`.
2. Buffers the complete upstream stream.
3. Checks the final stream usage chunks for `reasoning_tokens = 516`.
4. Retries once when matched.
5. Replays the kept stream to the client.

This avoids sending the bad first response before the retry decision is known.

## Default scope

By default, the plugin only wraps:

- source formats: `codex`, `openai`, `openai-response`
- model patterns: `gpt-5*`
- trigger reasoning tokens: `516`
- max retries: `1`
- streaming retry: enabled

The nested host model callback forwards `host_callback_id`, so CLIProxyAPI skips this plugin's router/interceptors for the nested execution and avoids recursion.

## Configuration

```yaml
plugins:
  enabled: true
  dir: "plugins"
  store-sources:
    - "https://raw.githubusercontent.com/mkioutcc/cpa-plugin-retry516/main/registry.json"
  configs:
    retry516:
      enabled: true
      priority: 100
      source_formats:
        - codex
        - openai
        - openai-response
      model_patterns:
        - "gpt-5*"
      trigger_reasoning_tokens: 516
      max_retries: 1
      retry_stream: true
      stream_mode: buffer_then_replay
```

### Fields

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `enabled` | boolean | `true` | When false, the plugin declines all matching requests. |
| `source_formats` | array | `codex`, `openai`, `openai-response` | Protocol formats to wrap. |
| `model_patterns` | array | `gpt-5*` | Case-insensitive glob patterns for requested models. |
| `trigger_reasoning_tokens` | integer | `516` | Exact reasoning-token count that triggers retry. Set `0` to disable retry decisions. |
| `max_retries` | integer | `1` | Maximum automatic retries per request. Values above `3` are capped to `3`. |
| `retry_stream` | boolean | `true` | When false, streaming requests are not routed to the plugin. |
| `stream_mode` | enum | `buffer_then_replay` | Only supported streaming retry mode. |

## Install through Plugin Store

1. Publish this repository to GitHub at `https://github.com/mkioutcc/cpa-plugin-retry516`.
2. Create a tag such as `v0.1.0`.
3. Let GitHub Actions publish release assets:

   ```text
   retry516_0.1.0_linux_amd64.zip
   retry516_0.1.0_linux_arm64.zip
   retry516_0.1.0_darwin_amd64.zip
   retry516_0.1.0_darwin_arm64.zip
   retry516_0.1.0_windows_amd64.zip
   checksums.txt
   ```

4. Add the registry URL to CLIProxyAPI `plugins.store-sources`:

   ```yaml
   plugins:
     enabled: true
     dir: "plugins"
     store-sources:
       - "https://raw.githubusercontent.com/mkioutcc/cpa-plugin-retry516/main/registry.json"
   ```

5. Open the CLIProxyAPI management panel or call:

   ```bash
   curl -X POST \
     -H "Authorization: Bearer <management-secret>" \
     "https://<your-cliproxy-host>/v0/management/plugin-store/retry516/install"
   ```

If your GitHub owner or repository name differs, update these strings before publishing:

- `pluginRepo` in `main.go`
- `repository` and `homepage` in `registry.json`
- registry URL examples in this README
- `module` in `go.mod`, if you want the module path to match the final repo

## Zeabur notes

CLIProxyAPI installs plugin files under `plugins.dir`. On Zeabur, mount a persistent volume for that directory if redeploys recreate the filesystem.

Recommended config:

```yaml
plugins:
  enabled: true
  dir: "/data/plugins"
```

Then mount `/data` as persistent storage in Zeabur.

If persistent storage is not available, install the plugin during image build or startup instead of relying on one-time Plugin Store installation.

## Build locally

```bash
make build
```

Override platform and output directory:

```bash
make build GOOS=linux GOARCH=amd64 BUILD_DIR=./dist
```

The output filename must match the plugin ID:

- Linux / FreeBSD: `retry516.so`
- macOS: `retry516.dylib`
- Windows: `retry516.dll`

## Manual install

Copy the library into CLIProxyAPI's plugin directory:

```text
plugins/<GOOS>/<GOARCH>/retry516.so
```

Then enable it:

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    retry516:
      enabled: true
      priority: 100
```

## Verification

```bash
go test ./...
go vet ./...
make build GOOS=linux GOARCH=amd64 BUILD_DIR=./dist
```

Runtime acceptance checks:

1. A matching `gpt-5*` request with `reasoning_tokens = 516` is retried once.
2. A matching request with any other reasoning-token count is not retried.
3. If the retry also returns `516`, the second response is returned and the plugin stops.
4. Streaming requests are delayed until the stream has been checked and replayed.
5. Non-matching models or protocols bypass the plugin.

## Tradeoffs

- A triggered retry costs another upstream request.
- Streaming TTFT is delayed because the stream must be buffered before replay.
- `516` is a strong empirical signal from the discussion, but it is still a heuristic.
- Standard dynamic-library plugins are trusted in-process code; install only binaries you trust.
