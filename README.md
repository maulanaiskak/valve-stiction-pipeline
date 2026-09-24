# valve-stiction-pipeline

V1 of a distributed IoT fault-detection pipeline: a synthetic valve simulator publishes over MQTT, a Go ingestion service windows the signal and forwards it over gRPC to a Python detection service, which reuses [valve-stiction-ml](https://github.com/maulanaiskak/valve-stiction-ml)'s classic detector (ellipse-fit + Kano) directly, persisting results to TimescaleDB for Grafana.

Full design rationale lives in [docs/V1_PLAN.md](docs/V1_PLAN.md).

## Architecture

```
[Simulator] --MQTT--> [Mosquitto broker]
    --> [Ingestion service (Go)] --window--> [Detection service (Python, gRPC)]
    --> [PostgreSQL/TimescaleDB] --> [Grafana]
```

## Run it

```bash
docker compose up --build
```

Then watch the ingestion service's detection output:

```bash
docker compose logs -f ingestion
```

Grafana: http://localhost:3000 (anonymous access enabled for local dev).

Toggle stiction injection via the simulator's `STICTION_ENABLED` env var in `docker-compose.yml` (default: `true`).

## Why Go, not Rust

Originally planned as Rust in the PRD. Switched before starting V1: Go is a refresh for the author rather than a from-zero language, which meaningfully de-risks "learning curve stalls V1" without giving up the original non-Java goal, and its goroutine/channel model is a natural fit for the subscribe → window → forward pattern here. See the PRD §7 for the full reasoning.

## Project structure

```
simulator/    Python -- synthetic PV/OP generator with a verified stiction toggle
ingestion/    Go -- MQTT subscribe, fixed-window buffering, gRPC client
detection/    Python -- gRPC server wrapping valve_stiction_ml.classic, TimescaleDB writes
proto/        shared gRPC contract (detection.proto)
db/           TimescaleDB schema
mosquitto/    broker config
docs/         V1_PLAN.md -- build decisions and what's verified vs. deferred
```
