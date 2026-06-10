# HAVS Exposure Tracking API — Architecture & Design

## Context

Build a production-quality Go HTTP API for tracking Hand and Arm Vibrating Syndrome (HAVS) equipment exposures. The API allows employers to log exposure events against employees, manage equipment, and retrieve exposure summaries within time windows — enabling safe rotation decisions. The deliverable is runnable with a single `docker compose up` and includes CI via GitHub Actions.

---

## 1. Architecture Overview

### Package Structure

```
ctrl-hub-challenge/
├── .github/
│   └── workflows/
│       └── ci.yml               # lint → test → build pipeline
├── main.go                      # entrypoint: wire deps, start server
├── data/                        # shared domain types (DTOs, entities)
│   ├── user.go
│   ├── equipment.go
│   ├── exposure.go
│   └── requests.go
├── db/                          # DB interface + implementations
│   ├── client.go                # Client interface + sentinel errors
│   ├── mongo/                   # ✅ active — MongoDB implementation
│   │   ├── mongo.go             # MongoDB implementation of Client
│   │   └── seed.go              # seed equipment + users on startup
│   └── hybrid/                  # 📐 reference — Valkey write-through cache over MongoDB
│       ├── store.go             # Client struct, New(), TTL constants, key helpers
│       ├── users.go             # GetUser with Valkey read-through
│       ├── equipment.go         # GetEquipment, ListEquipment with Valkey read-through
│       └── exposures.go         # CreateExposure, GetExposure, ListExposures, GetExposuresByUser
├── handler/                     # HTTP layer (thin — validate + respond only)
│   ├── handler.go               # Handler struct{db db.Client}, New(), Routes()
│   ├── get_exposures.go
│   ├── record_exposure.go
│   ├── get_exposure.go
│   └── get_user_exposure_summary.go
├── exposure/                    # exposure business logic + event publishing
│   ├── exposure.go              # Record, Get, List, GetSummary, Publisher interface
│   ├── noop_publisher.go        # no-op Publisher for initial wiring
│   └── exposure_test.go
├── equipment/                   # equipment business logic
│   ├── equipment.go             # Get, List
│   └── equipment_test.go
├── users/                       # user business logic
│   ├── users.go                 # Get
│   └── users_test.go
├── utils/                       # shared exported pure utilities
│   ├── havs.go                  # PartialExposureA8, PartialExposurePoints
│   └── havs_test.go
├── docs/
│   ├── tech-challenge/          # original challenge brief + spec
│   └── ARCHITECTURE_PLAN.md    # this document
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── README.md
```

### Package Dependency Rules

```
main.go
  └─► handler ──► exposure ──► db.Client (interface)
      │           │              └── data
      │           ├── users
      │           ├── equipment
      │           └── utils
      └─► db/mongo (concrete implementation injected at main)
```

- `data` and `utils` import nothing from this module
- `db` imports only `data`
- Logic packages (`exposure`, `users`, `equipment`) import `db`, `data`, `utils`
- `handler` imports logic packages + `db` (for the embedded interface)
- `main.go` is the only place concrete implementations are wired (dependency injection root)

---

## 2. Entity Relationship Diagram

```
┌─────────────┐         ┌──────────────────────┐
│    User      │         │    EquipmentItem      │
├─────────────┤         ├──────────────────────┤
│ id: UUID    │         │ id: UUID             │
│ name: string│         │ name: string         │
└──────┬──────┘         │ vibration_magnitude  │
       │                │   : float64 (m/s²)   │
       │                └──────────┬───────────┘
       │ 1                         │ 1
       │        ┌──────────────────┘
       └────────▼──────────────────┐
                │     Exposure      │
                ├───────────────────┤
                │ id: UUID          │
                │ user: User        │ ← embedded snapshot
                │ equipment:        │ ← embedded snapshot
                │   EquipmentItem   │
                │ duration: int     │ (minutes)
                │ a8: float64       │ (partial A(8) m/s²)
                │ points: float64   │ (partial exposure pts)
                │ created_at: Time  │
                └───────────────────┘
                         │
                         │ aggregated into
                         ▼
              ┌───────────────────────┐
              │   ExposureSummary     │
              ├───────────────────────┤
              │ user: User            │
              │ a8: float64           │ √(Σ a8ᵢ²)
              │ points: float64       │ Σ pointsᵢ
              └───────────────────────┘
```

