# LatinaSub

> High-performance proxy scraper and in-process verification engine built with Go 1.24+ and embedded sing-box core, powered by [`github.com/LalatinaHub/common`](https://github.com/LalatinaHub/common).

---

## ⚡ Overview

LatinaSub scrapes, decodes, and verifies proxy nodes (VMess, VLESS, Trojan, Shadowsocks) concurrently. It tests connectivity in-process using an embedded `sing-box` instance across multiple connection modes (CDN, SNI, Direct), filters dead nodes with a fast deterministic hash blacklist, enriches results with GeoIP & IATA airport codes, and persists active nodes directly into Turso LibSQL database for consumption by [LatinaApi](https://github.com/LalatinaHub/LatinaApi).

---

## 🚀 Features

- **Leaf Module Architecture**: Leverages standard models, parsers, formatters, and database connection pooling from `github.com/LalatinaHub/common`.
- **Pure-Go Embedded sing-box**: In-process sandbox verification with ephemeral port isolation—no external binaries needed.
- **Multi-Mode Mutation**: Automatically tests nodes with CDN anycast IP (`104.18.2.2`) and SNI bug host (`meet.google.com`).
- **Deterministic Blacklist Engine**: Fast $O(1)$ in-memory set with MD5 fingerprinting and atomic file persistence (`blacklist.txt`).
- **Turso LibSQL Synchronizer**: Parameterized SQL transactions eliminating SQL injection risks.
- **Multi-Format Subscription Exporter**: Generates subscription feeds, Clash Meta (`result/clash.yaml`), and sing-box JSON (`result/singbox.json`).
- **Telemetry & Monitoring**: Telegram Bot status heartbeats and execution reports via Echotron.

---

## 📦 Getting Started

### Prerequisites

- **Go 1.24+**

### Build

```bash
# Build executable with sing-box build tags
go build -v -tags "with_utls,with_grpc" -o bin/latinasub ./cmd/latinasub
```

### Running

```bash
# Run with environment variables or default settings
./bin/latinasub

# Dry-run mode (scrapes and tests locally without committing to Turso DB)
./bin/latinasub -dry-run

# Custom options
./bin/latinasub -concurrency 100 -max-nodes 500 -log-level info
```

---

## ⚙️ Configuration

Create a `.env` file or provide environment variables (see [`.env.example`](.env.example)):

| Variable | Description | Default |
| :--- | :--- | :--- |
| `APP_ENV` | Environment (`development`, `production`) | `development` |
| `LOG_LEVEL` | Log level (`debug`, `info`, `warn`, `error`) | `info` |
| `TURSO_DATABASE_URL` | Turso LibSQL connection URL | `""` |
| `TURSO_AUTH_TOKEN` | Turso authentication token | `""` |
| `BOT_TOKEN` | Telegram Bot token for notifications | `""` |
| `ADMIN_ID` | Telegram chat ID for admin reports | `0` |
| `MAX_NODES` | Maximum verified active nodes to collect | `500` |
| `CONCURRENCY_WORKERS`| Number of concurrent verification workers | `100` |
| `TEST_TIMEOUT_SECONDS`| Network probe timeout per test in seconds | `5` |
| `SUBLIST_PATH` | Path to subscription sources JSON | `./resources/sublist.json` |
| `BLACKLIST_PATH` | Path to dead account blacklist file | `./blacklist.txt` |
| `OUTPUT_DIR` | Output directory for subscription files | `./result` |
| `DRY_RUN` | Run without writing to remote database | `false` |

---

## 🧪 Testing

Run unit and integration test suite:

```bash
go test -v ./...
```

Run memory allocation and latency benchmarks:

```bash
go test -bench="BenchmarkPipeline" ./internal/service -benchmem
```

---

## 📄 License

This software is released under the [MIT License](https://github.com/LalatinaHub/License/blob/main/LICENSE).
