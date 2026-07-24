#!/bin/bash
set -e

cd "$(dirname "$0")"

SHARED_DIR=$HOME"/Projects/shared"

echo "=== Refreshing mirrorlists ==="
go run ./backend/internal/shared/mirrorlist_gen.go || echo "Warning: failed to refresh mirrorlists"

echo ""
echo "=== Starting Wails Dev Server (Debug Mode) ==="
echo "  Frontend: hot-reload enabled at http://localhost:34115"
echo "  Backend:  Go hot-reload enabled"
echo "  Press Ctrl+C to stop"
echo ""

wails dev -tags webkit2_41
