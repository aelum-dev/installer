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

echo "=== Building Go binary with Wails ==="
wails build -clean

echo "=== Copying executable to ${SHARED_DIR} ==="
mkdir -p "$SHARED_DIR"
cp build/bin/aesthetic-os-installer "$SHARED_DIR/"
chmod +x "$SHARED_DIR/aesthetic-os-installer"

echo "Done! Executable copied to: ${SHARED_DIR}/aesthetic-os-installer"
