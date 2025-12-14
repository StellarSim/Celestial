#!/bin/bash

cd "$(dirname "$0")/.."

if [ ! -f "bin/panel-tester" ]; then
    echo "Panel tester binary not found. Building..."
    ./scripts/build.sh
fi

HOST=${1:-localhost}
PORT=${2:-9090}

echo "Starting Panel Testing Tool..."
echo "Connecting to $HOST:$PORT"
./bin/panel-tester -host "$HOST" -port "$PORT"
