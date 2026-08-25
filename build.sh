#!/bin/bash
# build.sh — Build all AestheticOS Installer binaries.
#
# Usage:
#   ./build.sh              Build only (outputs to build/bin/)
#   SHARED_DIR=/path ./build.sh   Also copy to a shared folder (e.g. VM mount)
#   BIN_DIR=/path ./build.sh      Also copy to a distro airootfs bin dir
#
# Outputs:
#   build/bin/aestheticos-installer          GUI (Wails)
#   build/bin/aestheticos-installer-cli      CLI installer
set -e

cd "$(dirname "$0")"

# ── Smart dependency checks ───────────────────────────────────────────
_tmpfile=$(mktemp)
trap 'rm -f "$_tmpfile"' EXIT

check_command() {
  local cmd="$1" desc="$2" version_flag="${3:---version}"
  if command -v "$cmd" &>/dev/null; then
    "$cmd" "$version_flag" > "$_tmpfile" 2>&1
    local ver_output
    ver_output=$(head -1 "$_tmpfile") && echo "  ✓ $desc: $ver_output" && return 0
  fi
  echo "  ✗ $desc not found — installing…"
  case "$cmd" in
    node)
      echo "Install Node.js (v22+) from your package manager or https://nodejs.org or your package manager."
      exit 1
      ;;
    wails)
      go install github.com/wailsapp/wails/v2/cmd/wails@latest
      ;;
  esac
  # Verify installation succeeded
  if command -v "$cmd" &>/dev/null; then
    "$cmd" "$version_flag" > "$_tmpfile" 2>&1
    local ver_output
    ver_output=$(head -1 "$_tmpfile") && echo "  ✓ $desc: $ver_output"
  else
    echo "  ✗ Failed to install $desc — aborting."
    exit 1
  fi
}

echo "=== Checking dependencies ==="
check_command go "Go" "version"
check_command node "Node.js"
check_command wails "Wails CLI" "version"
echo
# ── End dependency checks ─────────────────────────────────────────────

# Sync dependencies — fails the build if go.mod/go.sum drift from go.sum

go mod tidy

# Override these to copy binaries to additional locations.
# SHARED_DIR  — e.g. /mnt/shared (QEMU VM shared folder)
# BIN_DIR     — e.g. path/to/airootfs/usr/local/bin (distro ISO build)
SHARED_DIR="${SHARED_DIR:-}"
BIN_DIR="${BIN_DIR:-}"

echo "=== Refreshing mirrorlists ==="
go run ./backend/shared/mirrorlist_gen.go || echo "Warning: failed to refresh mirrorlists"

echo "=== Generating TypeScript constants from Go ==="
go run ./backend/scripts/gen-ts-constants.go

echo "=== Building frontend ==="
if [ ! -d "frontend/node_modules" ]; then
  echo "Installing frontend dependencies..."
  cd frontend && npm install && cd ..
fi
cd frontend && npm run build && cd ..

# Clean previous build artifacts to avoid stale binaries
echo "=== Cleaning previous build ==="
mkdir -p build/bin
rm -f build/bin/aestheticos-installer build/bin/aestheticos-installer-cli

echo "=== Building Go binaries with Wails ==="
wails build -clean -tags webkit2_41

echo "=== Building aestheticos-installer-cli ==="
go build -ldflags "-X main.Version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')" -o build/bin/aestheticos-installer-cli ./backend/cli

if [ -n "$SHARED_DIR" ]; then
	echo "=== Copying executables to ${SHARED_DIR} ==="
	mkdir -p "$SHARED_DIR"
	cp build/bin/aestheticos-installer "$SHARED_DIR/"
	cp build/bin/aestheticos-installer-cli "$SHARED_DIR/"
	chmod +x "$SHARED_DIR/aestheticos-installer" "$SHARED_DIR/aestheticos-installer-cli"
fi

if [ -n "$BIN_DIR" ]; then
	echo "=== Copying executables to distro ${BIN_DIR} ==="
	mkdir -p "$BIN_DIR"
	cp build/bin/aestheticos-installer "$BIN_DIR/"
	cp build/bin/aestheticos-installer-cli "$BIN_DIR/"
fi

echo "=== Done! ==="
