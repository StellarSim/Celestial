#!/bin/bash

set -e

echo "Building Celestial Bridge Simulator..."

cd "$(dirname "$0")/.."

echo "Building main server..."
go build -o bin/celestial cmd/celestial/main.go

echo "Building panel testing tool..."
go build -o bin/panel-tester tools/panel-tester/main.go

echo "Build complete!"
echo "Binaries:"
echo "  - bin/celestial (main server)"
echo "  - bin/panel-tester (panel testing tool)"
