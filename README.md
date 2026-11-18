# BIST Trading Platform

A multi-service trading simulation platform built around a deterministic Go
matching engine, PostgreSQL persistence, and a live React trading screen. The
project focuses on the engineering behind exchange-style order flow: price-time
priority, serialized matching, transactional results, bounded queues, and
observable performance under concurrent submissions.

## Project overview

The engine accepts BUY and SELL limit orders through a bounded per-symbol
submission queue. The HTTP path creates and persists the order before handing it
to one background worker, while matching for the symbol remains sequential and
deterministic even when HTTP requests arrive concurrently.

The repository is a monorepo. The Go matching-engine API and the first React
trading screen are implemented. The Java bot simulator is the next planned
service and will act as an external API client rather than owning matching
logic.

The core backend currently supports:

- Price priority followed by FIFO time priority
- Full fills, partial fills, and matches across multiple resting orders
- Execution at the resting order's price
- Integer-based prices and symbol tick-size validation
- Opening-price-based daily price limits
- Per-symbol, per-session order sequencing assigned before enqueue
- Bounded worker queues for overload protection
- Explicit `CREATED` to `PENDING` or `REJECTED` submission states
- Transactional persistence of orders, trades, resting-order updates, and events
- Staged in-memory mutation after successful database persistence
- Aggregated, consistently read order book snapshots
- Startup reconstruction from resting PostgreSQL orders

The HTTP API exposes limit-order submission, symbol summaries, and level-based
order book snapshots. The React screen displays market summary and depth, accepts
limit orders, and streams market depth, last trade price, and live executions
over WebSocket. The trade list is session-only; persisted trade-history queries
are not yet exposed by the API.

### Engineering highlights

- Deterministic price-time matching behind a serialized per-symbol worker
- Transactional PostgreSQL persistence, with `pgx.Batch` for matching results
- Bounded asynchronous WebSocket delivery that keeps socket writes off the
  matching worker
- Measured local-run median queue wait reduced from **4.01 s to 65 ms** after
  batching database writes; detailed measurements and caveats follow below

## Core design

### Deterministic matching

The matching engine implements price-time priority: the best available price is
matched first, and resting orders at the same price retain FIFO order. Trade
prices come from the resting order. The behavior covers incoming and resting
partial fills as well as execution across multiple price levels.

### Go concurrency and backpressure

Each symbol has one `OrderWorker` and one bounded Go channel. Concurrent calls to
`Submit` are serialized while the next session sequence is assigned, the
`CREATED` order is inserted, and the enqueue result is recorded. A successful
send changes the persisted status to `PENDING`; a full queue changes it to
`REJECTED` and returns `503` instead of allowing unbounded memory growth.

The queued command contains a readiness signal. The worker does not start
matching until the HTTP submission path has finished saving `PENDING`. The
worker then processes commands one at a time, preventing concurrent mutation of
the same order book.

WebSocket delivery is isolated from matching. Each client has a bounded
outbound buffer and a dedicated writer goroutine; the worker enqueues market
updates without performing socket writes itself. A client whose outbound buffer
fills is disconnected rather than allowed to block the matching worker.

### Staged state changes

`Engine.Prepare` calculates the final incoming order, affected resting-order
copies, and trades without modifying the live book. After PostgreSQL commits the
complete submission, `Engine.Apply` updates the in-memory state and last trade
price.

### Transactional persistence

Persistence currently occurs in two stages. Before enqueueing, `Submit` inserts
the incoming order as `CREATED`, then records the enqueue result as `PENDING` or
`REJECTED`. After a `PENDING` command reaches the worker, one pgx transaction is
responsible for the matching result:

- The final incoming order
- Every affected resting-order update
- All generated trades
- Associated order events

An ordinary persistence failure rolls the entire database submission back and
leaves the live in-memory book unchanged.

### Consistent snapshots

Matching application and order book snapshots share an `RWMutex` boundary. A
snapshot therefore observes a completed state before or after an application,
never a partially mutated book.

### Package boundaries

The matching package does not access PostgreSQL. It produces a match plan while
the application and storage packages own sequencing and persistence. This keeps
the matching algorithm deterministic and independently testable.

## Performance engineering

The order path is instrumented by order ID and sequence. Logs separate submit
lock wait, queue wait, matching preparation, persistence, apply, and WebSocket
fanout. Persistence logs also separate transaction begin, the pgx batch, and
commit. This made it possible to distinguish worker backlog from database work
and socket delivery instead of attributing delays to CPU without evidence.

The first measured bottleneck was per-row database execution. A heavily matched
order with 27 trades spent about 152 ms in `PersistSubmission`: 28 order updates
took 48 ms, 27 trade inserts took 35 ms, and 56 event inserts took 65 ms; commit
took under 2 ms. The transaction was changed to queue the order updates, trades,
and events through `pgx.Batch`, while retaining one transaction and checking
every order update result before commit.

