#!/bin/bash
set -e

cd "$(dirname "$0")"

for arg in "$@"; do
  case "$arg" in
    --dry-run)
      export AESTHETIC_DRY_RUN=1
      echo "Running in DRY RUN mode..."
      ;;
    --debug-css)
      export VITE_DEBUG_CSS=1
      echo "Running with DEBUG CSS overlay enabled..."
      ;;
  esac
done

echo "Building and starting aesthetic-os-installer..."

# Install frontend dependencies if needed
if [ ! -d "frontend/node_modules" ]; then
  echo "Installing frontend dependencies..."
  cd frontend && npm install && cd ..
fi

# Build and run the Wails app
wails dev
