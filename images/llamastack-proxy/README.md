# LlamaStack OpenAI Proxy

A lightweight Go proxy that automatically injects MCP tools from LlamaStack into OpenAI-compatible API requests.

## Features

- **Automatic Tool Injection**: Fetches tools from LlamaStack and injects them into chat completion requests
- **Tool Execution**: Automatically executes tool calls via LlamaStack's tool runtime
- **OpenAI Compatibility**: Proxies all standard OpenAI endpoints (models, embeddings, etc.)
- **Streaming Support**: Handles both streaming and non-streaming requests
- **Zero Configuration**: Works out of the box with sensible defaults

## How It Works

1. Intercepts requests to `/v1/chat/completions` or `/openai/v1/chat/completions`
2. Fetches available tools from LlamaStack's `/v1/tools` endpoint
3. Converts LlamaStack tool definitions to OpenAI function calling format
4. Injects tools into the request before forwarding to LlamaStack
5. When the model requests tool calls:
   - Executes tools via LlamaStack's `/v1/tool-runtime/invoke` endpoint
   - Adds tool results to the conversation
   - Continues until the model produces a final response

## Usage

### Quick Start

```bash
# Build
go build -o llamastack-proxy

# Run with defaults (proxy on :8322, LlamaStack at localhost:8321)
./llamastack-proxy
```

### With Custom Configuration

```bash
# Set custom LlamaStack URL and proxy port
export LLAMASTACK_URL=http://localhost:8321
export PROXY_PORT=8322
./llamastack-proxy
```

### With Docker

```bash
# Build image
docker build -t llamastack-proxy .

# Run container
docker run -p 8322:8322 \
  -e LLAMASTACK_URL=http://host.docker.internal:8321 \
  llamastack-proxy
```

### Configure Open WebUI

In Open WebUI settings:

1. Go to **Admin Panel** → **Settings** → **Connections**
2. Add OpenAI API connection:
   - **API Base URL**: `http://localhost:8322/v1` (or your proxy address)
   - **API Key**: (any value, not validated)
3. Enable **Native** function calling mode in Advanced Params

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LLAMASTACK_URL` | `http://localhost:8321` | LlamaStack server URL |
| `PROXY_PORT` | `8322` | Port to run the proxy on |

## API Endpoints

### Proxied with Tool Injection
- `POST /v1/chat/completions`
- `POST /openai/v1/chat/completions`

### Direct Proxy (Passthrough)
- `GET /v1/models` - List available models
- `GET /openai/v1/models`
- `POST /v1/embeddings` - Create embeddings
- `POST /openai/v1/embeddings`
- All other endpoints are proxied directly

### Health Check
- `GET /health` - Returns `{"status": "healthy"}`

## Architecture

```
┌─────────────┐         ┌──────────────────┐         ┌──────────────┐
│             │         │                  │         │              │
│  Open WebUI │────────▶│  Proxy (Go)      │────────▶│  LlamaStack  │
│             │         │                  │         │              │
└─────────────┘         │  1. Fetch tools  │         │  - Models    │
                        │  2. Inject tools │         │  - Tools     │
                        │  3. Execute      │         │  - MCP       │
                        │     tool calls   │         │              │
                        └──────────────────┘         └──────────────┘
```

## Tool Execution Flow

```
1. User sends chat message to Open WebUI
2. Open WebUI → Proxy: POST /v1/chat/completions
3. Proxy → LlamaStack: GET /v1/tools (fetch available tools)
4. Proxy: Convert tools to OpenAI format and inject
5. Proxy → LlamaStack: POST /openai/v1/chat/completions (with tools)
6. If model requests tool_calls:
   a. Proxy → LlamaStack: POST /v1/tool-runtime/invoke (for each tool)
   b. Proxy: Add tool results to messages
   c. Goto step 5 (repeat until done)
7. Proxy → Open WebUI: Return final response
```

## Limitations

- **Streaming Mode**: Tool execution is not supported in streaming mode (tools are injected but execution is handled by the backend)
- **Max Iterations**: Tool execution loop is limited to 10 iterations to prevent infinite loops
- **Tool Discovery**: Tools are fetched fresh for each request (no caching currently)

## Development

```bash
# Run with live reload (requires air)
go install github.com/cosmtrek/air@latest
air

# Run tests
go test ./...

# Format code
go fmt ./...
```

## Example

Test the proxy with curl:

```bash
curl -X POST http://localhost:8322/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "meta-llama/Llama-3.2-3B-Instruct",
    "messages": [
      {"role": "user", "content": "Search for the latest news about AI"}
    ]
  }'
```

The proxy will automatically inject available tools (like `web_search`, `wolfram_alpha`, etc.) and execute them if the model decides to use them.

## License

MIT
