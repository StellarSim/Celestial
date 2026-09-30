#!/bin/bash

# Build both Go binaries from the backend module root.
# Set GOOS and GOARCH to cross compile, e.g. GOARCH=arm64 ./scripts/build.sh

set -e

cd "$(dirname "$0")/../backend"

suffix=""
if [ -n "$GOOS$GOARCH" ]; then
	suffix="-$GOOS-$GOARCH"
fi

echo "Building main server..."
go build -o "../bin/celestial$suffix" ./celestial

echo "Building panel testing tool..."
go build -o "../bin/panel-tester$suffix" ./tools/panel-tester

echo "Build complete!"
echo "  - bin/celestial$suffix (main server)"
echo "  - bin/panel-tester$suffix (panel testing tool)"
