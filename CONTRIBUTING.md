# Contributing to AestheticOS Installer

Thanks for your interest in contributing! This guide covers the basics.

## Development Rules

See [RULES.md](./RULES.md) for detailed conventions. Key points:

- **Do not auto-commit.** Always ask for approval before committing.
- **Build before testing.** Run `./build.sh` before testing — never assume old binaries.
- **Never guess tool behavior.** Check man pages or ask before assuming how `parted`, `bootctl`, `pacstrap`, etc. work.
- **Live ISO vs Installed System.** Never confuse the two. See RULES.md for the full distinction.

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

## Reporting Bugs

Open an issue with:
- What you expected to happen
- What actually happened
- Steps to reproduce
- Your environment (Arch version, disk type, etc.)

## Adding Features

Open an issue first to discuss the approach before writing code. Big changes benefit from early feedback.
