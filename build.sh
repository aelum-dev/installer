#!/bin/bash
set -e

cd "$(dirname "$0")"

SHARED_DIR=$HOME"/Projects/shared"
BIN_DIR=$HOME"/Projects/AestheticOS/airootfs/usr/local/bin"

echo "=== Refreshing mirrorlists ==="
go run ./backend/internal/shared/mirrorlist_gen.go || echo "Warning: failed to refresh mirrorlists"

echo "=== Building frontend ==="
if [ ! -d "frontend/node_modules" ]; then
  echo "Installing frontend dependencies..."
  cd frontend && npm install && cd ..
fi
cd frontend && npm run build && cd ..

echo "=== Building Go binaries with Wails ==="
wails build -clean -tags webkit2_41

echo "=== Building aestheticos-helper ==="
go build -o build/bin/aestheticos-helper ./backend/cmd/helper

echo "=== Building aestheticos-installer-cli ==="
go build -o build/bin/aestheticos-installer-cli ./backend/cmd/installer-cli

echo "=== Copying executables to ${SHARED_DIR} ==="
mkdir -p "$SHARED_DIR"
cp build/bin/aestheticos-installer "$SHARED_DIR/"
cp build/bin/aestheticos-helper "$SHARED_DIR/"
cp build/bin/aestheticos-installer-cli "$SHARED_DIR/"
chmod +x "$SHARED_DIR/aestheticos-installer" "$SHARED_DIR/aestheticos-helper" "$SHARED_DIR/aestheticos-installer-cli"

echo "=== Copying executables to distro ${BIN_DIR} ==="
mkdir -p "$BIN_DIR"
cp build/bin/aestheticos-installer "$BIN_DIR/"
cp build/bin/aestheticos-helper "$BIN_DIR/"
cp build/bin/aestheticos-installer-cli "$BIN_DIR/"

echo "=== Done! ==="
