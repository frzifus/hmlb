.PHONY: build run run-dev clean docker-build docker-run vet

IMAGE = ghcr.io/frzifus/openwebui-openhands-proxy
TAG   = latest

# Build the Go binary locally
build:
	CGO_ENABLED=0 go build -trimpath -o openwebui-openhands-proxy .

# Run locally against an in-cluster upstream (port-forward 18000 first)
run: build
	./openwebui-openhands-proxy

run-dev: build
	OPENHANDS_UPSTREAM_URL=http://localhost:18000 PORT=8080 \
	CONV_STORE_FILE=./conversations.json \
	OTEL_TRACES_EXPORTER=none ./openwebui-openhands-proxy

vet:
	go vet ./...

# Build the distroless image
docker-build:
	docker build -t $(IMAGE):$(TAG) .

# Push to GHCR (docker login ghcr.io first)
docker-push: docker-build
	docker push $(IMAGE):$(TAG)

docker-run: docker-build
	docker run --rm -p 8080:8080 -v $(PWD)/conversations.json:/data/conversations.json $(IMAGE):$(TAG)

clean:
	rm -f openwebui-openhands-proxy conversations.json