# APRSGO
A modern APRS-IS server written in Go.

We are still under development. Welcome to contribute!

![Gopher image](https://github.com/APRSCN/aprsgo/blob/main/gopher.webp?raw=true)
*Gopher image by [Ian Xia](https://www.ianxia.com), licensed under [Creative Commons 4.0 Attribution licence][cc4-by].*

## Features

- **Client ports**: TCP full-feed and IGate (client-defined filter) ports, with
  optional TLS (including client-certificate login) and SCTP (Linux).
- **Packet submission**: TCP, UDP submit (qAU), and HTTP POST (qAC).
- **Uplink**: TCP uplink with round-robin failover and exponential-backoff reconnect.
- **Core peers**: UDP and TCP server-to-server links (per-peer transport, mixable
  within a group) with aprsc-compatible loop prevention.
- **Q construct**: full qAC/qAS/qAR/qAr/qAo/qAO/qAU/qAX/qAI/qAZ handling with loop detection.
- **Filters**: the 14 standard APRS-IS filter types (`a b d e f g m o p q r s t u`),
  including position-aware `m/`, `f/` and ranged `t/`, plus runtime `#filter` updates.
- **IGate routing**: messages to heard stations are delivered regardless of filter,
  and a correspondent's next position is forwarded as a courtesy.
- **Parser**: positions (uncompressed/compressed), Mic-E, objects, items, messages,
  weather, telemetry, status, queries, NMEA and third-party traffic.
- **Connection health**: TCP keepalive on client and uplink sockets so dead idle
  peers are detected and dropped.
- **Web status page**: a Nuxt SSG dashboard (ElementPlus + Tailwind), embedded into the
  binary and served from memory — single-binary deployment.
- **Stats**: lock-free atomic counters, per-second rates and 30-day time series.

The reusable APRS algorithms (parser, filter, qConstruct, passcode, distance, base91,
client) live in the companion module [`aprsutils`](https://github.com/APRSCN/aprsutils);
server-only logic (listeners, links, stats, web) lives here.

## Building

The web UI is generated as static files and embedded via `//go:embed all:web/dist`, so the
frontend must be built **before** the Go binary. A `Makefile` enforces this order:

```bash
# Build everything (installs web deps, generates the UI, builds the binary)
make build

# Or step by step:
cd web && pnpm install && pnpm generate   # produces web/dist
cd .. && go build .                        # embeds web/dist and compiles
```

Requirements: Go 1.26+, Node.js + pnpm (for the web UI).

Other Makefile targets: `make run`, `make test`, `make test-race`, `make vet`, `make clean`.

## Running

```bash
./aprsgo
```

On first run a default `config.yaml` is written next to the binary. Edit it and restart
(or send `SIGHUP` to reload configuration). The web status page is served on the configured
status port (default `14501`).

## Configuration

`config.yaml` (excerpt):

```yaml
server:
  id: "N0CALL"           # your server callsign
  passcode: "0"          # passcode for the server id
  status:
    host: "[::]"
    port: 14501          # web status + HTTP submit
  listeners:             # client/submit ports (tcp or udp)
    - { name: "Full Feed", mode: "fullfeed", protocol: "tcp", host: "[::]", port: 10152, visible: "hidden" }
    - { name: "Client-Defined Filters", mode: "igate", protocol: "tcp", host: "[::]", port: 14580 }
  uplinks:               # upstream servers (tcp)
    - { name: "Core Rotate", mode: "full", protocol: "tcp", host: "rotate.aprs.net", port: 10152 }
  peer:                  # core peers (optional; per-peer udp/tcp)
    host: "[::]"
    port: 0
    peers: []
```

TLS with client-certificate login and UDP/TCP core peers are configured in the
generated `config.yaml` (see the commented examples there).

## HTTP API

| Method | Path           | Description                          |
|--------|----------------|--------------------------------------|
| GET    | `/api/ping`    | Health check                         |
| GET    | `/api/status`  | Server / uplink / listeners / clients|
| GET    | `/api/stats`   | Time-series statistics               |
| POST   | `/` `/api/submit` | APRS packet submit (octet-stream) |
| GET    | `/`            | Web status dashboard                 |

## Contributors
[![Contributors](https://contrib.rocks/image?repo=APRSCN/aprsgo)](https://github.com/APRSCN/aprsgo/graphs/contributors)

## Star trends
[![Stargazers over time](https://starchart.cc/APRSCN/aprsgo.svg?variant=adaptive)](https://starchart.cc/APRSCN/aprsgo)

[cc4-by]: https://creativecommons.org/licenses/by/4.0/