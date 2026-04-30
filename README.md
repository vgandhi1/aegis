<div align="center">

# ⚡ AEGIS · FORESIGHT

### Unified Manufacturing Correlation Engine + Streaming ML Inference

*Fuse high-frequency PLC telemetry with live MES work orders —*  
*every bolt torqued, every firmware flashed, every VIN correlated in real time.*  
*Then predict the defect before the car leaves the station.*

[![Rust](https://img.shields.io/badge/Rust-edge--gateway-CE422B?logo=rust&logoColor=white)](edge-gateway/)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](correlation-worker/)
[![Python](https://img.shields.io/badge/Python-ML%20training-3776AB?logo=python&logoColor=white)](inference-worker/)
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
**Foresight** then scores every reading against an ONNX anomaly model and fires a `defect_alert` before the vehicle leaves the station.

---

## Architecture

```mermaid
flowchart LR
    subgraph EDGE["⚙️ Edge / Control  (Level 1-2)"]
        PLC("🔧 Mock PLC\nTorque · Position")
        EG("🦀 Rust\nEdge Gateway")
        PLC -->|"10 ms samples"| EG
    end

    subgraph BROKER["📨 NATS JetStream  ·  AEGIS stream  (aegis.>)"]
        RAW(["aegis.telemetry.raw"])
        STATE(["aegis.mes.state"])
        ALERTS(["aegis.defect.alerts"])
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
        CW -->|"500-row batches · 250 ms flush"| CH
    end

    subgraph FORESIGHT["🧠 Foresight  (MLOps)"]
        IW("🐹 Go Inference Worker\nhot-swap scorer")
        MODEL["ONNX Model\nz-score mock in dev"]
        IW <-->|"sync.RWMutex swap"| MODEL
    end

    subgraph GATEWAY["🌐 API Gateway  :8081"]
        GW("🐹 Go API Gateway")
        REST["REST\n/auth/token · /webhooks/mes"]
        GQL["GraphQL\n/graphql"]
        SSE["SSE\n/api/v1/alerts/stream"]
        GRPC["gRPC :9090\nAnomalyScorer"]
        GW --- REST
        GW --- GQL
        GW --- SSE
        GW --- GRPC
    end

    subgraph DASH["📊 Dashboard"]
        UI("⚛️ React + TypeScript\nlocalhost:5173")
    end

    EG     -->|"aegis.telemetry.raw"| RAW
    MES    -->|"aegis.mes.state"| STATE
    RAW    -->|"consumer: aegis-correlation-plc"| CW
    STATE  -->|"NATS Core subscribe"| CACHE
    RAW    -->|"consumer: aegis-inference-plc"| IW
    IW     -->|"score > threshold"| ALERTS
    GQL    -->|"ClickHouse HTTP :8123"| CH
    SSE    -->|"NATS Core subscribe"| ALERTS
    REST   -->|"aegis.mes.state"| STATE
    UI     -->|"GraphQL queries"| GQL
    UI     -->|"SSE live alerts"| SSE
    UI     -->|"GET /api/v1/*\nVite proxy"| MES
```

---

## The Core Idea — Stream–Table Join + Streaming Inference

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
  "firmware":   "v2.1.4"
}
```

</td>
</tr>
</table>

Foresight then scores every enriched reading. When the anomaly score exceeds the threshold, a `DefectAlert` is published to NATS and immediately streamed to the dashboard:

```json
{
  "station_id":        "5",
  "anomaly_score":     0.97,
  "trigger_torque_nm": 61.8,
  "timestamp":         1714234567890
}
```

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
| Correlation Worker | **Go** · goroutines · `sync.RWMutex` | Stream–table join; 5 concurrent enrichers; 500-row batch writer |
| OLAP Store | **ClickHouse 24** | Partitioned by month · 90-day TTL · sub-second aggregation |
| **Inference Worker** | **Go** · `sync.RWMutex` hot-swap | NATS consumer; z-score scorer (ONNX in production); defect alert publisher |
| **API Gateway** | **Go** · `graphql-go` · `net/http` | GraphQL ↔ ClickHouse; REST auth + MES webhook; SSE live alerts; gRPC scaffold |
| Dashboard | **React 18 + TypeScript 5** · Vite | MES snapshot + work order table · live anomaly alerts |
| Infra | **Docker Compose** | NATS · PostgreSQL · ClickHouse — all with health checks |

---

## Data Flow (Step by Step)

```
① Edge gateway publishes 20 samples/sec
   → NATS: aegis.telemetry.raw
   → {"station_id":"5","torque":43.2,"timestamp":1714234567890}

② MES service creates a session (mock: every 45 s, or via POST /api/v1/sessions)
   → Inserts work_orders row in PostgreSQL
   → Publishes to NATS: aegis.mes.state
   → {"station_id":"5","vin":"1HGBH41JXMN100005","firmware":"v2.1.4"}

③ Correlation worker receives both streams
   → aegis.mes.state  → updates RWMutex station cache (slow writes)
   → aegis.telemetry.raw → 5 goroutines look up cache, enrich, push to out channel

④ ClickHouse sink drains out channel
   → Batches up to 500 rows, flushes every 250 ms
   → INSERT INTO aegis.enriched_telemetry ...

⑤ Inference worker (Foresight) runs in parallel on the same NATS stream
   → Reads aegis.telemetry.raw (durable consumer: aegis-inference-plc)
   → Scores each torque reading against the in-memory scorer
   → If anomaly_score > 0.95 → publishes to aegis.defect.alerts (fail-open)

⑥ API Gateway exposes the data to clients
   → GraphQL /graphql → queries aegis.enriched_telemetry in ClickHouse
   → SSE /api/v1/alerts/stream → forwards aegis.defect.alerts to browser
   → REST /api/v1/webhooks/mes → ingests MES state from legacy Level 4 software

⑦ React dashboard
   → Polls MES REST API (GET /api/v1/work-orders) every 10 s
   → Queries GraphQL for per-station telemetry
   → Receives live defect alerts via SSE without polling
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

# Terminal 2 · Correlation worker (enrichment → ClickHouse)
cd correlation-worker && go run ./cmd/worker

# Terminal 3 · Foresight inference worker (anomaly scoring → defect alerts)
cd inference-worker && go run ./cmd/worker

# Terminal 4 · API Gateway (REST + GraphQL + SSE)
cd api-gateway && go run ./cmd/gateway

# Terminal 5 · PLC mock → JetStream
cd edge-gateway && cargo run --release

# Terminal 6 · Dashboard  →  http://localhost:5173
cd web && npm install && npm run dev
```

**Or use the root Makefile:**

```bash
make infra-up
make mes           # T1
make correlation   # T2
make inference     # T3
make gateway       # T4
make edge          # T5
make web           # T6
```

### Build & test everything

```bash
make build-all   # compiles all services
make test-all    # runs Go tests (with race detector on correlation-worker)
```

---

## HTTP API — MES Service (`:8080`)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Liveness — `{"status":"ok"}` |
| `GET` | `/api/v1/status` | Work order count — `{"service":"mes","work_orders":N}` |
| `GET` | `/api/v1/work-orders` | Last 50 rows (JSON array) |
| `POST` | `/api/v1/sessions` | Create session + publish MES state to NATS |

```bash
# Create a session manually
curl -X POST http://localhost:8080/api/v1/sessions \
  -H 'Content-Type: application/json' \
  -d '{"station_id":"5","vin":"1HGBH41JXMN100042","firmware":"v2.1.4"}'
```

---

## HTTP API — API Gateway (`:8081`)

| Method | Path | Protocol | Description |
|--------|------|----------|-------------|
| `GET` | `/health` | REST | Liveness check |
| `POST` | `/api/v1/auth/token` | REST | Issues an HS256-signed JWT |
| `POST` | `/api/v1/webhooks/mes` | REST | Ingests MES state from legacy software → NATS |
| `GET\|POST` | `/graphql` | GraphQL | Query `enriched_telemetry` from ClickHouse |
| `GET` | `/api/v1/alerts/stream` | SSE | Real-time `defect_alert` stream from NATS |
| `:9090` | — | gRPC | `AnomalyScorer.ScoreTelemetryBatch` (after `make proto`) |

```bash
# Get an auth token
curl -X POST http://localhost:8081/api/v1/auth/token \
  -H 'Content-Type: application/json' \
  -d '{"username":"operator","password":"aegis"}'

# Ingest a MES state change from a legacy webhook
curl -X POST http://localhost:8081/api/v1/webhooks/mes \
  -H 'Content-Type: application/json' \
  -d '{"station_id":"6","vin":"1HGBH41JXMN100099","firmware":"v2.1.5"}'

# Query recent telemetry via GraphQL
curl -X POST http://localhost:8081/graphql \
  -H 'Content-Type: application/json' \
  -d '{"query":"{ telemetry(stationId: \"5\", limit: 10) { vin torqueNm ts } }"}'

# Stream live defect alerts (keep-alive SSE)
curl -N http://localhost:8081/api/v1/alerts/stream
```

---

## gRPC — AnomalyScorer (`:9090`)

The Protobuf contract lives in `proto/inference.proto`. Generate Go stubs and build the gRPC-enabled gateway with:

```bash
# Install code generators (once)
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate stubs + build
cd api-gateway && make build-grpc
```

The service definition:

```protobuf
service AnomalyScorer {
  rpc ScoreTelemetryBatch (TelemetryBatch) returns (ScoreResponse) {}
}

message TelemetryBatch {
  string vin = 1;
  repeated float torque_readings = 2;
  repeated float temp_readings = 3;
}

message ScoreResponse {
  bool is_anomalous = 1;
  float confidence_score = 2;
  string error_context = 3;
}
```

---

## Foresight MLOps — Offline Training Pipeline

The inference worker runs in two modes:

**Online (always running):** consumes live PLC telemetry, scores each reading, publishes `defect_alert` events.

**Offline (nightly, Python):** a training job queries ClickHouse for historical correlated telemetry and confirmed defect labels, trains an anomaly model, and exports it to ONNX:

```python
# Pseudo-code for the offline training pipeline
df = clickhouse.query("SELECT * FROM aegis.enriched_telemetry WHERE ...")
model = IsolationForest().fit(df[["torque"]])
onnx_model = to_onnx(model, df[["torque"]].values[:1])
s3.upload(onnx_model, "s3://aegis-models/inference/latest.onnx")
```

The inference worker polls the S3 model registry every 5 minutes. When a new version is detected, it downloads the file and calls `scorer.Swap()` — replacing the in-memory model under a `sync.RWMutex` with **zero downtime**.

```
Model Registry (S3 / MLflow)
        │
        │  poll every 5 min
        ▼
Inference Worker
  ┌─────────────────────────────────┐
  │  scorer.Swap(newConfig)         │  ← write lock (microseconds)
  │  sync.RWMutex hot-swap          │
  │  scorer.Score(torqueReadings)   │  ← read lock (concurrent)
  └─────────────────────────────────┘
        │  score > 0.95
        ▼
  aegis.defect.alerts  →  SSE  →  Dashboard
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
| `ANOMALY_THRESHOLD` | `0.95` | inference-worker |
| `MODEL_POLL_SECONDS` | `300` | inference-worker |
| `HTTP_ADDR` | `:8081` | api-gateway |
| `GRPC_ADDR` | `:9090` | api-gateway |
| `CLICKHOUSE_HTTP` | `http://127.0.0.1:8123` | api-gateway |
| `JWT_SECRET` | `aegis-dev-secret` | api-gateway |

---

## Repository Layout

```
aegis/
├── proto/
│   └── inference.proto             Protobuf contract — gateway ↔ inference
│
├── edge-gateway/                   🦀 Rust — PLC mock → NATS JetStream
│   ├── Cargo.toml
│   └── src/main.rs                 signal-aware publish loop
│
├── mes-service/                    🐹 Go — MES HTTP API + work orders
│   ├── cmd/mes/main.go             graceful shutdown, mock sessions
│   └── internal/store/             PostgreSQL migrate + repository
│
├── correlation-worker/             🐹 Go — stream-table join → ClickHouse
│   ├── cmd/worker/main.go
│   └── internal/
│       ├── config/                 env config loader
│       ├── models/                 PLCMessage, EnrichedMessage
│       ├── processor/              StreamEnricher + unit tests
│       ├── sink/                   ClickHouse batch writer
│       ├── state/                  RWMutex station cache + concurrency tests
│       └── stream/                 NATS JetStream consumer
│
├── inference-worker/               🧠 Go — Foresight streaming ML inference
│   ├── cmd/worker/main.go          fail-open NATS consumer + model registry poll
│   └── internal/
│       ├── config/                 env config (threshold, poll interval)
│       ├── model/                  hot-swap scorer + tests (ONNX in production)
│       └── stream/                 NATS consumer · defect alert publisher
│
├── api-gateway/                    🌐 Go — polyglot API gateway
│   ├── cmd/gateway/main.go         HTTP :8081 (REST + GraphQL + SSE)
│   └── internal/
│       ├── config/                 env config (addr, JWT secret, ClickHouse)
│       ├── rest/                   auth token · MES webhook handler
│       ├── graphql/                schema · ClickHouse resolver · HTTP handler
│       ├── stream/                 SSE defect alert forwarder
│       ├── grpc/                   AnomalyScorer server (build tag: grpc)
│       └── pb/                     generated protobuf stubs (make proto)
│
├── web/                            ⚛️  React 18 + TypeScript 5 + Vite
│   └── src/App.tsx                 MES status, work order table, auto-refresh
│
├── infra/
│   ├── docker-compose.yml          NATS · PostgreSQL · ClickHouse · Redpanda · MQTT
│   └── clickhouse/init.sql         MergeTree, PARTITION BY month, 90-day TTL
│
└── Makefile                        infra-up/down · mes · edge · correlation
                                    inference · gateway · web
                                    build-all · test-all · clean
```

---

## Design Decisions

**Why NATS JetStream over Kafka?**  
JetStream gives durable, at-least-once delivery with far lower operational overhead for a single-cluster deployment. The `AEGIS` stream retains 24 hours, letting the worker replay on restart without data loss. Both the correlation worker and inference worker bind independent durable consumers to the same stream — they advance at their own pace with no coupling.

**Why ClickHouse over TimescaleDB?**  
Columnar storage gives sub-second aggregations over billions of rows. `PARTITION BY toYYYYMM(ts)` and a 90-day TTL keep storage bounded automatically.

**Why `sync.RWMutex` over a channel-based cache?**  
MES state is read on every 10 ms sample but written only every ~45 s. `RWMutex` lets all 5 enricher goroutines read concurrently with zero contention in the steady state. The same pattern is used in the inference worker for zero-downtime model hot-swapping.

**Why GraphQL for the UI and REST for webhooks?**  
Factory supervisors need varying levels of detail depending on the view — GraphQL prevents over-fetching. Legacy Level 4 MES systems speak JSON-over-HTTP, so REST is the right interface for webhook ingestion. Each protocol is scoped strictly to its domain strength.

**Why Server-Sent Events (SSE) over WebSockets for live alerts?**  
Defect alerts flow in one direction: server to browser. SSE is a first-class HTTP primitive (`text/event-stream`), requires no library, works through proxies, and reconnects automatically. WebSockets add bidirectional complexity that the alert use case simply doesn't need.

**Why gRPC for internal scoring?**  
When the gateway needs to request an anomaly score for 1,000 PLC metrics, passing a JSON array over HTTP wastes CPU on serialization. Protobuf binary encoding is significantly smaller and faster. The strict `.proto` contract also prevents runtime shape mismatches between services.

**Why fail-open in the inference worker?**  
The ML model is experimental. A scoring bug must never stop the assembly line. The worker acknowledges the NATS message *before* invoking the scorer — if scoring fails, it logs the error and moves on. The factory floor is always the higher priority.

---

<div align="center">

*Built to answer the question a quality engineer should never have to struggle to ask:*

**"What was happening to VIN `1HGBH41JXMN100042` — physically and in software — at 09:22:47?  
And did Foresight see it coming?"**

</div>
