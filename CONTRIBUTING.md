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
# Go tests
go test ./backend/...

# Frontend tests
cd frontend && npm run test

# Type checking
cd frontend && npm run check
```

CI runs all of the above on every push/PR via [GitHub Actions](./.github/workflows/ci.yml).

## Project Structure

```
backend/
├── cli/                 # CLI installer
├── gui/                 # Wails GUI layer
├── install/             # Installation steps + orchestrator
│   ├── installer.go     # Installer struct + Install()/RunStep()/Rollback()
│   ├── step.go          # Step interface + step implementations
│   ├── plan.go          # Plan struct for step weights/percentages
│   ├── executor.go      # Executor interface
│   ├── configuration.go # Merged ConfigureSystem step
│   ├── errors.go        # Error types
│   ├── logger.go        # Logger interface + ConsoleLogger
│   ├── format.go        # Partitioning + formatting
│   ├── mount.go         # Mount filesystem
│   ├── install.go       # Pacstrap progress tracking
│   ├── install_base.go  # Base package installation
│   ├── install_desktop.go # Desktop package installation
│   ├── fstab.go         # fstab generation
│   ├── bootloader.go    # systemd-boot configuration
│   ├── users.go         # User account setup
│   ├── system.go        # Hostname, locale, timezone, mirrors
│   ├── greetd.go        # Login manager configuration
│   ├── greetd/          # greetd package (config, sync, memory)
│   ├── plymouth.go      # Plymouth BGRT silent boot
│   ├── services.go      # systemd service enabling
│   ├── zram.go          # Compressed swap configuration
│   ├── copy.go          # ISO-to-target file copying
│   ├── cleanup.go       # Mount cleanup
│   ├── command.go       # Run(), runChroot(), writeFile(), appendFile()
│   ├── rollback.go      # Rollback partial installations
│   ├── context.go       # InstallContext
│   ├── checks.go        # Validation helpers
│   └── progress/        # Pacstrap progress tracker
├── validation/          # Centralized validation
└── shared/              # Types, validators, disk detection, regions

frontend/src/
├── pages/               # 6 wizard pages
├── components/          # Reusable UI components
└── lib/                 # State, navigation, validation
```

## Submitting Changes

1. Fork the repo and create a feature branch
2. Make your changes
3. Run `go test ./backend/...` and `cd frontend && npm run test`
4. Run `./build.sh` to verify the full build
5. Commit with a clear message, then push and open a PR

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

## Pull Requests

1. Fork and create a feature branch
2. Write or update tests for your changes
3. Run the full test suite: `go test ./...` and `cd frontend && npm run test`
4. Build to verify: `./build.sh`
5. Squash commits into a single logical commit per feature
6. Open a PR with:
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
