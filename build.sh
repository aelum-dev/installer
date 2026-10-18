#!/bin/bash
# build.sh — Build all Aelum Installer binaries.
#
# Usage:
#   ./build.sh              Build only (outputs to build/bin/)
#   SHARED_DIR=/path ./build.sh   Also copy to a shared folder (e.g. VM mount)
#   BIN_DIR=/path ./build.sh      Also copy to a distro airootfs bin dir
#
# Outputs:
#   build/bin/aelum-installer          GUI (Wails)
#   build/bin/aelum-installer-cli      CLI installer
set -e

# Disable WebKit's DMABUF/GPU renderer so the Wails window (and detached
# devtools) don't render black on this host. See Wails issue #2977.
# (Only takes effect when the built binary is actually launched, not during
#  this build step.)
export WEBKIT_DISABLE_DMABUF_RENDERER=1

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
rm -f build/bin/aelum-installer build/bin/aelum-installer-cli

# ── Patch Wails v2's Linux window startup ─────────────────────────────
# Wails' Linux Run() hides a StartHidden window *before* calling
# gtk_widget_show_all(), so the hide is a no-op and the window is mapped while
# WebKit still shows its light pre-page fill — a white flash before the dark
# page appears. The dev host wins that race, the live ISO usually loses it.
# Moving the hide after show_all keeps the window unmapped until main.ts calls
# WindowShow() from its first painted frame.
#
# This edits the module cache, so it is idempotent and fails loudly if the
# anchor moves — a Wails bump must not silently bring the flash back.
echo "=== Patching Wails v2 window startup ==="
WAILS_WINDOW="$(go list -m -f '{{.Dir}}' github.com/wailsapp/wails/v2)/internal/frontend/desktop/linux/window.go"
if grep -q "AELUM: keep the window hidden" "$WAILS_WINDOW"; then
  echo "  already patched"
else
  chmod u+w "$WAILS_WINDOW" "$(dirname "$WAILS_WINDOW")"
  perl -0pi -e 's/\tif w\.appoptions\.StartHidden \{\n\t\tw\.Hide\(\)\n\t\}\n(\tC\.gtk_widget_show_all\(w\.asGTKWidget\(\)\))/$1\n\tif w.appoptions.StartHidden {\n\t\t\/\/ AELUM: keep the window hidden until the frontend shows it.\n\t\tw.Hide()\n\t}/' "$WAILS_WINDOW"
  if grep -q "AELUM: keep the window hidden" "$WAILS_WINDOW"; then
    echo "  patched"
  else
    echo "  ERROR: Wails Run() anchor not found — update this patch for the current Wails version" >&2
    exit 1
  fi
fi

echo "=== Building Go binaries with Wails ==="
wails build -clean -tags webkit2_41

echo "=== Building aelum-installer-cli ==="
go build -ldflags "-X main.Version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')" -o build/bin/aelum-installer-cli ./backend/cli

if [ -n "$SHARED_DIR" ]; then
	echo "=== Copying executables to ${SHARED_DIR} ==="
	mkdir -p "$SHARED_DIR"
	cp build/bin/aelum-installer "$SHARED_DIR/"
	cp build/bin/aelum-installer-cli "$SHARED_DIR/"
	chmod +x "$SHARED_DIR/aelum-installer" "$SHARED_DIR/aelum-installer-cli"
fi

if [ -n "$BIN_DIR" ]; then
	echo "=== Copying executables to distro ${BIN_DIR} ==="
	mkdir -p "$BIN_DIR"
	cp build/bin/aelum-installer "$BIN_DIR/"
	cp build/bin/aelum-installer-cli "$BIN_DIR/"
fi

echo "=== Done! ==="
