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
- Orders inserted as `PENDING`, with `REJECTED` recorded when the queue is full
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
order is inserted as `PENDING`, and the order is enqueued. A successful send
needs no additional status update; a full queue changes the persisted status to
`REJECTED` and returns `503` instead of allowing unbounded memory growth.

Only orders whose inserts have succeeded are enqueued, so the queued command
needs no readiness signal. The worker processes commands one at a time,
preventing concurrent mutation of the same order book.

WebSocket delivery is isolated from matching. Each client has a bounded
outbound buffer and a dedicated writer goroutine; the worker enqueues market
updates without performing socket writes itself. A client whose outbound buffer
fills is disconnected rather than allowed to block the matching worker.
Each writer combines messages already waiting in its queue into one WebSocket
message containing a JSON array, preserving FIFO order and the individual topic
payloads. A single pending message keeps its original JSON-object format. Clients
must accept both formats; the frontend dispatches array entries in order.

### Staged state changes

`Engine.Prepare` calculates the final incoming order, affected resting-order
copies, and trades without modifying the live book. After PostgreSQL commits the
complete submission, `Engine.Apply` updates the in-memory state and last trade
price.

If `Engine.Apply` returns an error after the database commit, the worker preserves
the committed order/trade records, publishes no market updates for that command,
and stops processing and accepting orders for that symbol. Commands already queued
remain unprocessed as `PENDING`; automatic recovery is not implemented. This path
is distinguished by `ErrApplyCommittedPlan` and does not overwrite the incoming
order with `FAILED` and its original quantity. `Stop` releases the submission lock
before waiting for worker termination, including when shutdown overlaps this path.

### Transactional persistence

Persistence currently occurs in two stages. Before enqueueing, `Submit` inserts
the incoming order as `PENDING`. A full queue requires a separate `REJECTED`
update; successful enqueueing requires no further write. After a command reaches
the worker, one pgx transaction is
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

The order path is instrumented by order ID and sequence. Detailed diagnostic
logs originally separated submission lock wait, queue wait, matching preparation,
persistence, apply, and WebSocket fanout, including transaction begin, batch, and
commit. Normal timing output now retains two application records per successful
order: total submission time, then queue wait and worker processing time together.
Errors and Gin request logs remain enabled. Removing the detailed synchronous
logging reduces measurement overhead on the submission and worker paths.

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

An earlier run measured submission-mutex wait at 64 ms median and 142 ms p95.
The lock still protects sequence assignment, initial persistence, and enqueueing.
Successful submissions now insert directly as `PENDING`, removing the second
status write and the worker readiness handshake. Queue-full rejection still
requires a separate status update.

### Database and logging comparison — 2026-11-21

From the repository root, the reduced-log comparison runs used:

```powershell
.\matching-engine-go\tests\asels.book.tester.ps1 -DurationSeconds 10 -Workers 8
```

Each of the eight workers submits requests sequentially for ten seconds. Prices
and quantities are randomized, and the workers are not synchronized at startup.
Only log entries appended during this run were analyzed. All **5,618** requests
returned `202`, and every submitted order had a matching worker-completion record;
none remained unfinished in the captured log. Processing ran from approximately
21:30:30 through 21:30:49, including draining the queue after submissions stopped.

| Measurement | Earlier verbose-log run | Previous reduced-log run | Latest reduced-log run |
| --- | ---: | ---: | ---: |
| Submitted and processed orders | 881 | 4,831 | 5,618 |
| Total submission, median | 88.26 ms | 12.57 ms | 6.73 ms |
| Total submission, p95 | 115.78 ms | 14.86 ms | 13.69 ms |
| Total submission, maximum | 191.51 ms | 21.33 ms | 36.11 ms |
| Worker processing, median | 9.49 ms | 3.64 ms | 3.22 ms |
| Worker processing, p95 | 37.09 ms | 5.23 ms | 4.38 ms |
| Worker processing, maximum | 119.92 ms | 50.62 ms | 47.65 ms |
| Worker queue wait, median | 3.80 s | 4.67 s | 4.93 s |
| Worker queue wait, p95 | 5.21 s | 8.04 s | 7.53 s |
| Worker queue wait, maximum | 5.28 s | 8.37 s | 7.80 s |

