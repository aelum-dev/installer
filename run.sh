#!/bin/bash
set -e

cd "$(dirname "$0")"

DRY_RUN=""
if [[ "$1" == "--dry-run" ]]; then
  DRY_RUN="--dry-run"
  echo "Running in DRY RUN mode..."
fi

echo "Building and starting aesthetic-os-installer..."

# Install frontend dependencies if needed
if [ ! -d "frontend/node_modules" ]; then
  echo "Installing frontend dependencies..."
  cd frontend && npm install && cd ..
fi

# Build and run the Wails app
wails dev $DRY_RUN
