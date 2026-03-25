# ClueBot

**Lightweight VPS Incident Monitoring Agent**

ClueBot monitors your Linux server for instability and captures detailed system snapshots when things go wrong. Think of it as a black-box recorder for your VPS.

![Dashboard](https://img.shields.io/badge/platform-linux--amd64-blue)
![Go](https://img.shields.io/badge/Go-1.25+-00ADD8)
![License](https://img.shields.io/badge/license-GPL3-green)

<strong style="color: red;">[THIS PROJECT IS VIBE CODED]</strong>

## What It Monitors

| Monitor | Source | Alert Trigger |
|---------|--------|---------------|
| CPU Usage | `/proc/stat`, `/proc/loadavg` | > 90% |
| Memory | `/proc/meminfo` | > 90% |
| Disk Usage | `syscall.Statfs` | > 90% |
| Process Explosion | `/proc/[pid]/comm` | 5x baseline or > 5000 |
| Services | `systemctl show`, `journalctl` | Service goes inactive/failed |
| Kernel Events | `dmesg`, `journalctl -k` | Panic, OOM, segfault, I/O error |
| Server Restart | `/proc/uptime` | Uptime drops unexpectedly |

## Features

- **Live Dashboard** — Web UI with real-time metrics via WebSocket
- **Incident Snapshots** — Full system state captured at the moment of failure
- **Two-Tier Process Detection** — Fast 1s count catches fork bombs before they kill your server
- **Service Log Capture** — Automatically grabs journalctl logs when a service crashes
- **Kernel Event Detection** — Watches dmesg for panics, OOM kills, segfaults, hardware errors
- **Structured Logging** — Incidents saved as JSON, organized by type and date
- **Authentication** — Login-protected dashboard
- **Zero Dependencies** — Single static binary, no runtime required

## Quick Install

```bash
git clone https://github.com/youruser/cluebot.git
cd cluebot
./build-and-install.sh
sudo systemctl enable --now cluebot
```

Dashboard available at `http://<your-server-ip>:8090`

Default login: `admin` / `admin`

## Manual Build

```bash
go build -o ./build/cluebot ./cmd/cluebot
./build/cluebot start
```

#### Or with [.Runner](https://github.com/mhs003/runner)

```bash
run build
```

## Configuration

Config file: `/var/lib/cluebot/configs/default.yaml`

```yaml
monitor_interval: 5       # Main check interval (seconds)
process_interval: 1       # Fast process check interval (seconds)
http_port: 8090           # Dashboard port
log_dir: /var/lib/cluebot # Data directory
pid_file: /run/cluebot.pid

services:                 # Services to monitor
  - nginx
  - docker
  - postgres
  - redis

thresholds:
  cpu_alert: 90           # CPU usage %
  memory_alert: 90        # Memory usage %
  disk_alert: 90          # Disk usage %
  process_limit: 5000     # Hard process count limit
```

## Incident Storage

```
/var/lib/cluebot/
├── logs/
│   ├── cpu/2026-03-24.log
│   ├── memory/2026-03-24.log
│   ├── disk/2026-03-24.log
│   ├── service/2026-03-24.log
│   ├── process/2026-03-24.log
│   └── kernel/2026-03-24.log
└── incidents/
    ├── 2026-03-24T14-30-00.json
    └── 2026-03-24T15-45-12.json
```

Each incident snapshot contains the full system state at the time of the event: CPU, memory, disk, services (with logs), process counts, and kernel events.

## Systemd Management

```bash
# Start / Stop
sudo systemctl start cluebot
sudo systemctl stop cluebot

# Status
sudo systemctl status cluebot

# View logs
sudo journalctl -u cluebot -f

# Disable
sudo systemctl disable cluebot
```

## API Endpoints

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/` | GET | No | Dashboard (login page) |
| `/api/login` | POST | No | Authenticate, returns token |
| `/api/verify` | GET | Yes | Verify session token |
| `/api/stats` | GET | Yes | Current system metrics |
| `/api/incidents` | GET | Yes | Recent incident list |
| `/ws` | WS | Yes | Live metrics via WebSocket |

## How Process Detection Works

ClueBot uses a two-tier approach to catch fork bombs:

1. **Fast check (1s)** — Counts total processes by reading `/proc` directory entries. Lightweight, runs frequently.
2. **Full check (5s)** — Enumerates all processes by name, builds top-10 list for the dashboard.

The baseline is established from the first 5 samples after startup. If total processes exceed `baseline × 5` or the hard limit (default 5000), an incident is logged immediately.

## Project Structure

```
cluebot/
├── cmd/cluebot/main.go          # Entry point, CLI, monitor loops
├── internal/
│   ├── monitor/                 # System monitors
│   │   ├── cpu.go
│   │   ├── memory.go
│   │   ├── disk.go
│   │   ├── restart.go
│   │   ├── services.go
│   │   ├── processes.go
│   │   └── kernel.go
│   ├── incidents/collector.go   # Snapshot assembly
│   ├── logger/logger.go         # File-based logging
│   ├── config/config.go         # YAML config loader
│   ├── server/                  # HTTP + WebSocket
│   └── cli/cli.go               # PID management
├── configs/default.yaml
├── scripts/cluebot.service      # Systemd unit
└── build-and-install.sh         # Install script
```

## Requirements

- Linux (amd64)
- Root access (for `systemctl`, `journalctl`, `/proc`)
- 256 MB RAM minimum

## License

[GPL3](/LICENSE)
