# valve-stiction-pipeline

A distributed IoT fault-detection pipeline, built in phases (V1 single-sensor, V2 multi-sensor/distributed, V3 custom dashboard). A synthetic valve simulator publishes over MQTT; a Go ingestion service windows the signal, calls a Python ML service that runs both [valve-stiction-ml](https://github.com/maulanaiskak/valve-stiction-ml)'s classic detector (ellipse-fit + Kano) and its trained RF model, and persists the result to TimescaleDB. A Go backend serves a REST + WebSocket API plus a React dashboard (status, animated sticky valve, PV-vs-OP phase plot, PV/OP time series); Grafana stays available for ad-hoc ops queries.

Full design rationale: [docs/V1_PLAN.md](docs/V1_PLAN.md), [docs/V2_PLAN.md](docs/V2_PLAN.md), [docs/V3_PLAN.md](docs/V3_PLAN.md).

## V1 — single sensor, gRPC

```
[Simulator] --MQTT--> [Mosquitto broker]
    --> [Ingestion service (Go)] --window--> [ML service (Python, gRPC): classic detector + RF]
    --> [PostgreSQL/TimescaleDB] --> [Grafana]
                                  --> [Backend (Go): REST + WebSocket] --> [React dashboard]
```

```bash
docker compose up --build
docker compose logs -f ingestion   # watch detection output
```

Dashboard: http://localhost:8080 — per-sensor status (classic + RF), an animated valve (smooth when healthy, stepped when sticking), a PV-vs-OP phase plot, and PV/OP-over-time charts, all pushed live over WebSocket. See [docs/V3_PLAN.md](docs/V3_PLAN.md) for why this exists alongside Grafana, not instead of it.

Grafana: http://localhost:3000 (anonymous access enabled for local dev) — datasource and dashboard ("Valve Stiction Detection": PV/OP signal, stiction label timeline, ellipse-index/kano-verdict trend) are auto-provisioned on startup. Kept for ad-hoc ops/debug queries against TimescaleDB.

Toggle stiction injection via the simulator's `STICTION_ENABLED` env var in `docker-compose.yml` (default: `true`). Three simulator instances run concurrently by default (`simulator`/`simulator-2`/`simulator-3`, different `sensor_id`s, mixed stiction settings) to demonstrate per-sensor isolation — see `docs/V1_PLAN.md` for the load-test results.

## V2 — multi-sensor, distributed via Redpanda

```
[Simulator: N valves] --MQTT--> [Mosquitto]
    --> [MQTT-Kafka bridge] --> [Redpanda, partitioned by sensor_id]
    --> [Detection service x N instances, consumer group] --> [TimescaleDB] --> [Grafana]
```

```bash
docker compose down                                                          # V1 and V2 share host ports, run one at a time
docker compose -f docker-compose.v2.yml up --build --scale detection-worker=3
docker compose -f docker-compose.v2.yml logs -f detection-worker
```

Same ingestion/detection code as V1 — `PUBLISH_MODE=kafka` switches ingestion to publish windows to Redpanda instead of calling detection directly, and `kafka_worker.py` is a second, thin transport over the same `detector.py` core `main.py`'s gRPC server uses. Verified: 3 `detection-worker` replicas in one consumer group each get assigned one of `valve-windows`' 3 partitions, and since `sensor_id` is the partition key, each replica ends up consistently handling one sensor — real horizontal scaling, not just multiple processes running the same thing. Grafana's dashboard adds an "all sensors" latest-status table and stiction-rate bar chart (FR-10) ahead of the existing per-sensor detail view. See `docs/V2_PLAN.md` for the full verification and the bugs found getting there (Redpanda topic auto-creation race, Python stdout buffering hiding all worker logs).

## Why Go, not Rust

Originally planned as Rust in the PRD. Switched before starting V1: Go is a refresh for the author rather than a from-zero language, which meaningfully de-risks "learning curve stalls V1" without giving up the original non-Java goal, and its goroutine/channel model is a natural fit for the subscribe → window → forward pattern here. See the PRD §7 for the full reasoning.

## Project structure

```
simulator/    Python -- synthetic PV/OP generator with a verified stiction toggle
ingestion/    Go -- MQTT subscribe, fixed-window buffering, gRPC client (V1) or Redpanda producer (V2),
              persists the result (V1) after the ML service responds
detection/    stateless ML/detection service -- no DB access (V3)
  detector.py    transport-agnostic core: classic detector + RF model (both V1 and V2 share this)
  persist.py     TimescaleDB writes -- only used by kafka_worker.py (V2 has no Go consumer downstream)
  main.py        gRPC server (V1)
  kafka_worker.py  Redpanda consumer (V2)
  model/        trained RF artifact (from valve-stiction-ml)
backend/      Go -- REST + WebSocket API, serves the built React app's static files (V3)
frontend/     React + TypeScript (Vite) -- the dashboard (V3)
proto/        shared gRPC contract (detection.proto)
db/           TimescaleDB schema
mosquitto/    broker config
docs/         V1_PLAN.md, V2_PLAN.md, V3_PLAN.md -- build decisions and what's verified vs. deferred
```
