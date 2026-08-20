.PHONY: install build clean test format lint image image-amd64

# The root VERSION file is the source of truth (AGENTS.md → Versioning).
VERSION ?= $(shell cat ../../VERSION 2>/dev/null || echo dev)
# Container engine — detect Podman first, fall back to Docker (same rule as the root Makefile).
DOCKER  := $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)
LDFLAGS := -s -w -X main.version=$(VERSION)

install:
	go mod download

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/aws-ddns ./cmd/aws-ddns

clean:
	rm -rf bin

test:
	go test -race ./...

format:
	gofmt -w .

lint:
	go vet ./...

# Container image for local runs (native architecture).
image:
	$(DOCKER) build --build-arg VERSION=$(VERSION) -t aws-ddns:$(VERSION) .

# Deployment image for linux/amd64 targets. The builder stage cross-compiles,
# so the build machine's architecture does not matter (see README).
image-amd64:
	$(DOCKER) build --platform linux/amd64 --build-arg VERSION=$(VERSION) -t aws-ddns:$(VERSION) .
