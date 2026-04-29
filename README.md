<div align="center">

# ⚡ AEGIS

### Unified Manufacturing Correlation Engine

*Fuse high-frequency PLC telemetry with live MES work orders —*  
*every bolt torqued, every firmware flashed, every VIN correlated in real time.*

[![Rust](https://img.shields.io/badge/Rust-edge--gateway-CE422B?logo=rust&logoColor=white)](edge-gateway/)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](correlation-worker/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.6-3178C6?logo=typescript&logoColor=white)](web/)
[![NATS](https://img.shields.io/badge/NATS-JetStream-27AAE1?logo=natsdotio&logoColor=white)](infra/)
[![ClickHouse](https://img.shields.io/badge/ClickHouse-OLAP-FFCC01?logo=clickhouse&logoColor=black)](infra/clickhouse/)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)](infra/docker-compose.yml)

</div>

---

## The Problem — The ISA-95 Pyramid Is a Data Silo

Traditional factory software passes data **sequentially up a rigid hierarchy**. By the time a torque anomaly at a robotic arm reaches an enterprise dashboard, it has lost its real-time link to the specific VIN being built or the firmware being flashed — making root-cause analysis a forensic nightmare.

```
┌──────────────────────────────────────────────────────┐
│  Level 5 · Enterprise / ERP                          │  ← days of latency
├──────────────────────────────────────────────────────┤
│  Level 4 · MES · Work Orders · Firmware Deployment   │  ← minutes of latency
├──────────────────────────────────────────────────────┤
│  Level 3 · SCADA / DCS          ◄── the silo wall    │
├──────────────────────────────────────────────────────┤
│  Level 2 · Supervisory Control                       │
├──────────────────────────────────────────────────────┤
│  Level 1 · PLC / Sensors         ← raw truth @ 10ms  │
└──────────────────────────────────────────────────────┘
```

**Aegis collapses this stack.** Both the Edge (Level 1-2) and the MES (Level 4) publish directly to a shared event stream. The correlation engine joins them in memory — producing enriched telemetry in milliseconds, not minutes.

---

## Architecture

```mermaid
flowchart LR
    subgraph EDGE["⚙️ Edge / Control  (Level 1-2)"]
        PLC("🔧 Mock PLC\nTorque · Position")
        EG("🦀 Rust\nEdge Gateway")
        PLC -->|"10 ms samples"| EG
    end

    subgraph BROKER["📨 NATS JetStream  ·  AEGIS stream"]
        RAW(["aegis.telemetry.raw"])
        STATE(["aegis.mes.state"])
    end

    subgraph MES_BOX["🏭 MES  (Level 4)"]
        MES("🐹 Go MES Service\n:8080")
        PG[("🐘 PostgreSQL\nwork_orders")]
        MES <-->|"CRUD"| PG
    end

    subgraph CORE["⚡ Correlation Core"]
        CW("🐹 Go Correlation Worker\n5 enricher goroutines")
        CACHE["RWMutex\nStation Cache"]
        CH[("🏛️ ClickHouse\nenriched_telemetry")]
        CW <-->|"station_id lookup"| CACHE
        CW -->|"500-row batches\n250 ms flush"| CH
    end

    subgraph DASH["📊 Dashboard"]
        UI("⚛️ React + TypeScript\nlocalhost:5173")
    end

    EG    -->|"aegis.telemetry.raw"| RAW
    MES   -->|"aegis.mes.state"| STATE
    RAW   -->|"JetStream consumer\naegis-correlation-plc"| CW
    STATE -->|"NATS Core subscribe"| CACHE
    UI    -->|"GET /api/v1/*\nVite proxy"| MES
```

---

## The Core Idea — Stream–Table Join

Every PLC reading is a point in time. Every MES state update is a slow-moving fact. Aegis performs a **stream–table join** in memory: the Go worker keeps a per-station `RWMutex` cache and stamps each 10 ms sample with the active work order before the data ever touches a database.

<table>
<tr>
<td><b>Before</b> — raw edge telemetry</td>
<td><b>After</b> — enriched record in ClickHouse</td>
</tr>
<tr>
<td>

```json
{
  "station_id": "5",
  "torque":     43.21,
  "timestamp":  1714234567890
}
```

</td>
<td>

```json
{
  "station_id": "5",
  "torque":     43.21,
  "timestamp":  1714234567890,
  "vin":        "1HGBH41JXMN100005",
  "firmware":   "v2.1.4",
  "ingested_at":"2025-04-28 09:22:47"
}
```

</td>
</tr>
</table>

Now you can ask:  
> *"Show me every torque reading > 50 Nm for all VINs built with firmware `v2.1.4` that later reported a steering defect."*

That's a single ClickHouse `SELECT`.

---

## Tech Stack

| Layer | Technology | Role |
|-------|-----------|------|
| Edge Gateway | **Rust** · `async-nats` · Tokio | Deterministic PLC mock at up to 1 kHz; graceful SIGINT shutdown |
| Message Broker | **NATS JetStream** | Durable at-least-once delivery; `AEGIS` stream on `aegis.>` |
| MES Service | **Go** · `pgx/v5` · `net/http` | Work order CRUD, NATS state publisher, mock sessions every 45 s |
| Relational Store | **PostgreSQL 16** | `work_orders` table with station index |
| Correlation Worker | **Go** · goroutines · `sync.RWMutex` | Stream–table join; 5 concurrent enrichers; batch writer |
| OLAP Store | **ClickHouse 24** | Partitioned by month · 90-day TTL · sub-second aggregation |
| Dashboard | **React 18 + TypeScript 5** · Vite | MES snapshot + work order table · 10 s auto-refresh |
| Infra | **Docker Compose** | NATS · PostgreSQL · ClickHouse — all with health checks |

---

## Data Flow (Step by Step)

```
① Edge gateway publishes 20 samples/sec
   → NATS subject: aegis.telemetry.raw
   → {"station_id":"5","torque":43.2,"timestamp":1714234567890}

② MES service creates a session (mock: every 45 s, or via POST /api/v1/sessions)
   → Inserts work_orders row in PostgreSQL
   → Publishes to NATS subject: aegis.mes.state
   → {"station_id":"5","vin":"1HGBH41JXMN100005","firmware":"v2.1.4"}

③ Correlation worker receives both streams
   → aegis.mes.state  → updates RWMutex station cache (slow writes)
   → aegis.telemetry.raw → 5 goroutines look up cache, enrich, push to out channel

④ ClickHouse sink drains out channel
   → Batches up to 500 rows, flushes every 250 ms
   → INSERT INTO aegis.enriched_telemetry ...

⑤ React dashboard polls MES REST API every 10 s
   → GET /api/v1/status   (work order count)
   → GET /api/v1/work-orders  (last 50 rows)
```

---

## Quick Start

### Prerequisites

| Tool | Version |
|------|---------|
| Docker + Compose | any recent |
| Go | 1.22+ |
| Rust toolchain | stable (`rustup`) |
| Node.js | 20+ |

### 1 — Start infrastructure

```bash
cd infra && docker compose up -d

# Wait for health checks (~15 s)
docker compose ps   # all services show "healthy"
```

### 2 — Run services  *(separate terminals)*

```bash
# Terminal 1 · MES HTTP API + mock sessions
cd mes-service && go run ./cmd/mes

# Terminal 2 · Correlation worker
cd correlation-worker && go run ./cmd/worker

# Terminal 3 · PLC mock → JetStream
cd edge-gateway && cargo run --release

# Terminal 4 · Dashboard  →  http://localhost:5173
cd web && npm install && npm run dev
```

**Or use the root Makefile:**

```bash
make infra-up
make mes           # T1
make correlation   # T2
make edge          # T3
make web           # T4
```

### Build & test everything

```bash
make build-all   # compiles all four services
make test-all    # runs Go tests with race detector
```

---

## HTTP API — MES Service (`:8080`)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Liveness — `{"status":"ok"}` |
| `GET` | `/api/v1/status` | Work order count — `{"service":"mes","work_orders":N}` |
| `GET` | `/api/v1/work-orders` | Last 50 rows (JSON array) |
| `POST` | `/api/v1/sessions` | Create session + publish MES state to NATS |

**Create a session manually:**

```bash
curl -X POST http://localhost:8080/api/v1/sessions \
  -H 'Content-Type: application/json' \
  -d '{"station_id":"5","vin":"1HGBH41JXMN100042","firmware":"v2.1.4"}'
```

---

## ClickHouse Queries

```sql
-- Last 20 enriched records
SELECT station_id, vin, firmware, round(torque, 2) AS torque_nm, ts
FROM aegis.enriched_telemetry
ORDER BY ts DESC
LIMIT 20;

-- Torque anomalies for a firmware version
SELECT vin, max(torque) AS peak_nm, count() AS samples
FROM aegis.enriched_telemetry
WHERE firmware = 'v2.1.4'
  AND torque > 50
GROUP BY vin
ORDER BY peak_nm DESC;

-- Per-station throughput in the last hour
SELECT station_id,
       count()       AS messages,
       avg(torque)   AS avg_torque_nm,
       max(torque)   AS max_torque_nm
FROM aegis.enriched_telemetry
WHERE ts > (toUnixTimestamp(now() - INTERVAL 1 HOUR)) * 1000
GROUP BY station_id;
```

---

## Environment Variables

| Variable | Default | Service |
|----------|---------|---------|
| `NATS_URL` | `nats://127.0.0.1:4222` | all |
| `DATABASE_URL` | `postgres://aegis:aegis@127.0.0.1:5432/aegis_mes?sslmode=disable` | mes-service |
| `CLICKHOUSE_ADDR` | `127.0.0.1:9000` | correlation-worker |
| `HTTP_PORT` | `8080` | mes-service |
| `EDGE_STATION_ID` | `5` | edge-gateway |
| `EDGE_HZ` | `20` | edge-gateway |
| `ENRICH_WORKERS` | `5` | correlation-worker |

---

## Repository Layout

```
aegis/
├── edge-gateway/               🦀 Rust — PLC mock → NATS JetStream
│   ├── Cargo.toml
│   └── src/main.rs             signal-aware publish loop
│
├── mes-service/                🐹 Go — MES HTTP API + work orders
│   ├── cmd/mes/main.go         graceful shutdown, mock sessions
│   └── internal/store/         PostgreSQL migrate + repository
│
├── correlation-worker/         🐹 Go — stream-table join → ClickHouse
│   ├── cmd/worker/main.go
│   └── internal/
│       ├── config/             env config loader
│       ├── models/             PLCMessage, EnrichedMessage
│       ├── processor/          StreamEnricher + unit tests
│       ├── sink/               ClickHouse batch writer
│       ├── state/              RWMutex station cache + concurrency tests
│       └── stream/             NATS JetStream consumer
│
├── web/                        ⚛️  React 18 + TypeScript 5 + Vite
│   └── src/App.tsx             MES status, work order table, auto-refresh
│
├── infra/
│   ├── docker-compose.yml      NATS · PostgreSQL · ClickHouse · Redpanda · MQTT
│   └── clickhouse/init.sql     MergeTree, PARTITION BY month, 90-day TTL
│
└── Makefile                    infra-up/down · mes · edge · correlation · web
                                build-all · test-all · clean
```

---

## Design Decisions

**Why NATS JetStream over Kafka?**  
JetStream gives durable, at-least-once delivery with far lower operational overhead for a single-cluster deployment. The `AEGIS` stream retains 24 hours, letting the worker replay on restart without data loss.

**Why ClickHouse over TimescaleDB?**  
Columnar storage gives sub-second aggregations over billions of rows. `PARTITION BY toYYYYMM(ts)` and a 90-day TTL keep storage bounded automatically.

**Why `sync.RWMutex` over a channel-based cache?**  
MES state is read on every 10 ms sample but written only every ~45 s. `RWMutex` lets all 5 enricher goroutines read concurrently with zero contention in the steady state.

---

<div align="center">

*Built to answer the question a quality engineer should never have to struggle to ask:*

**"What was happening to VIN `1HGBH41JXMN100042` — physically and in software — at 09:22:47?"**

</div>