| Local ASELS load log | Before batching | After batching |
| --- | ---: | ---: |
| Worker queue wait, median | 4.01 s | 65 ms |
| Worker queue wait, p95 | 5.21 s | 192 ms |
| Worker queue wait, maximum | 5.28 s | 209 ms |
| `PersistSubmission`, median | 8.76 ms | 4.39 ms |
| `PersistSubmission`, p95 | 37.64 ms | 28.34 ms |
| `PersistSubmission`, maximum | 152.31 ms | 50.89 ms |
| Peak observed queue length | 335 | 11 |

The after-batching run recorded a median batch duration of 1.16 ms and a p95 of
2.12 ms. These are diagnostic runs, not a controlled benchmark: the log files do
not record machine specifications, PostgreSQL configuration, or the exact load
script arguments. Treat the comparison as evidence of improvement in these
runs, not a guaranteed throughput claim.

One remaining measured cost is submission contention before enqueueing. In the
latest run, waiting for the per-symbol submission mutex was 64 ms at the median
and 142 ms at p95. That lock currently protects sequence assignment and the
initial database and enqueue steps. It is a separate delay from worker queue
wait and should be evaluated without weakening order sequencing.

## Submission architecture

```mermaid
flowchart TD
    A[POST /orders] --> B[Gin HTTP handler]
    B --> C[Validate and create order]
    C --> D[Assign next session sequence]
    D --> E[Insert order as CREATED]
    E --> F{Bounded queue send}
    F -->|Full| G[Persist REJECTED and return 503]
    F -->|Queued| H[Persist PENDING]
    H --> I[Signal command ready]
    I --> J[Return HTTP 201]
    I --> K[Single per-symbol worker]
    K --> L[Engine.Prepare]
    L --> M[PersistSubmission transaction]
    M -->|Rollback| N[Log background failure; live book unchanged]
    M -->|Commit| O[Engine.Apply]
    O --> P[Update live book and last trade price]
```

The `201` response confirms that the order was created, successfully queued, and
saved as `PENDING`; it does not contain the final matching result. The command is
processed completely before the worker begins the next command for that symbol.

## Technology

- Go 1.25
- Gin HTTP framework
- PostgreSQL 16
- pgx v5
- Docker Compose
- Standard Go testing package

Prices and quantities use integers. For example, `350.50 TL` can be represented
as `35050`, avoiding floating-point arithmetic in matching and persistence.

## Project structure

```text
matching-engine-go/       Go matching engine, HTTP API, WebSocket feed, and PostgreSQL storage
  cmd/api/                application startup and dependency wiring
  internal/               domain, book, matching, app, HTTP, and storage packages
  migrations/             PostgreSQL schema and development seed data
  tests/integration/      end-to-end test area
frontend/                 user-facing web application
bot-simulator-java/       planned Spring Boot market-participant simulator
docker-compose.yml        shared local infrastructure
```

## Current boundaries

The current version does not include market orders, cancel/modify operations,
authentication, persistent user identity, or external message brokers. The
React application currently supports live market views and demo limit-order
entry; user-specific order ownership and authentication remain later work. The
Java bot simulator is planned and does not own matching or persistence.

PostgreSQL and process memory cannot participate in one distributed atomic
transaction. The current ordering prevents memory changes when normal database
persistence fails. A process failure after the database commit but before
`Engine.Apply` requires the replay/recovery work planned for hardening.

The queue is currently in memory. Graceful shutdown drains accepted commands,
but ungraceful process failure can leave a persisted `PENDING` order unprocessed.
Reloading those pending orders is planned recovery work and is not implemented
yet.

## Roadmap

### V1 — Go backend (core implemented; hardening ongoing)

- Price-time matching, tick validation, bounded per-symbol worker queue, and
  PostgreSQL persistence are implemented
- HTTP order submission, symbol summary, order book snapshots, and WebSocket
  market updates are implemented
- Continue expanding end-to-end coverage, operational configuration, and
  recovery behavior

### V2 — React trading application (base screen implemented)

- Display market summary, aggregated order book levels, and live executions
- Submit demo BUY and SELL limit orders through the Go API
- Consume the WebSocket feed for market depth, last price, and trades
- Add user identity, authorization, ownership, and user-specific order state in
  the later identity phase

