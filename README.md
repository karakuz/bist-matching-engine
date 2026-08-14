# BIST Matching Engine

A backend-first limit order matching engine written in Go and inspired by the
price-time priority rules used in exchange order books.

## Project overview

The engine accepts BUY and SELL limit orders through a bounded per-symbol
submission queue. The HTTP path creates and persists the order before handing it
to one background worker, while matching for the symbol remains sequential and
deterministic even when HTTP requests arrive concurrently.

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

The HTTP API currently exposes limit-order submission and level-based order book
snapshots.

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
cmd/api/             application startup and dependency wiring
internal/app/        submission worker and application-level orchestration
internal/book/       in-memory price levels and snapshots
internal/domain/     orders, symbols, participants, trades, and events
internal/http/       Gin routes and request handling
internal/matching/   deterministic matching and staged match plans
internal/storage/    pgx repositories and transactional persistence
migrations/          PostgreSQL schema and development seed data
tests/integration/   end-to-end test area
```

## Current boundaries

The current version does not include market orders, cancel/modify operations,
authentication, a frontend, or external message brokers.

PostgreSQL and process memory cannot participate in one distributed atomic
transaction. The current ordering prevents memory changes when normal database
persistence fails. A process failure after the database commit but before
`Engine.Apply` requires the replay/recovery work planned for hardening.

The queue is currently in memory. Graceful shutdown drains accepted commands,
but ungraceful process failure can leave a persisted `PENDING` order unprocessed.
Reloading those pending orders is planned recovery work and is not implemented
yet.

The staged submission refactor also still needs the background persistence path
to update the already inserted incoming order. `PersistSubmission` currently
attempts to insert it again, so this integration must be completed before the
asynchronous submission path is considered finished.

## Roadmap

### V1 — Go backend (current)

- Complete the REST surface for orders, trades, market data, and health
- Polish error responses and operational configuration
- Add end-to-end API coverage and deployment documentation

### V2 — Frontend market board

- Display order book levels and recent trades
- Show best bid, best ask, spread, and last trade price
- Add an order book chart that compares bid and ask quantities across price
  levels
- Add a live trade table below the chart showing price, quantity, buyer, seller,
  and execution time
- Add a simple manual order-entry screen
- Introduce WebSocket or SSE updates after the market-data read model is defined

[![Order book chart visual reference](https://www.highcharts.com/demo/images/samples/stock/demo/orderbook-chart/thumbnail.webp)](https://www.highcharts.com/demo/stock/orderbook-chart)

*Visual reference: [Highcharts Order book chart](https://www.highcharts.com/demo/stock/orderbook-chart).*

The frontend will build this view from aggregated order book levels supplied by
the Go API. Highcharts is currently a visual reference; the charting library
will be selected during V2.

Planned live trade table:

![Planned live trade table showing price, quantity, buyer, seller, and execution time](docs/assets/trade-table-preview.png)

New executions will appear at the top of the table, with existing rows moving
downward. A short recording of the live animation will replace this static
preview after the V2 frontend is implemented.

### V3 — Java bot simulator

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

## Testing

First configure `BME_PG_CONNSTRING` using the platform-specific command above.
PostgreSQL-backed repository tests require the migrated local database.

All platforms:

```bash
go test -count=1 ./...
```

Focused packages:

```bash
go test -count=1 -v ./internal/matching
go test -count=1 -v ./internal/book
```
