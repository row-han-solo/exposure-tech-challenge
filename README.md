# HAVS Exposure Tracking API

A Go HTTP API for tracking Hand and Arm Vibrating Syndrome (HAVS) equipment exposures. Employers log exposure events against employees, and the service maintains running totals so managers can rotate employees off high-risk equipment before HSE limits are breached.

## Running the API

**Single command:**

```bash
docker compose up --build
```

The API starts on **http://localhost:8080**. MongoDB data is persisted in a named Docker volume between restarts.

## Seed Data

Two users and two equipment items are loaded automatically on first startup:

### Users

| Name | ID |
|---|---|
| Alice Smith | `713be58e-0d79-4df2-a85c-9f44ca513a7d` |
| Bob Jones | `b2e8c3a1-5f7d-4a9e-8c2b-1d3e6f8a0b4c` |

### Equipment

| Name | Vibration Magnitude | ID |
|---|---|---|
| AirCat - Drill - 4337 | 2.1 m/s² | `2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49` |
| JCB - Hydraulic Breaker - CEJCBHM25 | 4.0 m/s² | `36603447-2f30-41b1-a908-526c0b6f1755` |

## API Endpoints

### Record an exposure
```
POST /exposure
Content-Type: application/json

{
  "equipment_id": "2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49",
  "user_id":      "713be58e-0d79-4df2-a85c-9f44ca513a7d",
  "duration":     60
}
```

### List all exposures
```
GET /exposure
```

### Get a specific exposure
```
GET /exposure/{exposureId}
```

### Get a user's exposure summary
```
GET /users/{userId}/exposure-summary
GET /users/{userId}/exposure-summary?starting_at=2025-01-01T00:00:00Z&ending_at=2025-01-31T23:59:59Z
```

Time parameters accept RFC3339 format. If omitted, all exposures for the user are included.

## Running Tests

```bash
go test ./...
```

Requires Go 1.23+. No running MongoDB needed for tests — all tests use in-memory stubs.

### Smoke Test (end-to-end)

A shell script that starts the service, calls every endpoint, and verifies each response:

```bash
# Start services, run all checks, leave them running
bash scripts/smoke_test.sh

# Start, run, then tear down
bash scripts/smoke_test.sh --cleanup

# If the service is already up
bash scripts/smoke_test.sh --no-start
```

Requires `docker`, `curl`, and `python3`. Exits non-zero if any assertion fails.

## Project Structure

```
├── main.go           entrypoint — wires concrete dependencies
├── data/             shared domain types (User, EquipmentItem, Exposure, ExposureSummary)
├── db/               db.Client interface + sentinel errors
│   └── mongo/        MongoDB implementation + seed
│   └── hybrid/       Production ready hybrid implementation Valeky + MongoDB
├── handler/          HTTP handlers (thin: validate + respond only)
├── exposure/         exposure business logic + Publisher interface
├── equipment/        equipment business logic
├── users/            user business logic
├── utils/            HAVS calculation functions (PartialExposureA8, PartialExposurePoints)
└── docs/             challenge brief, API spec, architecture plan
```

See [docs/ARCHITECTURE_PLAN.md](docs/ARCHITECTURE_PLAN.md) for full design documentation including database option analysis and event-driven architecture.

## Design Decisions

### Calculation Fix

The original calculation functions in the challenge brief contain an integer division bug:

```go
// Original (BUGGY): triggerTime / 60 truncates to 0 for durations < 60 minutes
triggerTime / 60

// Fixed: explicit float64 conversion preserves precision
float64(triggerTime) / 60.0
```

### Snapshot Semantics

User and equipment data are embedded by value into each Exposure record. This captures the state of those entities at the time of recording — if a piece of equipment's vibration magnitude is later updated, historical exposure records remain accurate for compliance audits.

### Exposure Summary Aggregation

Consistent with HSE guidance for multiple vibration sources in a workday:
- **Points**: additive — `Σ points_i`
- **A(8)**: root-sum-of-squares — `√(Σ a8_i²)`

