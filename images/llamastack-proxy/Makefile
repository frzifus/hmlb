.PHONY: build run clean docker-build docker-run test

# Build the Go binary
build:
	GOROOT=/usr/lib/golang go build -o llamastack-proxy main.go

# Run locally
run: build
	./llamastack-proxy

# Run with custom config
run-dev: build
	LLAMASTACK_URL=http://localhost:8321 PROXY_PORT=8322 ./llamastack-proxy

# Clean build artifacts
clean:
	rm -f llamastack-proxy

# Build Docker image
docker-build:
	docker build -t llamastack-proxy:latest .

# Run Docker container
docker-run: docker-build
	docker run -p 8322:8322 \
		-e LLAMASTACK_URL=http://host.docker.internal:8321 \
		llamastack-proxy:latest

# Run with docker-compose
compose-up:
	docker-compose up -d

compose-down:
	docker-compose down

# Test the proxy
test-health:
	curl http://localhost:8322/health

test-models:
	curl http://localhost:8322/v1/models

test-chat:
	curl -X POST http://localhost:8322/v1/chat/completions \
		-H "Content-Type: application/json" \
		-d '{"model":"meta-llama/Llama-3.2-3B-Instruct","messages":[{"role":"user","content":"Hello!"}]}'
