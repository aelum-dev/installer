#!/bin/bash
set -e

# Disable WebKit's DMABUF/GPU renderer: on this host the detached devtools
# (and, on some setups, the whole window) renders black because KMS/GBM
# buffer creation fails. See Wails issue #2977.
export WEBKIT_DISABLE_DMABUF_RENDERER=1

cd "$(dirname "$0")"

# The distro workspace's shared/ is the directory the test VM mounts at /shared,
# so the dev build and the QEMU tests use the same one. Exported: the installer
# reads SHARED_DIR from the environment.
SHARED_DIR="$(cd .. && pwd)/shared"
export SHARED_DIR

# echo "=== Refreshing mirrorlists ==="
# go run ./backend/shared/mirrorlist_gen.go || echo "Warning: failed to refresh mirrorlists"

echo ""
echo "=== Starting Wails Dev Server (Debug Mode) ==="
echo "  Frontend: hot-reload enabled at http://localhost:34115"
echo "  Backend:  Go hot-reload enabled"
echo "  Press Ctrl+C to stop"
echo ""

wails dev -tags webkit2_41
