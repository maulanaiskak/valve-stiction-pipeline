# V1 — Single-sensor pipeline: build plan

Status: v0.3 (Grafana dashboard provisioned and verified, see §"What's scaffolded now vs. deferred") · Scope: PRD §4 V1 (FR-1 through FR-6). See the PRD (Notion) for full context — this doc only covers implementation decisions specific to building V1.

## Architecture (from PRD §6)

```
[Simulator] --MQTT--> [Mosquitto broker]
    --> [Ingestion service (Go)] --window--> [Detection service (Python, gRPC)]
    --> [PostgreSQL/TimescaleDB] --> [Grafana]
```

## Component decisions

- **Simulator (Python)**: not the thesis's `publish.py` (which replayed real CSVs) — FR-1 asks for a *synthetic* signal generator with a stiction-injection toggle. **Not** a closed-loop PI controller — tried that first and found it could self-oscillate from integral windup against the OP saturation limits even with a perfectly healthy valve, confounding the exact signal this project needs to stay clean. Instead OP is directly scripted as a triangle wave (mimicking a controller actively driving the valve), and PV follows it through the same stick-slip model `valve-stiction-ml`'s test fixtures (`make_stick_slip`) use. Verified against the real classic detector before wiring it in (`simulator/test_main.py`): stiction on → ellipse_index≈1.9/kano=True, stiction off → ≈0.1/False.
- **Ingestion service (Go)**: MQTT subscribe (`eclipse/paho.mqtt.golang`), buffers each sensor's PV/OP into fixed-size windows (100 samples, matching what `valve-stiction-ml` validated the classic detector against — see its ML_PLAN.md §13 on why window size isn't an arbitrary choice), sends each full window to the detection service over gRPC.
- **Detection service (Python, gRPC)**: reuses `valve-stiction-ml`'s `classic.py` directly — installed from its public GitHub repo (`requirements.txt`), not vendored/copied and not a local path dependency (which wouldn't survive a Docker build context). One implementation, reusable by both the training pipeline and this real-time service, exactly as `valve-stiction-ml`'s ML_PLAN.md §7 intended. Persists raw signal + detection result to TimescaleDB.
- **Storage**: TimescaleDB (Postgres + time-series extension), one hypertable (`window_results`) holding both the raw per-window PV/OP arrays and the detection result — `db/init.sql`.
- **Dashboard**: Grafana, TimescaleDB datasource and 3 panels (PV/OP signal, stiction label state-timeline, ellipse-index/kano-verdict trend) provisioned automatically on startup via `grafana/provisioning/` — no manual clicking required, satisfying FR-5 and the "one command" spirit of FR-6. The datasource needs a pinned `uid: TimescaleDB` in its provisioning YAML, not just a `name` — Grafana auto-generates a random UID otherwise, which silently breaks any dashboard panel that references the datasource by a fixed UID (found this by testing the panels via `/api/ds/query` after provisioning, not by assuming the JSON was correct once it loaded without error).
- **gRPC contract** (`proto/detection.proto`): `WindowRequest{sensor_id, pv[], op[], window_start_unix_ms}` → `WindowResponse{label, ellipse_index, kano_verdict, has_activity}` — mirrors the classic detector's output directly, no translation layer to keep in sync.
- **Generated gRPC stubs are committed** (`ingestion/detectionpb/`, `detection/detectionpb/`), not regenerated at Docker build time. FR-6 wants `docker compose up` to just work for a reviewer who hasn't installed `protoc` + plugins locally — regenerate manually (see below) only when `proto/detection.proto` changes.

## Regenerating gRPC stubs

Only needed after editing `proto/detection.proto`.

```bash
# Go stubs
PATH="$PATH:$(go env GOPATH)/bin" protoc \
  --go_out=ingestion/detectionpb --go_opt=paths=source_relative \
  --go-grpc_out=ingestion/detectionpb --go-grpc_opt=paths=source_relative \
  -I proto proto/detection.proto

# Python stubs (from a venv with grpcio-tools installed)
python -m grpc_tools.protoc -I proto \
  --python_out=detection/detectionpb --grpc_python_out=detection/detectionpb --pyi_out=detection/detectionpb \
  proto/detection.proto
# grpc_tools.protoc generates a flat `import detection_pb2`, which breaks as a
# package import -- fix it to a relative import after every regeneration:
#   sed -i '' 's/^import detection_pb2/from . import detection_pb2/' detection/detectionpb/detection_pb2_grpc.py
```

## What's scaffolded now vs. deferred

Verified with `docker compose up` — all 6 services (mosquitto, timescaledb, grafana, detection, ingestion, simulator) came up, and within ~10s the ingestion logs showed real detection results flowing through the whole chain (`[valve-1] label=yes ellipse_index=1.898 kano=true has_activity=true`), matching the simulator's own stiction=True verification. Confirmed persisted correctly in `window_results` via `docker compose exec timescaledb psql ...`. Grafana responded 200 on `/api/health`.

- ✅ Simulator generating synthetic PV/OP with a verified stiction toggle, publishing over MQTT
- ✅ Go ingestion service: MQTT subscribe, windowing, gRPC client
- ✅ Python detection service: gRPC server wrapping `valve_stiction_ml.classic`, TimescaleDB writes
- ✅ Docker Compose wiring Mosquitto + TimescaleDB + Grafana + all three services — `docker compose up` brings up the whole stack, satisfying FR-6
- ✅ End-to-end verified: simulator → MQTT → ingestion → gRPC → detection → TimescaleDB, with correct stiction labels
- ✅ Grafana dashboard: datasource + 3 panels auto-provisioned, each panel's SQL verified directly against `/api/ds/query` (status 200, real rows) — not just "the JSON loaded without error"
- ✅ `has_sufficient_activity`'s reference std, adapted for real-time: `RollingActivityReference` (`detection/main.py`) keeps a per-sensor EMA of OP std across windows, cold-starting as always-active for the first `MIN_WINDOWS_BEFORE_GUARD` windows until the EMA means something. Verified through the actual `DetectionServicer.DetectWindow` path (not just the isolated helper class) that an inactive window correctly returns `label="no", has_activity=False` after the reference is established, and that sustained normal-amplitude activity stays correctly classified as active (`detection/test_main.py`).
- ⏳ Window stride (currently non-overlapping, matching training exactly; sliding-window real-time responsiveness not yet explored)
- ⏳ Multiple concurrent sensors (FR-7, V2 scope) — ingestion's per-sensor buffering already supports this, not load-tested