Median worker processing was approximately **66% lower** than in the verbose-log
run. Faster submission also allowed substantially more orders into the queue;
queue latency did not improve at the median because arrivals still outpaced the
single symbol worker during the burst. Worker processing includes matching,
persistence, apply, and fanout enqueueing, but excludes queue wait and asynchronous
socket writes. Total submission includes validation, lock wait, initial persistence,
and enqueueing. Percentiles use the nearest-rank method.

These are local diagnostic comparisons, not controlled benchmark results.
Randomized match counts, existing book state, machine load, and database conditions
were not held constant. The reduced-log runs used the same explicit script
arguments above; the verbose-log run's arguments were not captured in its log.
The comparison supports logging overhead as a contributor, but does not isolate
its effect or establish a guaranteed throughput increase.

The 5,618-order run also recorded **two WebSocket outbound-buffer overflows**, each
disconnecting a client, followed by cancellation errors in its reader and writer.
There were no logged order-processing failures or queue-full submission rejections.
Asynchronous delivery kept these socket waits off the matching worker, but sustained
market-feed delivery under this burst remains unresolved. Buffer capacities and
timeouts now have named package/module-level settings; extracting those settings
did not change their values or increase delivery capacity.

### WebSocket batching and frontend profiling — 2026-11-21

The per-client WebSocket writer now collects messages already waiting in its
outbound channel and sends them together as one JSON array. It preserves message
order and the separate price, trade, and depth payloads. It does not wait for more
messages to build a batch; a single message retains its original object format.
The frontend accepts both objects and arrays and dispatches batch entries in order.
Batching reduces the number of socket writes when messages have accumulated, but
does not guarantee delivery: subsequent load tests still recorded write timeouts.

Frontend inspection during a burst reproduced live price advancing ahead of
History. Received trades were being queued and displayed one every 10 ms, limiting
playback to approximately 100 trades per second even when a whole batch had arrived.
`ENABLE_TRADE_PLAYBACK` is now `false`, so each received trade batch updates History
immediately, newest first, while retaining deduplication and the 60-row display cap.
The playback code and 10 ms interval remain available for re-enabling later. This
removes artificial display pacing; it does not provide replay of missed trades or
guarantee atomic synchronization of the separate price and trade updates.

With playback disabled, Chrome's Performance tab was recorded during another
10-second, 8-worker burst. All **4,733** submitted orders completed. Backend median
submission time was **7.29 ms**, median queue wait **5.48 s**, and median worker
processing **4.28 ms**. Two WebSocket writes exceeded the configured 2-second
timeout and disconnected clients. Live price and the newest History row matched
in the observed UI samples, but these samples do not prove synchronization for
every update.

Within the selected **25.18-second** trace range, Chrome reported **16.81 seconds
of scripting**, **695 ms of rendering**, and **375 ms of painting**. The Bottom-up
view showed these self times:

| Activity | Self time |
| --- | ---: |
| `createTask` | 4.77 s |
| `run` | 3.91 s |
| `Run console task` | 3.80 s |
| Price formatter `fmt` | 2.34 s |

The local React development JSX runtime invokes `console.createTask`, identifying
development/debug instrumentation as a substantial cost in this recording. The
trace confirms heavy browser scripting but does not establish that it alone caused
the socket timeouts. Recording and development mode also affect these measurements;
a production-build comparison has not yet been run.

`fmt` now reuses a module-level `Intl.NumberFormat('tr-TR', ...)` instead of calling
`toLocaleString` with formatting options on every render. Trade-time formatting
also reuses a module-level `Intl.DateTimeFormat`. The frontend build and lint checks
passed after the price-formatter change. Its runtime improvement has not yet been
remeasured, and WebSocket delivery timeouts remain unresolved.

## Submission architecture

```mermaid
flowchart TD
    A[POST /orders] --> B[Gin HTTP handler]
    B --> C[Validate and create order]
    C --> D[Assign next session sequence]
    D --> E[Insert order as PENDING]
    E --> F{Bounded queue send}
    F -->|Full| G[Persist REJECTED and return 503]
    F -->|Queued| J[Return HTTP 202]
    F -->|Queued| K[Single per-symbol worker]
    K --> L[Engine.Prepare]
    L --> M[PersistSubmission transaction]
    M -->|Rollback| N[Log background failure; live book unchanged]
    M -->|Commit| O[Engine.Apply]
    O --> P[Update live book and last trade price]
```

The `202` response confirms that the order was created, successfully queued, and
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
