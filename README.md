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
- Web panels: port 8080 (`/panels`, one page per panel id)

### Verification

```bash
cd backend && go build ./... && go vet ./... && go test ./...
```

## Web Panels

Browser fallback for rooms without ESP32 hardware. Open
`http://<server>:8080/panels` and pick a panel; each page sends through the
same action catalog as the screens and physical panels, and renders the same
per-panel state.

## Panel Testing Tool

Test ESP32 panel inputs without physical hardware. Commands map onto the same
action catalog the screens use, so a panel input and a button produce the same
effect:

```bash
./scripts/run-panel-tester.sh [host] [port]
```

Run `help` inside the tool for the full list; the common ones are:
`breaker <name>`, `repair|extinguish|seal <section>`, `deploy|recall <team>`,
`throttle <value>`, `torpedo <bay> <arm|disarm|load|fire>`, `phaser <n>`,
`shields <up|down>`, `sensors <mode>`, `scan <target_id>`, `alert <level>`,
`self_destruct <arm|abort>`, and `send <panel_id> <action> [value]` for raw
panel inputs.
