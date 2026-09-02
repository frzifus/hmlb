GO ?= go

WASM_OUT  := internal/web/static/game.wasm
WASM_EXEC := internal/web/static/wasm_exec.js
WASM_LDFLAGS := -s -w

.PHONY: assets build native run test test-native race vet clean docker-build

# assets builds the wasm client and copies the matching wasm_exec.js
# (it MUST come from the same toolchain — a mismatch fails confusingly).
assets:
	CGO_ENABLED=0 GOOS=js GOARCH=wasm $(GO) build -ldflags="$(WASM_LDFLAGS)" -o $(WASM_OUT) ./cmd/wasm
	cp "$$($(GO) env GOROOT)/lib/wasm/wasm_exec.js" $(WASM_EXEC)

# build produces the server binary with the assets embedded.
build: assets
	CGO_ENABLED=0 $(GO) build -o bin/flappy-race-server ./cmd/server

# native builds the desktop dev client against a running server. It is the
# one target that needs cgo: Ebitengine's desktop driver links GLFW, so it
# cannot keep the CGO_ENABLED=0 rule the shipped wasm/server builds follow.
# Build-time deps on Fedora: gcc, mesa-libGL-devel, libX11-devel.
native:
	CGO_ENABLED=1 $(GO) build -tags native -o bin/flappy-race-native ./cmd/native

run: build
	./bin/flappy-race-server -addr :8080

# test: host-buildable packages (the js/wasm client packages are skipped).
test:
	CGO_ENABLED=0 $(GO) test ./...

# test-native runs the client reconnect e2e: a live in-process server is
# killed and restarted under the real Game state machine (no GUI).
test-native:
	CGO_ENABLED=1 $(GO) test -tags native -count=1 -run Reconnect ./internal/client/

# race additionally checks for data races (needs cgo for the detector only).
race:
	CGO_ENABLED=1 $(GO) test -race ./...

# vet covers all three build targets: the host packages, the js/wasm
# client, and the native dev client (the pass that needs cgo).
vet:
	CGO_ENABLED=0 $(GO) vet ./...
	CGO_ENABLED=0 GOOS=js GOARCH=wasm $(GO) vet ./...
	CGO_ENABLED=1 $(GO) vet -tags native ./cmd/native ./internal/client/...

clean:
	rm -rf bin $(WASM_OUT) $(WASM_EXEC) data

docker-build:
	docker build -t flappy-race:latest .