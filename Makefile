VERSION ?= dev
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
GOCACHE ?= /tmp/runwisp-go-cache
GOMODCACHE ?= /tmp/runwisp-go-mod-cache

.PHONY: web build test lint run clean parity

web:
	cd web && if command -v bun >/dev/null 2>&1; then bun install --frozen-lockfile; else npm install --no-audit --no-fund; fi && npm run check && npm run build
	rsync -a --delete web/build/ internal/ui/dist/

build:
	CGO_ENABLED=0 GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go build -trimpath -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)" -o runwisp-manager ./cmd/runwisp-manager

test:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go test ./cmd/... ./internal/...
	cd web && npm run check

lint:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go vet ./cmd/... ./internal/...
	@if command -v golangci-lint >/dev/null 2>&1; then GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) golangci-lint run ./cmd/... ./internal/...; fi

run:
	go run ./cmd/runwisp-manager serve

clean:
	rm -f runwisp-manager
	rm -rf web/build web/.svelte-kit

parity:
	cd web && bun run parity
