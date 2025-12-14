# Celestial Bridge Simulator

Production backend for a cinematic spaceship bridge simulator with physical controls.

## System Requirements

- Go 1.23 or later
- Raspberry Pi 5 (Debian Linux) for production deployment
- 8 station PCs running Godot 4.5
- ~15 ESP32 panels for physical controls
- LAN network

## Quick Start

### Build

```bash
./scripts/build.sh
```

For Raspberry Pi 5:
```bash
./scripts/build-pi.sh
```

### Run

```bash
./scripts/run.sh
```

The server will start on:
- WebSocket: port 8080 (Godot clients)
- TCP: port 9090 (ESP32 panels)

### Configuration

Edit `configs/server.yaml` to adjust:
- `tick_rate`: Simulation update frequency (default: 60 Hz)
- `websocket_port`: WebSocket server port (default: 8080)
- `tcp_port`: TCP server port (default: 9090)
- `snapshot_interval`: Time between automatic state snapshots in seconds (default: 20)

## Ship Configuration

Ship classes are defined in `configs/ships/*.yaml`. Each ship defines:
- Physics properties (mass, speed, acceleration, turn rate)
- Engines, weapons, shields, hull sections
- Subsystems and launch bays

Included ship classes:
- `player_cruiser`: Player ship (Federation Cruiser)
- `enemy_frigate`: Enemy frigate
- `enemy_dreadnought`: Enemy capital ship

## Panel Configuration

Physical panel mappings are defined in `configs/panels.yaml`. Each panel maps physical inputs to ship systems and actions.

## Missions

Mission scripts are located in `missions/*.lua`. Included missions:
- `border_patrol.lua`: Patrol mission with combat encounters
- `rescue_operation.lua`: Rescue mission with escort objectives

## Panel Testing Tool

Test ESP32 panel inputs without physical hardware:

```bash
./scripts/run-panel-tester.sh [host] [port]
```

Available commands:
- `torpedo <bay> <action> [value]` - Control torpedo bays (arm, load, lock, fire)
- `phaser <num> fire` - Fire phaser arrays
- `breaker <name> <on|off>` - Toggle power breakers
- `fire <location>` - Extinguish fires
- `breach <location>` - Seal hull breaches
- `shields <on|off>` - Toggle shields
- `alert <level>` - Set alert level

## Deployment to Raspberry Pi 5

1. Build for ARM64:
   ```bash
   ./scripts/build-pi.sh
   ```

2. Copy files to Pi:
   ```bash
   scp bin/celestial-arm64 pi@<ip>:~/celestial/celestial
   scp -r configs pi@<ip>:~/celestial/
   scp -r missions pi@<ip>:~/celestial/
   ```

3. Run on Pi:
   ```bash
   ssh pi@<ip>
   cd ~/celestial
   ./celestial
   ```

## Architecture

- `cmd/celestial/` - Main server entry point
- `internal/simulation/` - Physics and world simulation
- `internal/ship/` - Ship systems and state
- `internal/damage/` - Damage model and repair
- `internal/ai/` - NPC ship AI
- `internal/mission/` - Lua mission scripting
- `internal/network/` - WebSocket and TCP servers
- `internal/input/` - Action registry and routing
- `internal/gm/` - Game Master controls
- `internal/config/` - Configuration loading
- `tools/panel-tester/` - Panel testing tool Bridge Simulator

A real-time starship bridge simulation system designed to provide an immersive multi-station bridge experience.

*"To boldly go where no simulation has gone before."*
