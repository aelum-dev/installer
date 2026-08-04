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
├── cmd/
│   ├── helper/          # Privileged helper (sudo-privileged system ops)
│   └── installer-cli/   # CLI installer
├── gui/                 # Wails GUI layer
└── internal/
    ├── core/            # Installer orchestrator (shared by CLI + GUI)
    ├── install/         # Installation steps
    └── shared/          # Types, validators, disk detection

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
- **No magic numbers**: Extract constants into named `const` variables.
- **Single responsibility**: Each function does one thing. Validation functions validate only — they don't mutate state.
- **Live ISO vs installed system**: Never confuse the two. See `RULES.md` for the full distinction.

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