**Snapshot semantics:** User and EquipmentItem are embedded by value into Exposure records. An Exposure captures the state of those entities at recording time — consistent with audit and compliance requirements.

---

## 3. Request Flow Diagram

```
Client
  │  POST /exposure {equipment_id, user_id, duration}
  ▼
handler.RecordExposure
  │  1. decode + validate JSON body
  │  2. validate UUIDs, duration > 0
  ▼
exposure.Record(ctx, db, pub, req)
  │  3. equipment.Get(ctx, db, req.EquipmentID)  → EquipmentItem
  │  4. users.Get(ctx, db, req.UserID)           → User
  │  5. utils.PartialExposureA8(mag, duration)   → a8
  │  6. utils.PartialExposurePoints(mag, dur)    → points
  │  7. db.CreateExposure(ctx, &Exposure{...})   → saved record
  │  8. pub.Publish("exposure.recorded", ...)    → event emitted
  │  9. GetSummary → check EAV/ELV thresholds   → threshold events
  ▼
handler.RecordExposure
  │  10. write 201 + JSON body
  ▼
Client
```

---

## 4. Calculation Notes

The original functions from the challenge brief have an **integer division bug** — `triggerTime / 60` truncates to 0 for any input under 60 minutes. The corrected implementations use `float64` conversion:

```go
func PartialExposureA8(vibrationMagnitude float64, triggerTime int) float64 {
    return vibrationMagnitude * math.Sqrt((float64(triggerTime) / 60.0) / 8.0)
}

func PartialExposurePoints(vibrationMagnitude float64, triggerTime int) float64 {
    points := math.Pow((vibrationMagnitude / 2.5), 2) * ((float64(triggerTime) / 60.0) / 8.0 * 100)
    return math.Round(points)
}
```

**Summary aggregation (HSE multi-source method):**
- `points` = Σ(all partial points in window) — additive
- `a8` = √(Σ aᵢ²) — root-sum-of-squares across all exposures in window

**HSE thresholds:**
- EAV (Exposure Action Value): A(8) ≥ 2.5 m/s² or points ≥ 100 — employer must take action
- ELV (Exposure Limit Value): A(8) ≥ 5.0 m/s² or points ≥ 400 — must not exceed

---

## 5. Database Options

Four options were considered. **Option C (MongoDB) was chosen for implementation.**

### Option A — In-Memory (sync.Map)

| | |
|---|---|
| **Pros** | Zero setup; fastest; no extra Docker service; simplest CI |
| **Cons** | Lost on restart; no time-range query support; not production-ready |
| **Verdict** | Acceptable for a quick demo; inappropriate for compliance data that must be retained |

### Option B — Valkey (Redis-compatible OSS fork)

| | |
|---|---|
| **Pros** | Sub-millisecond reads; native key TTL perfect for 24h windows; ZADD/ZRANGE for time-series; lightweight Docker image |
| **Cons** | Flat key-value model is awkward for nested document queries; equipment/user data requires careful serialisation; volatile without AOF/RDB persistence config |
| **Verdict** | Excellent for the hot exposure layer (current day), weaker for historical audit and complex entity lookups |

### Option C — MongoDB ✅ Chosen

| | |
|---|---|
| **Pros** | Document model maps perfectly to nested entities (Exposure embeds User+Equipment); rich date-range aggregation; long-term audit trail; Ctrl Hub's primary database; easy Docker Compose setup; similar model to DynamoDB |
| **Cons** | Higher latency than Valkey; heavier Docker image; slightly over-engineered if data were truly ephemeral |
| **Verdict** | Best single-store choice — compliance data should persist long-term, and the document model is a natural fit. The `db.Client` interface means a future swap to a hybrid requires only a new implementation |