### API Spec Deviation

The spec declares `GET /exposure/{exposureId}` with response code `201`. This is a spec error — the implementation correctly returns `200`.

### Event-Driven Architecture

The `exposure.Publisher` interface is wired with a `NoopPublisher` by default. The events below are published whenever the corresponding business conditions are met. Swapping in a real broker (SQS, Kafka) requires only a new `Publisher` implementation injected in `main.go`.

**Events published:**

| Event | Trigger |
|---|---|
| `exposure.recorded` | Any new exposure is created |
| `exposure.eav_reached` | User's daily A(8) ≥ 2.5 m/s² or daily points ≥ 100 (HSE Action Value) |
| `exposure.elv_reached` | User's daily A(8) ≥ 5.0 m/s² or daily points ≥ 400 (HSE Limit Value) |

**Events worth consuming:**

| Event | Source | Effect |
|---|---|---|
| `shift.started` | Workforce system | Begin daily exposure tracking window |
| `shift.ended` | Workforce system | Finalise and archive shift summary |
| `equipment.decommissioned` | Equipment management | Block new exposures for decommissioned equipment |
| `user.deactivated` | HR / identity | Block new exposures for inactive users |
| `rotation.approved` | Scheduling | Acknowledge rotation; optionally reset daily accumulator |

### Proposed Production Database: Hybrid Valkey + MongoDB

The active implementation uses **MongoDB only**, which is the right choice for the challenge scope. However, `db/hybrid` contains a complete, production-oriented implementation that layers **Valkey** (Redis-compatible, OSS) as a write-through cache over the MongoDB primary store.

The motivation is the hot path that runs on every `POST /exposure`: after recording an exposure, the service must immediately aggregate the user's full daily exposure to check whether the HSE Exposure Action or Limit Value has been crossed. With MongoDB only, this is a database query on every write. With the hybrid, it becomes a sub-millisecond Valkey sorted-set lookup for the same-day window.

Key design points:

- **Write-through, MongoDB-first** — MongoDB is always written before Valkey. Valkey failures are non-fatal and logged; the primary store is always the source of truth.
- **Per-user daily sorted set** — `exposures:daily:{userID}:{YYYY-MM-DD}` stores exposure IDs scored by Unix timestamp. `ZRANGEBYSCORE` filters by time window; individual exposure objects are reconstructed via a pipeline `GET`.
- **Ready-flag pattern** — a separate `:ready` sentinel key is only set after a full MongoDB warm. This prevents `GetExposuresByUser` from returning a partial result when the sorted set has been partially populated by `CreateExposure` calls before the first cache warm.
- **Entity caching** — users and equipment are cached by ID (1h TTL), avoiding repeated MongoDB lookups across multiple exposures in a single shift.
- **Zero caller changes** — `db/hybrid.Client` satisfies the same `db.Client` interface. Switching the active implementation requires only two lines in `main.go`; no handlers, logic packages, or tests change.

See [docs/ARCHITECTURE_PLAN.md](docs/ARCHITECTURE_PLAN.md) §5 for the full key schema, cold-start trace, and a comparison of all four database options.

### What I Would Do Given More Time

- **Equipment and user management endpoints** — the spec only covers exposures; adding CRUD for users and equipment would remove the need for seed data
- **Activate the hybrid** — add a `valkey/valkey:8-alpine` service to `docker-compose.yml` and wire `hybrid.New` in `main.go`
- **Event broker integration** — wire Kafka or SQS in the Docker Compose alongside MongoDB; the `Publisher` interface is already in place
- **Integration tests** — `testcontainers-go` suite that spins up real MongoDB and Valkey instances for the `db/mongo` and `db/hybrid` packages
- **Structured logging** — replace `log` with `slog` and add request IDs
- **Pagination**  — on `GET /exposure`
- **System Deployment**  — Write Terraformation code to deploy system into GCP
- **Monitoring** — Create Grafana/ELK stack to monitor system health and logs
