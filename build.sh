#!/bin/bash
set -e

cd "$(dirname "$0")"

SHARED_DIR="/home/power007/Projects/shared"

echo "=== Building frontend ==="
if [ ! -d "frontend/node_modules" ]; then
  echo "Installing frontend dependencies..."
  cd frontend && npm install && cd ..
fi
cd frontend && npm run build && cd ..

echo "=== Building Go binaries with Wails ==="
wails build -clean

echo "=== Building root-helper ==="
go build -o build/bin/root-helper ./cmd/root-helper

echo "=== Building installer (CLI) ==="
go build -o build/bin/installer ./cmd/installer

echo "=== Copying executables to ${SHARED_DIR} ==="
mkdir -p "$SHARED_DIR"
cp build/bin/aesthetic-os-installer "$SHARED_DIR/"
cp build/bin/root-helper "$SHARED_DIR/"
cp build/bin/installer "$SHARED_DIR/"
chmod +x "$SHARED_DIR/aesthetic-os-installer" "$SHARED_DIR/root-helper" "$SHARED_DIR/installer"

echo "Done! Executables built:"
echo "  GUI:    build/bin/aesthetic-os-installer"
echo "  Helper: build/bin/root-helper"
echo "  CLI:    build/bin/installer"
