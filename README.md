# Celestial Bridge Simulator

Production backend for a cinematic spaceship bridge simulator with physical controls.

## System Requirements

- Go 1.23 or later
- 8 station PCs running Godot 4.5
- ~15 ESP32 panels for physical controls
- LAN network

## Quick Start

### Build

```bash
./scripts/build.sh
```

### Run

```bash
./scripts/run.sh
```

The server will start on:
- WebSocket: port 8080 (Godot clients)
- TCP: port 9090 (ESP32 panels)

### Verification

```bash
cd backend && go build ./... && go vet ./... && go test ./...
```

## Panel Testing Tool

Test ESP32 panel inputs without physical hardware. Commands map onto the same
action catalog the screens use, so a panel input and a button produce the same
effect:

```bash
./scripts/run-panel-tester.sh [host] [port]
```

Run `help` inside the tool for the full list; the common ones are:
`breaker <name>`, `repair|extinguish|seal <section>`, `deploy|recall <team>`,
`throttle <value>`, `torpedo <bay> <arm|load|lock|fire>`, `phaser <n>`,
`shields <up|down>`, `sensors <mode>`, `scan <target_id>`, `alert <level>`,
`self_destruct <arm|abort>`, and `send <panel_id> <action> [value]` for raw
panel inputs.
