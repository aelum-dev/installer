# Contributing to AestheticOS Installer

Thanks for your interest in contributing! This guide covers the basics.

## Building

```bash
# Build all binaries (outputs to build/bin/)
./build.sh

# Development mode (hot-reload)
./run.sh
```

### Prerequisites

- Go 1.25+
- Node.js 22+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)
- `webkit2gtk-4.1` (Linux) or `WebView2` (Windows)

## Testing

```bash
# Go tests (unit + integration)
go test ./...

# Frontend tests
cd frontend && npm run test

# Type checking
cd frontend && npm run check

# Lint + auto-fix formatting (run after every Go change)
golangci-lint run ./... --fix
```

## Project Structure

```
backend/
├── cli/                 # CLI installer (password read from stdin)
├── gui/                 # Wails GUI layer
│   ├── app.go           # Wails app struct, StartUp, OnBeforeClose, cancel/cleanup
│   ├── bindings.go      # Wails API bindings (GetDisks, GetLanguages, GetFileSystem)
│   ├── installation.go  # StartInstallation (spawns CLI via sudo, reads JSON from stderr)
│   ├── navigation.go    # Page IDs, navigation state
│   ├── root_check.go    # Sudo verification
│   └── validation.go    # Validate + Save methods
├── install/             # Installation orchestrator + steps
│   ├── installer.go     # Installer orchestrator (Install()/Rollback())
│   ├── step.go          # Step interface + stepWeights() (order + progress weights)
│   ├── context.go       # InstallContext (shared state, Cancel channel, Logger)
│   ├── command.go       # Run(), runChroot(), writeFile(), appendFile()
│   ├── errors.go        # Error types
│   ├── logger.go        # Logger interface + ConsoleLogger
│   ├── cleanup.go       # Lazy recursive mount cleanup
│   ├── format/          # Partitioning + formatting (+ btrfs subvolumes)
│   ├── mount/           # Mount filesystem + verification
│   ├── installbase/     # Base pacstrap
│   ├── installdesktop/  # Desktop pacstrap
│   ├── users/           # User account setup
│   ├── configure/       # Merged: fstab→bootloader→system→greetd→plymouth→zram→services
│   ├── clearcache/      # Clear pacman cache in chroot
│   ├── greetd/          # greetd config (config, sync, memory)
│   ├── primitives/      # Low-level disk primitives
│   ├── progress/        # Pacstrap progress tracker
│   └── *_test.go        # Unit + integration tests
├── validation/          # Centralized validation
└── shared/              # Types, validators, disk detection, regions, mirrorlists

frontend/src/
├── pages/               # 6 wizard pages
├── components/          # Reusable UI components
├── lib/                 # State, navigation, validation, i18n
└── translations/        # i18n locale JSON files
```

## Submitting Changes

1. Fork the repo and create a feature branch
2. Make your changes
3. Run `go test ./...` and `cd frontend && npm run test`
4. Run `./build.sh` to verify the full build
5. Commit with a clear message, then push and open a merge request

## Code Style

- **Go**: Follow [Effective Go](https://go.dev/doc/effective_go) and `gofmt`. Run `go vet ./...` and `go test ./...` before submitting.
- **Svelte/TypeScript**: Follow [Svelte docs](https://svelte.dev/docs) conventions. Run `npm run check` for type checking.
- **No magic numbers**: Extract constants into named `const` variables. All constants live in `backend/shared/config.go`.
- **Single responsibility**: Each function does one thing. Validation functions validate only — they don't mutate state.
- **Live ISO vs installed system**: Never confuse the two. See `RULES.md` for the full distinction.
- **Executor pattern**: Use the `Executor` interface for command execution in testable code. Direct `Run()` / `runChroot()` calls are fine for simple cases.

## Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]
```

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`

Examples:
```
fix(core): return error on wipefs failure in Rollback
feat(cli): add --version flag with ldflags injection
refactor(format): replace magic numbers in stepWeights with constants
```

## Merge Requests

1. Fork and create a feature branch
2. Write or update tests for your changes
3. Run the full test suite: `go test ./...` and `cd frontend && npm run test`
4. Build to verify: `./build.sh`
5. Squash commits into a single logical commit per feature
6. Open a merge request with:
   - Clear description of what and why
   - Link to related issue
   - Screenshots for UI changes
   - Testing notes (VM test results, etc.)

## Reporting Bugs

Open an issue with:
- What you expected to happen
- What actually happened
- Steps to reproduce
- Your environment (Arch version, disk type, etc.)

## Adding Features

Open an issue first to discuss the approach before writing code. Big changes benefit from early feedback.
