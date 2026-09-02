# Build stage: wasm client + wasm_exec.js + static CGO_ENABLED=0 server
# with the assets embedded. No cgo anywhere — the final image is scratch.
FROM golang:1.27-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o internal/web/static/game.wasm ./cmd/wasm \
 && cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" internal/web/static/ \
 && CGO_ENABLED=0 go build -o /flappy-race-server ./cmd/server

FROM scratch
COPY --from=build /flappy-race-server /flappy-race-server

USER 65532:65532
EXPOSE 8080
VOLUME /data

ENTRYPOINT ["/flappy-race-server", "-addr", ":8080", "-data", "/data/state.json"]