[![Order book chart visual reference](https://www.highcharts.com/demo/images/samples/stock/demo/orderbook-chart/thumbnail.webp)](https://www.highcharts.com/demo/stock/orderbook-chart)

*Visual reference: [Highcharts Order book chart](https://www.highcharts.com/demo/stock/orderbook-chart).*

The frontend will build this view from aggregated order book levels supplied by
the Go API. Highcharts is currently a visual reference; the charting library
will be selected during V2.

Live trade table design reference:

![Planned live trade table showing price, quantity, buyer, seller, and execution time](docs/assets/trade-table-preview.png)

The current React screen displays live executions from the WebSocket feed.
Persisted trade-history queries and the referenced order book chart remain
future enhancements.

### V3 — Java bot simulator (planned)

- Build a separate Spring Boot service that submits orders through the Go API
- Support configurable bot count, order rate, symbols, prices, and quantities
- Add random and strategy-based order generation
- Generate sustained market activity and overload scenarios

Matching remains owned by the Go service. The Java application acts as an
external market participant and load generator.

### V4 — Production-minded hardening

- Add idempotency keys for safe client retries
- Rebuild or replay in-memory state across process/database failure boundaries
- Expand race, concurrency, and load testing
- Add structured logging, metrics, tracing, and graceful shutdown
- Generalize lifecycle management for more symbols and market sessions
- Evaluate a durable broker if asynchronous or multi-process workflows require
  one

## Running locally

### Requirements

- Go 1.25 or later
- Docker with Docker Compose
- PowerShell on Windows, or Bash on Linux/macOS

### 1. Start PostgreSQL and run migrations

Windows PowerShell:

```powershell
.\migrate.ps1
```

Linux or macOS:

```bash
bash ./migrate.sh
```

The scripts start PostgreSQL, apply the SQL files in filename order, and stop on
the first migration error. The migrations create the schema and seed symbols,
market sessions, participants, and an initial ASELS order book.

### 2. Configure the database connection

Windows PowerShell:

```powershell
$env:BME_PG_CONNSTRING = "postgres://pte:pte@localhost:5432/pte?sslmode=disable"
```

Linux or macOS:

```bash
export BME_PG_CONNSTRING="postgres://pte:pte@localhost:5432/pte?sslmode=disable"
```

### 3. Start the API

All platforms:

```bash
cd matching-engine-go
go run ./cmd/api
```

The server listens on `http://localhost:8080`.

## API examples

### Submit an order

```http
POST /orders
Content-Type: application/json
```

```json
{
  "participantId": 1,
  "symbol": "ASELS",
  "side": "BUY",
  "price": 35100,
  "quantity": 100
}
```

Windows PowerShell:

```powershell
curl.exe -X POST http://localhost:8080/orders `
  -H "Content-Type: application/json" `
  -d '{"participantId":1,"symbol":"ASELS","side":"BUY","price":35100,"quantity":100}'
```

Linux or macOS:

```bash
curl -X POST http://localhost:8080/orders \
  -H 'Content-Type: application/json' \
  -d '{"participantId":1,"symbol":"ASELS","side":"BUY","price":35100,"quantity":100}'
```

A successful enqueue currently returns `201 Created`:

```json
{
  "message": "Order Created"
}
```

This response confirms creation and enqueueing, not final matching. Matching and
final persistence continue in the per-symbol worker. A full worker queue returns
`503 Service Unavailable` after the order is recorded as `REJECTED`.

### Read an order book snapshot

```http
GET /engine/ASELS/snapshot/5
```

Example response:

```json
{
  "symbol": "ASELS",
  "buy": [
    { "price": 34950, "quantity": 125 }
  ],
  "sell": [
    { "price": 35050, "quantity": 100 }
  ]
}
```

The URL controls the maximum number of levels returned for each side and accepts
values from 1 through 100. BUY levels are ordered highest-first, SELL levels are
ordered lowest-first, and quantities are aggregated at each price.

### Subscribe to market updates

Connect a WebSocket client to:

```text
ws://localhost:8080/ws
```

Subscribe by sending one message for each required topic:

```json
{"action":"subscribe","symbol":"ASELS","topic":"MARKET_DEPTH"}
```

Supported topics are:

- `MARKET_DEPTH` — full level-based order book snapshot after every successfully
  processed order
- `CURRENT_PRICE` — latest trade price after an order produces a trade
- `TRADES` — all trades produced by the processed order

To stop receiving a topic, send the same message with `"action":"unsubscribe"`.
The server publishes the JSON payload for each subscribed topic after database
persistence and the corresponding in-memory match plan have both succeeded.

## Testing

First configure `BME_PG_CONNSTRING` using the platform-specific command above.
PostgreSQL-backed repository tests require the migrated local database.

All platforms:

```bash
cd matching-engine-go
go test -count=1 ./...
```

### Coverage

Last recorded core statement coverage snapshot (2026-08-29; not rerun for this
README update):

- matching engine: 80.0%
- domain model: 87.2%

Tests focus on price-time priority, full and partial fills, multi-order
matching, staged persistence, worker sequencing, failure handling, and
WebSocket fanout.

Focused packages:

```bash
cd matching-engine-go
go test -count=1 -v ./internal/matching
go test -count=1 -v ./internal/book
```
