#!/bin/bash
set -e

cd "$(dirname "$0")"

if [[ "$1" == "--dry-run" ]]; then
  export AESTHETIC_DRY_RUN=1
  echo "Running in DRY RUN mode..."
fi

echo "Building and starting aesthetic-os-installer..."

# Install frontend dependencies if needed
if [ ! -d "frontend/node_modules" ]; then
  echo "Installing frontend dependencies..."
  cd frontend && npm install && cd ..
fi

# Build and run the Wails app
wails dev