### Option D — Hybrid: Valkey (cache) + MongoDB (store) 📐 Implemented in `db/hybrid`

```
Write path:  exposure.Record → MongoDB (persist) → Valkey SET exposure:{id} + ZADD daily set
Read path:   GetExposuresByUser (single-day) → Valkey sorted set → pipeline GET → MongoDB fallback
```

| | |
|---|---|
| **Pros** | EAV/ELV threshold checks at sub-millisecond cache speed; MongoDB as durable audit trail; Valkey TTL automates daily window expiry; entity lookups (user, equipment) bypass MongoDB entirely after first read |
| **Cons** | Two Docker services; "ready flag" pattern needed to prevent serving incomplete sorted sets on cold cache; Valkey failures must be non-fatal (adds error-path complexity) |
| **Verdict** | The right architecture for production at scale. Implemented in `db/hybrid` for reference; not the active wiring |

#### Hybrid Key Schema

| Key | Type | TTL | Purpose |
|---|---|---|---|
| `user:{id}` | String (JSON) | 1h | Cache individual user records |
| `equipment:{id}` | String (JSON) | 1h | Cache individual equipment items |
| `equipment:all` | String (JSON array) | 5m | Cache full equipment list |
| `exposure:{id}` | String (JSON) | 24h | Cache individual exposure records |
| `exposures:daily:{userID}:{YYYY-MM-DD}` | Sorted Set (score=unix_ts, member=exposureID) | 25h | Per-user daily exposure window for fast aggregation |
| `exposures:daily:{userID}:{YYYY-MM-DD}:ready` | String sentinel | 25h | Signals the sorted set has been fully warmed from MongoDB |

#### Ready-Flag Pattern

The sorted set can be partially populated (only by `CreateExposure` calls since the last restart) before a full MongoDB warm has occurred. The `:ready` sentinel key is only set after `GetExposuresByUser` successfully fetches and caches all historical exposures for that day from MongoDB. This prevents `GetExposuresByUser` from returning a partial result when the cache is cold.

```
Cold start:
  CreateExposure(e1) → ZADD daily set (ready=0, incomplete)
  GetExposuresByUser → ready=0 → MongoDB → warmDailyCache → SET ready=1
  CreateExposure(e2) → ZADD daily set (ready=1, complete)
  GetExposuresByUser → ready=1 → ZRANGEBYSCORE → pipeline GET → [e1, e2] ✓
```

#### Switching to Hybrid

Because all callers program against the `db.Client` interface, switching from MongoDB-only to hybrid requires only a change in `main.go`:

```go
// current
client, _ := dbmongo.New(ctx, mongoURI)

// hybrid
mongoClient, _ := dbmongo.New(ctx, mongoURI)
client, _ := hybrid.New(ctx, mongoClient, valkeyAddr)
```

No handler, logic package, or test changes required.

---

## 6. Event-Driven Architecture

### Events to Publish

| Event | Trigger | Key Payload |
|---|---|---|
| `exposure.recorded` | POST /exposure succeeds | exposure_id, user_id, equipment_id, a8, points, recorded_at |
| `exposure.eav_warning` | Daily A(8) crosses 80% of EAV | user_id, current_a8, current_points, percentage_of_eav, date |
| `exposure.eav_reached` | Daily A(8) ≥ 2.5 m/s² **or** points ≥ 100 | user_id, current_a8, current_points, date |
| `exposure.elv_reached` | Daily A(8) ≥ 5.0 m/s² **or** points ≥ 400 | user_id, current_a8, current_points, date — **immediate action required** |
| `equipment.high_risk_flagged` | Equipment registered with vibration ≥ 5.0 m/s² | equipment_id, vibration_magnitude |

The EAV and ELV events are the most operationally important — they drive manager rotation decisions and satisfy the HSE compliance obligation.

### Events to Consume

