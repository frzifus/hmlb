CONTENT_IMAGE ?= ghcr.io/frzifus/blog-klimlive-content:latest
SERVE_IMAGE ?= ghcr.io/frzifus/blog-klimlive-serve:latest

.PHONY: dev build sync image-content image-serve

dev:
	cd hugo && hugo server -D --bind 0.0.0.0

build:
	cd hugo && hugo --minify

sync:
	go run ./cmd/sync

image-content:
	podman build -f Containerfile.content -t $(CONTENT_IMAGE) .

image-serve:
	podman build -f Containerfile.serve -t $(SERVE_IMAGE) .
