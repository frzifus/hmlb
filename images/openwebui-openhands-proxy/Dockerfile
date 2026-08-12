# Build stage — fully static binary (CGO disabled).
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -trimpath -o openwebui-openhands-proxy .

# Final stage — distroless, non-root, includes CA certificates.
# The proxy is a static binary with no shell, no package manager, minimal attack
# surface. Runs as UID/GID 65532 (distroless nonroot).
FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /

COPY --from=builder /app/openwebui-openhands-proxy /openwebui-openhands-proxy

# Persistent conversation-ID mapping lives here (mount a PVC at /data).
ENV CONV_STORE_FILE=/data/conversations.json
ENV OPENHANDS_UPSTREAM_URL=http://openhands-agent-server.llm.svc.cluster.local:18000
ENV PORT=8080

EXPOSE 8080

ENTRYPOINT ["/openwebui-openhands-proxy"]