| Event | Source System | Action in This Service |
|---|---|---|
| `shift.started` | Workforce / scheduling | Begin exposure tracking window for user; set TTL keys in Valkey if hybrid |
| `shift.ended` | Workforce / scheduling | Finalise shift summary; archive exposure snapshot |
| `equipment.created` / `equipment.updated` | Equipment management | Upsert equipment in local store |
| `equipment.decommissioned` | Equipment management | Block new exposure records for this equipment |
| `user.created` / `user.updated` | HR / identity | Upsert user record |
| `user.deactivated` | HR / identity | Block new exposure records for this user |
| `rotation.approved` | Scheduling | Log rotation acknowledgement; optionally reset daily accumulator |

### Implementation Strategy

A `Publisher` interface is defined in the `exposure` package. The current wiring uses `NoopPublisher` — a no-op stub. Swapping in a real broker (NATS JetStream, SQS, Kafka) requires only a new `Publisher` implementation injected at `main`:

```go
type Publisher interface {
    Publish(ctx context.Context, event string, payload any) error
}
```

**Recommended broker:** NATS JetStream — Go-native, lightweight Docker image, durable subscriptions, at-least-once and exactly-once delivery options. Kafka for very high throughput; SQS if already on AWS.

---

## 7. Handler Pattern

Handlers are strictly thin. They own request parsing, input validation, and response formation. All business logic lives in the relevant logic package.

```go
// handler/handler.go
type Handler struct {
    db  db.Client
    pub exposure.Publisher
}

func New(db db.Client, pub exposure.Publisher) *Handler {
    return &Handler{db: db, pub: pub}
}
```

Each endpoint is a method on `*Handler` in its own file:

```go
// handler/record_exposure.go
func (h *Handler) RecordExposure(w http.ResponseWriter, r *http.Request) {
    var req data.RecordExposureRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    if err := validateRecordExposureRequest(req); err != nil {
        writeError(w, http.StatusBadRequest, err.Error())
        return
    }
    result, err := exposure.Record(r.Context(), h.db, h.pub, req)
    if errors.Is(err, db.ErrNotFound) {
        writeError(w, http.StatusNotFound, "user or equipment not found")
        return
    }
    if err != nil {
        writeError(w, http.StatusInternalServerError, "internal server error")
        return
    }
    writeJSON(w, http.StatusCreated, result)
}
```

---

## 8. Testing Strategy

All tests are BDD-style, table-driven, with stubs defined inside `_test.go` files. No test code leaks into live code.

```go
func TestRecord(t *testing.T) {
    cases := []struct {
        name        string
        req         data.RecordExposureRequest
        setupDB     func(*stubDB)
        wantErr     error
        checkResult func(*testing.T, *data.Exposure)
    }{
        {
            name: "valid request calculates and stores exposure",
            // ...
        },
        {
            name:    "equipment not found returns ErrNotFound",
            wantErr: db.ErrNotFound,
        },
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) { ... })
    }
}
```

---

## 9. API Spec Notes

- `GET /exposure/{exposureId}` in the spec declares response code `201` — this is a spec error. The implementation correctly returns `200`.
- The spec has no user or equipment management endpoints. Two users and the two challenge-specified equipment items are seeded at startup. UUIDs are documented in README and printed to logs.
- `starting_at` / `ending_at` query params accept RFC3339 format (e.g. `2025-01-01T00:00:00Z`). If absent, no time filter is applied.

---

## 10. Infrastructure

### GitHub Actions CI

Three sequential stages triggered on every push and pull request:

1. **lint** — `golangci-lint` (covers `go vet`, `staticcheck`, `errcheck`, `gofmt`)
2. **test** — `go test -race ./...` with coverage output
3. **build** — `docker build .` (depends on lint + test passing)

### Docker

Multi-stage Dockerfile: `golang:1.23-alpine` build stage → `alpine:3.20` runtime image (~15 MB final image).

`docker-compose.yml` services: `mongodb` (mongo:7 with healthcheck) + `api` (built from Dockerfile, waits on MongoDB healthcheck).

To run the hybrid configuration, add a `valkey` service to `docker-compose.yml` (image: `valkey/valkey:8-alpine`) and update `main.go` to wire `hybrid.New` as described in §5 Option D.
