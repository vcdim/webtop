# portview

A lightweight, single-binary web-based port monitor for Linux.

See real-time port → process → PID → user mappings in your browser.

![License](https://img.shields.io/badge/license-MIT-blue.svg)

## Features

- **Web UI** — Clean, dark-themed dashboard accessible via browser
- **Real-time** — WebSocket auto-refresh, no manual reload
- **Deduplication** — Groups duplicate port+PID+proto entries, shows count (e.g. ×40)
- **Single binary** — Zero runtime dependencies, just run and open browser
- **Lightweight** — Uses `ss` and `/proc` for data collection

## Quick Start

```
./portview
```

Open http://localhost:8080 in your browser.

## Options

```
./portview -port 9090 -interval 5s
```

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | `8080` | HTTP listen port |
| `-interval` | `2s` | Data refresh interval |

## Build from Source

```
go build -o portview .
```

Cross-compile for Linux from macOS:

```
GOOS=linux GOARCH=amd64 go build -o portview .
```

## Requirements

- Linux with `ss` command available (part of `iproute2`, installed by default)
- Root/sudo recommended for full process visibility

## License

MIT
