# Aegis — Unified Manufacturing Correlation Engine

This repository implements the plan in `planning.md` and the system layout in `architecture.md`. Implementation details for the Go correlation worker follow `core-logic.md` (module layout and enrichment pattern).

## Repository layout

| Path | Role |
|------|------|
| `planning.md`, `architecture.md` | Authoritative scope and phases |
| `core-logic.md` | Reference: correlation worker package split and patterns |
| `infra/docker-compose.yml` | NATS (JetStream), PostgreSQL, ClickHouse, JetStream init |
| `infra/clickhouse/init.sql` | `aegis.enriched_telemetry` table |
| `edge-gateway/` | Rust mock edge: PLC-style JSON → JetStream `aegis.telemetry.raw` |
| `mes-service/` | Go mock MES: PostgreSQL work orders + HTTP API + NATS `aegis.mes.state` |
| `correlation-worker/` | Go: MES state cache (`RWMutex`), enrich PLC stream, batch insert to ClickHouse |
| `web/` | React + TypeScript dashboard (MES status + work orders via Vite dev proxy) |

## Data flow

1. **Edge** publishes high-frequency samples to JetStream subject `aegis.telemetry.raw` (JSON: `station_id`, `torque`, `timestamp`). The mock **edge-gateway** does this; alternatively **SentinelFlow ingestion** (`../SentinelFlow/services/ingestion`) can subscribe to MQTT, write raw bytes to Kafka when `KAFKA_BROKERS` is set, and publish the same PLC JSON to `aegis.telemetry.raw` when `NATS_URL` is set.
2. **MES** inserts work orders in Postgres and publishes slow state to `aegis.mes.state` (JSON: `station_id`, `vin`, `firmware`).
3. **Correlation worker** subscribes to MES (core NATS), consumes PLC from JetStream, joins in memory, writes to ClickHouse `aegis.enriched_telemetry`.

Local end-to-end with Sentinel: from repo root run `./run-iag-stack.sh`, then start MES, correlation-worker, and ingestion with `KAFKA_BROKERS` and `NATS_URL` as printed.

## Prerequisites

- Docker (for infra)
- Go 1.22+ (correlation worker, MES)
- Rust toolchain (edge gateway)
- Node 20+ (web UI)

## Run infrastructure

```bash
cd infra
docker compose up -d
```

This starts NATS (with JetStream), `nats-init` (creates stream **AEGIS** on `aegis.>`), PostgreSQL, and ClickHouse. Wait until `nats-init` exits successfully (check `docker compose logs nats-init`).

## Configuration (environment)

| Variable | Default | Service |
|----------|---------|---------|
| `NATS_URL` | `nats://127.0.0.1:4222` | all |
| `DATABASE_URL` | `postgres://aegis:aegis@127.0.0.1:5432/aegis_mes?sslmode=disable` | mes-service |
| `CLICKHOUSE_ADDR` | `127.0.0.1:9000` | correlation-worker |
| `HTTP_PORT` | `8080` | mes-service |
| `EDGE_STATION_ID` | `5` | edge-gateway |
| `EDGE_HZ` | `20` | edge-gateway (telemetry rate) |
| `ENRICH_WORKERS` | `5` | correlation-worker |

## Run applications (separate terminals)

Recommended order after infra is healthy:

1. **MES** — HTTP `:8080`, mock sessions every 45s, publishes NATS state  
   `cd mes-service && go run ./cmd/mes`

2. **Correlation worker** — consumes PLC + MES, writes ClickHouse  
   `cd correlation-worker && go run ./cmd/worker`

3. **Edge gateway** — mock PLC telemetry into JetStream  
   `cd edge-gateway && cargo run --release`

4. **Web** — `cd web && npm run dev` then open `http://localhost:5173` (proxies `/api` to MES).

## HTTP API (MES)

- `GET /health` — liveness  
- `GET /api/v1/status` — JSON `{ "service": "mes", "work_orders": N }`  
- `GET /api/v1/work-orders` — recent rows  
- `POST /api/v1/sessions` — body `{"station_id":"5","vin":"...","firmware":"v2.1.4"}` — inserts and publishes MES state  

## ClickHouse query example

```sql
SELECT station_id, vin, firmware, torque, ts
FROM aegis.enriched_telemetry
ORDER BY ts DESC
LIMIT 20;
```

## Build shortcuts

```bash
make -C correlation-worker build
make -C mes-service build
cargo build --release -p aegis-edge-gateway --manifest-path edge-gateway/Cargo.toml
npm run build --prefix web
```

## Notes

- GraphQL for the presentation layer is described in `architecture.md`; this scaffold uses REST for the MES API and a simple React UI.
- `core-logic.md` shows a single-process layout; this repo extends it with per-station cache keys, JetStream wiring, and ClickHouse batching.
