# V2 — Multi-sensor, distributed: build plan

Status: Draft v0.1 · Scope: PRD §4 V2 (FR-7 through FR-10).

## Architecture (from PRD §6)

```
[Simulator: N valves] --MQTT--> [Mosquitto]
    --> [MQTT-Kafka bridge] --> [Kafka/Redpanda, partitioned by sensor_id]
    --> [Detection service x N instances, consumer group] --> [TimescaleDB] --> [Grafana]
```

## Key decision: V1 stays independently demoable

The PRD's own framing (§4): "each phase is independently 'done' and demoable." V2 must not break `docker compose up` giving exactly V1's working single-path demo. Rather than fork the codebase, the same binaries/images serve both:

- **Ingestion (Go)**: `PUBLISH_MODE` env var. `grpc` (default, unchanged V1 behavior) calls the detection service directly after windowing. `kafka` (V2) publishes each completed window to Redpanda instead, keyed by `sensor_id` for partitioning (FR-8) — the windowing logic itself (buffering, stride, per-sensor isolation) is identical in both modes, not duplicated.
- **Detection (Python)**: core detection logic (activity guard + classic detector + persistence) extracted into `detector.py`, shared by `main.py` (gRPC server, V1, unchanged) and `kafka_worker.py` (new, V2 — a Redpanda consumer). Running multiple `kafka_worker.py` instances in the same consumer group is what satisfies FR-9's horizontal scaling — Kafka/Redpanda's own partition assignment handles distributing sensor_id-partitioned work across instances, no extra code needed for that part.
- **Compose profiles**: `docker compose up` (no profile) = V1 exactly, unchanged. `docker compose --profile v2 up` additionally brings up Redpanda, switches ingestion to `kafka` mode, and runs N `detection-worker` replicas via `--scale` instead of the single gRPC `detection` service.

## Other decisions

- **Redpanda over Kafka** (PRD §11 open question, resolved): Kafka-API-compatible (same client library, same partitioning/consumer-group semantics as FR-8/FR-9 ask for), single-binary, lower overhead for local Compose dev.
- **Per-sensor MQTT topics** (FR-7, literal reading): simulator publishes to `valve/data/{sensor_id}` instead of V1's shared `valve/data` topic with sensor_id in the payload. Ingestion subscribes with a wildcard (`valve/data/+`). V1's load test already proved per-sensor isolation works with a shared topic + payload field; this change is about matching FR-7's literal architecture, not fixing a correctness gap.
- **Redpanda topic**: `valve-windows`, partitioned by `sensor_id` (matches FR-8 exactly). Message: same shape as the gRPC `WindowRequest` (sensor_id, pv[], op[], window_start_unix_ms), JSON-encoded (simpler than maintaining a second serialization path for the same proto over Kafka; revisit if a schema registry becomes worth it later).
- **Dashboard aggregation** (FR-10): add an "All sensors" table panel (latest label + timestamp per sensor_id) alongside V1's existing per-sensor detail view (already filterable via the `$sensor_id` template variable) — V1's dashboard already had the per-sensor piece, V2 adds the cross-sensor overview.

## What's built vs. verified vs. deferred

(filled in as V2 is built)
