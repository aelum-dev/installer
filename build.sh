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

echo "=== Building aestheticos-helper ==="
go build -o build/bin/aestheticos-helper ./backend/helper

echo "=== Building aestheticos-installer-cli ==="
go build -o build/bin/aestheticos-installer-cli ./backend/cli

echo "=== Copying executables to ${SHARED_DIR} ==="
mkdir -p "$SHARED_DIR"
cp build/bin/aestheticos-installer "$SHARED_DIR/"
cp build/bin/aestheticos-helper "$SHARED_DIR/"
cp build/bin/aestheticos-installer-cli "$SHARED_DIR/"
chmod +x "$SHARED_DIR/aestheticos-installer" "$SHARED_DIR/aestheticos-helper" "$SHARED_DIR/aestheticos-installer-cli"

echo "Done! Executables built:"
echo "  GUI:     build/bin/aestheticos-installer"
echo "  Helper:  build/bin/aestheticos-helper"
echo "  CLI:     build/bin/aestheticos-installer-cli"
