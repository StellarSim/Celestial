#!/bin/bash

cd "$(dirname "$0")/.."

if [ ! -f "bin/celestial" ]; then
    echo "Server binary not found. Building..."
    ./scripts/build.sh
fi

echo "Starting Celestial Bridge Simulator..."
./bin/celestial
