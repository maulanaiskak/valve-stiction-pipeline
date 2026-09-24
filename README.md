# valve-stiction-pipeline

A distributed IoT fault-detection pipeline, built in phases (V1 single-sensor, V2 multi-sensor/distributed). A synthetic valve simulator publishes over MQTT; a Go ingestion service windows the signal and forwards it to a Python detection service, which reuses [valve-stiction-ml](https://github.com/maulanaiskak/valve-stiction-ml)'s classic detector (ellipse-fit + Kano) directly, persisting results to TimescaleDB for Grafana.

Full design rationale: [docs/V1_PLAN.md](docs/V1_PLAN.md), [docs/V2_PLAN.md](docs/V2_PLAN.md).

## V1 — single sensor, gRPC

```
[Simulator] --MQTT--> [Mosquitto broker]
    --> [Ingestion service (Go)] --window--> [Detection service (Python, gRPC)]
    --> [PostgreSQL/TimescaleDB] --> [Grafana]
```

```bash
docker compose up --build
docker compose logs -f ingestion   # watch detection output
```

Grafana: http://localhost:3000 (anonymous access enabled for local dev) — datasource and dashboard ("Valve Stiction Detection": PV/OP signal, stiction label timeline, ellipse-index/kano-verdict trend) are auto-provisioned on startup.

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

Same ingestion/detection code as V1 — `PUBLISH_MODE=kafka` switches ingestion to publish windows to Redpanda instead of calling detection directly, and `kafka_worker.py` is a second, thin transport over the same `detector.py` core `main.py`'s gRPC server uses. Verified: 3 `detection-worker` replicas in one consumer group each get assigned one of `valve-windows`' 3 partitions, and since `sensor_id` is the partition key, each replica ends up consistently handling one sensor — real horizontal scaling, not just multiple processes running the same thing. See `docs/V2_PLAN.md` for the full verification and the bugs found getting there (Redpanda topic auto-creation race, Python stdout buffering hiding all worker logs).

## Why Go, not Rust

Originally planned as Rust in the PRD. Switched before starting V1: Go is a refresh for the author rather than a from-zero language, which meaningfully de-risks "learning curve stalls V1" without giving up the original non-Java goal, and its goroutine/channel model is a natural fit for the subscribe → window → forward pattern here. See the PRD §7 for the full reasoning.

## Project structure

```
simulator/    Python -- synthetic PV/OP generator with a verified stiction toggle
ingestion/    Go -- MQTT subscribe, fixed-window buffering, gRPC client (V1) or Redpanda producer (V2)
detection/
  detector.py    transport-agnostic detection core (both V1 and V2 share this)
  main.py        gRPC server (V1)
  kafka_worker.py  Redpanda consumer (V2)
proto/        shared gRPC contract (detection.proto)
db/           TimescaleDB schema
mosquitto/    broker config
docs/         V1_PLAN.md, V2_PLAN.md -- build decisions and what's verified vs. deferred
```
