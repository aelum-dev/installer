# AestheticOS Installer — Makefile
#
# Targets:
#   make build      Build all binaries (go test + build.sh)
#   make dev        Start Wails dev server with hot-reload
#   make test       Run all tests (Go + frontend)
#   make lint       Run golangci-lint + svelte-check
#   make clean      Remove build artifacts
#   make fmt        Format Go and TypeScript code

.PHONY: build dev test lint clean fmt

# ── Build ─────────────────────────────────────────────────────────────────────

build:
	./build.sh

# ── Development ───────────────────────────────────────────────────────────────

dev:
	./run.sh

# ── Testing ───────────────────────────────────────────────────────────────────

test:
	go test ./...
	cd frontend && npm run test

test-verbose:
	go test -v ./...
	cd frontend && npm run test

# ── Linting ───────────────────────────────────────────────────────────────────

lint:
	golangci-lint run
	cd frontend && npm run check

# ── Formatting ────────────────────────────────────────────────────────────────

fmt:
	gofmt -w backend/
	cd frontend && npx prettier --write "src/**/*.{ts,svelte}" 2>/dev/null || true

# ── Clean ─────────────────────────────────────────────────────────────────────

clean:
	rm -rf build/bin
	rm -rf frontend/dist
	rm -rf frontend/.svelte-kit
	rm -rf frontend/.vite
