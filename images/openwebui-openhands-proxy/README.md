# openwebui-openhands-proxy

A tiny reverse proxy that lets **OpenWebUI** (web or Android app) hold
**multi-turn conversations** with an **OpenHands** agent-server through its
OpenAI-compatible `/v1/chat/completions` gateway — preserving the agent's
workspace/sandbox state across turns.

## Why it exists

The OpenHands agent-server exposes an OpenAI-compatible endpoint, but
conversation continuity is driven by a **server-generated** conversation ID
returned in the `X-OpenHands-ServerConversation-ID` **response** header. To
continue a conversation you must echo that ID back as a **request** header on
follow-up turns.

OpenWebUI can send *static* custom request headers but **cannot capture a
response header and replay it**. So without this proxy, every message starts a
fresh OpenHands conversation and the agent loses its accumulated
state (files it created, terminal session, prior tool activity).

## How it works

For each `POST /v1/chat/completions` request:

1. Derive a stable **thread key** = `sha256(first user message)`. OpenWebUI
   sends the full message history every turn, so the first user message is
   invariant for a chat thread.
2. Look up a stored OpenHands conversation ID for that key.
3. If found, inject `X-OpenHands-ServerConversation-ID` on the upstream request.
4. Capture the ID from the upstream response and persist it for the key.
5. If a stored ID is rejected (upstream 4xx/5xx), retry once without it to
   start a fresh conversation (self-healing for stale IDs).

The `key → conversation ID` map is persisted to a JSON file
(`CONV_STORE_FILE`, default `/data/conversations.json`) so it survives proxy
restarts. Entries are kept forever (no TTL). Mount a PVC at `/data`.

Everything else (`GET /v1/models`, `/alive`, …) is reverse-proxied transparently
with the caller's `Authorization` header passed through unchanged.

### Known limitation

Thread identity is derived from the first user message. Two distinct chats that
start with the *identical* first user message will collide and share one
OpenHands conversation. Acceptable until OpenWebUI can send a per-chat
identifier we can key on (its `{{chat_id}}` header template exists but is not
expanded by the backend — open-webui/open-webui#26989).

## Configuration (env)

| Var | Default | Purpose |
|---|---|---|
| `OPENHANDS_UPSTREAM_URL` | `http://openhands-agent-server.llm.svc.cluster.local:18000` | OpenHands agent-server gateway |
| `PORT` / `PROXY_PORT` | `8080` | Listen port |
| `CONV_STORE_FILE` | `/data/conversations.json` | Persistent mapping file |
| `OTEL_TRACES_EXPORTER` | `otlp` | `otlp` to export traces, `none` to disable (autoexport) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4317` | OTLP collector endpoint |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc` | `grpc` or `http/protobuf` |
| `OTEL_EXPORTER_OTLP_INSECURE` | `false` | `true` for plaintext (in-cluster) collectors |
| `OTEL_SERVICE_NAME` | `openwebui-openhands-proxy` | Resource `service.name` |

## Tracing

The proxy is instrumented with OpenTelemetry traces. The exporter is selected
by the standard OTel environment via the
[`autoexport`](https://pkg.go.dev/go.opentelemetry.io/contrib/exporters/autoexport)
package — `OTEL_TRACES_EXPORTER=otlp` (default) exports; `none` disables export
while keeping the proxy a transparent propagation bridge. Endpoint, protocol,
insecure, and headers come from the usual `OTEL_EXPORTER_OTLP_*` vars.

Each inbound request becomes a **server span**; each outbound call (the
chat-completions client and the catch-all reverse proxy) becomes a **client
span**. The `tracecontext` + `baggage` propagators are wired both ways, so the
proxy bridges OpenWebUI's incoming trace context to the OpenHands upstream as a
`traceparent` request header. The net effect is one distributed trace across
**OpenWebUI → proxy → OpenHands → Ollama**, even when the proxy itself exports
nothing (`OTEL_TRACES_EXPORTER=none`).

The `chat_completions` server span is tagged with `openhands.thread_key`,
`openhands.had_mapping`, `openhands.conversation_id`, and `http.status_code`,
and records a `self-heal retry` event when a stored conversation ID is rejected
upstream.

The in-cluster deployment points the proxy at the observability collector over
HTTP/protobuf (`:4318`, plaintext):

```yaml
OTEL_TRACES_EXPORTER: otlp
OTEL_EXPORTER_OTLP_ENDPOINT: http://backend-collector.observability.svc.cluster.local:4318
OTEL_EXPORTER_OTLP_PROTOCOL: http/protobuf
OTEL_EXPORTER_OTLP_INSECURE: "true"
OTEL_SERVICE_NAME: openwebui-openhands-proxy
```

For local dev without a collector, `make run-dev` sets `OTEL_TRACES_EXPORTER=none`.

## Build / push

```sh
make docker-build
docker login ghcr.io   # if not already
make docker-push IMAGE=ghcr.io/frzifus/openwebui-openhands-proxy TAG=latest
```

Image: `ghcr.io/frzifus/openwebui-openhands-proxy` — distroless, non-root
(UID 65532), CGO disabled (static binary).

## OpenWebUI connection

Point an OpenAI-compatible connection at the proxy instead of the gateway
directly:

| Field | Value |
|---|---|
| Base URL | `http://openwebui-openhands-proxy.llm.svc.cluster.local:8080/v1` |
| API Key | the OpenHands agent-server session key (`LOCAL_BACKEND_API_KEY`) |
| Streaming | **off** (the gateway returns 400 for `stream:true`) |

Models `openhands_glm-5.2` and `openhands_deepseek-v4-flash` appear as usual.