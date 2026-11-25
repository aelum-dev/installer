# Aelum Installer — Agent Context

Graphical Arch Linux installer. **Wails v2** (Go backend + Svelte 5 frontend) plus a headless CLI
(`aelum-installer-cli`); the GUI spawns the CLI via `sudo` for privileged ops. Offline installer —
every package comes from the live ISO's embedded pacman mirror.

The sibling [live-iso](https://github.com/aelum-dev/live-iso) repo builds the ISO that boots and
launches this installer. This repo is what installs onto the disk.

## Where things live

```
main.go                     — Wails entry point
backend/cli/                — CLI installer (password from stdin, never flags)
backend/gui/                — Wails bindings, navigation, validation, progress plumbing
backend/install/            — the 8 install steps shared by CLI + GUI; step.go = order + weights
backend/install/primitives/ — exec + file-I/O seams; primitivestest/ = fakes for tests
backend/shared/             — ALL constants (config.go), types, disks, region/locale/timezone tables
backend/scripts/            — pkg-list.go (feeds the ISO mirror), gen-ts-constants.go
frontend/                   — Svelte 5 wizard (src/pages, src/lib; i18n in src/translations)
docs/                       — OVERVIEW, EXECUTION_FLOW, TROUBLESHOOTING, SECURITY, CONTRIBUTING
```

Install steps (`install/step.go`): format → mount → base → desktop → users → configure → clear cache → verify.

## Build & test

```bash
go test ./...                       # always before building
golangci-lint run ./... --fix       # after every Go change
./build.sh                          # binaries → build/bin/
cd frontend && npm run test         # Vitest
cd frontend && npm run check        # TypeScript type check
cd frontend && npm run e2e          # Playwright, drives the real app in dry-run mode
```

## Rules

- Commit messages: keep them SHORT — one conventional subject line (`type(scope): summary`, ≤72 chars); at most two or three short body lines when the *why* is not obvious.
