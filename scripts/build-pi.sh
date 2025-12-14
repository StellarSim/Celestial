#!/bin/bash

set -e

echo "Building Celestial for Raspberry Pi 5 (ARM64)..."

cd "$(dirname "$0")/.."

export GOOS=linux
export GOARCH=arm64

echo "Building main server for ARM64..."
go build -o bin/celestial-arm64 cmd/celestial/main.go

echo "Building panel testing tool for ARM64..."
go build -o bin/panel-tester-arm64 tools/panel-tester/main.go

echo "Build complete for Raspberry Pi 5!"
echo "Binaries:"
echo "  - bin/celestial-arm64 (main server)"
echo "  - bin/panel-tester-arm64 (panel testing tool)"
echo ""
echo "To deploy to Raspberry Pi 5:"
echo "  scp bin/celestial-arm64 pi@<raspberry-pi-ip>:~/celestial/celestial"
echo "  scp -r configs pi@<raspberry-pi-ip>:~/celestial/"
echo "  scp -r missions pi@<raspberry-pi-ip>:~/celestial/"
