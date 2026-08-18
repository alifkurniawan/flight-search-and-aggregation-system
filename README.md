# ✈️ Flight Aggregator API

A backend service that aggregates flight search results from four mock airline providers (Garuda Indonesia, Lion Air, Batik Air, AirAsia) into a single, normalized API response — with filtering, sorting, caching, retry, and a "best value" ranking algorithm.

Built for the BookCabin Software Engineer take-home assignment.

---

## Table of Contents

- [Requirements](#requirements)
- [Setup & Run](#setup--run)
- [API Usage](#api-usage)
- [Running Tests](#running-tests)
- [Project Structure](#project-structure)
- [Approach & Design Choices](#approach--design-choices)
- [Known Limitations](#known-limitations)

---

## Requirements

- **Go 1.21+** (developed against Go 1.26.5, as declared in `go.mod`)
- No external dependencies — standard library only, no `go mod tidy` needed
- No network access required — all four providers are mocked with data embedded at compile time (`//go:embed`), so the service runs fully offline

---

## Setup & Run

### 1. Clone the repository

```bash
git clone https://github.com/alifkurniawan/flight-search-and-aggregation-system.git
cd flight-search-and-aggregation-system
```

### 2. Run directly

```bash
go run ./cmd/api
```

The server starts on **`:8080`** by default. On startup, a demo search (`CGK → DPS`, sorted by `best_value`) also runs and prints its `metadata` and `flights` to stdout, so you can see the service working without sending a request yourself.

To use a custom port:

```bash
go run ./cmd/api -addr :9090
```

### 3. Build and run a binary

```bash
go build -o flight-aggregator ./cmd/api
./flight-aggregator
```

### 4. Try it

```bash
curl -X POST http://localhost:8080/search \
  -H "Content-Type: application/json" \
  -d '{
        "origin": "CGK",
        "destination": "DPS",
        "departureDate": "2025-12-15",
        "passengers": 1,
        "cabinClass": "economy"
      }'

curl http://localhost:8080/health
```

---

## API Usage

### `POST /search`

Search for available flights across all four providers, in parallel, with optional filtering and sorting.

**Request body (JSON):**

```json
{
  "origin": "CGK",
  "destination": "DPS",
  "departureDate": "2025-12-15",
  "passengers": 1,
  "cabinClass": "economy",
  "minPrice": 0,
  "maxPrice": 1500000,
  "maxStops": 1,
  "airlines": ["Garuda Indonesia", "Lion Air"],
  "maxDurationMinutes": 180,
  "departureAfter": "2025-12-15T10:00:00Z",
  "departureBefore": "2025-12-15T18:00:00Z",
  "arrivalAfter": "2025-12-15T12:00:00Z",
  "arrivalBefore": "2025-12-15T22:00:00Z",
  "sortBy": "best_value"
}
```

Only `origin`, `destination`, and `departureDate` are required — every other field is an optional filter/sort knob.

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `origin` | string | ✅ | — | IATA airport code (e.g. `CGK`) |
| `destination` | string | ✅ | — | IATA airport code (e.g. `DPS`) |
| `departureDate` | string | ✅ | — | Date string (e.g. `2025-12-15`) |
| `passengers` | int | ❌ | `1` | Number of passengers |
| `cabinClass` | string | ❌ | `"economy"` | Cabin class |
| `minPrice` | int64 | ❌ | `0` | Minimum price in IDR |
| `maxPrice` | int64 | ❌ | `0` (no limit) | Maximum price in IDR |
| `maxStops` | int | ❌ | `10` (unlimited) | Max number of stops |
| `airlines` | []string | ❌ | all | Filter by exact airline name (e.g. `"Garuda Indonesia"`, not `"GA"`) |
| `maxDurationMinutes` | int | ❌ | `0` (no limit) | Max total flight duration |
| `departureAfter` / `departureBefore` | string (RFC3339) | ❌ | — | Filter by departure time window |
| `arrivalAfter` / `arrivalBefore` | string (RFC3339) | ❌ | — | Filter by arrival time window |
| `sortBy` | string | ❌ | `"price_asc"` | One of: `price_asc`, `price_desc`, `duration_asc`, `duration_desc`, `departure_time`, `arrival_time`, `best_value` |

> Any `sortBy` value not in that list (including an empty/omitted field) falls back to `price_asc`.

**Response (JSON):**

```json
{
  "search_criteria": {
    "origin": "CGK",
    "destination": "DPS",
    "departure_date": "2025-12-15",
    "passengers": 1,
    "cabin_class": "economy"
  },
  "metadata": {
    "total_results": 10,
    "providers_queried": 4,
    "providers_succeeded": 4,
    "providers_failed": 0,
    "search_time_ms": 312,
    "cache_hit": false
  },
  "flights": [
    {
      "id": "QZ520_AirAsia",
      "provider": "AirAsia",
      "airline": { "name": "AirAsia", "code": "QZ" },
      "flight_number": "QZ520",
      "departure": {
        "airport": "CGK",
        "city": "Jakarta",
        "datetime": "2025-12-15T04:45:00+07:00",
        "timestamp": 1765748700
      },
      "arrival": {
        "airport": "DPS",
        "city": "Denpasar",
        "datetime": "2025-12-15T07:25:00+08:00",
        "timestamp": 1765754700
      },
      "duration": { "total_minutes": 100, "formatted": "1h 40m" },
      "stops": 0,
      "price": { "amount": 650000, "currency": "IDR", "formatted": "Rp650.000" },
      "available_seats": 67,
      "cabin_class": "economy",
      "aircraft": null,
      "amenities": [],
      "baggage": { "carry_on": "Cabin baggage only", "checked": "checked bags additional fee" }
    }
  ]
}
```

Notes on specific fields:

- **`metadata.providers_succeeded` / `providers_failed`** reflect what actually happened on *this* request. AirAsia is mocked with a 10% failure rate, so occasionally you'll see `providers_failed: 1` — the search still returns results from the other three providers rather than failing the whole request.
- **`metadata.cache_hit`** is `true` if **at least one** of the queried providers served its data from the in-memory cache (60s TTL) rather than fetching fresh. Send the same search twice in a row within 60 seconds to see it flip to `true`.
- **`price.formatted`** is an IDR string with `.` as the thousands separator (e.g. `"Rp1.500.000"`), following Indonesian number formatting conventions.
- **`best_value_score`** (0–100, higher is better) only appears on each flight when `sortBy` is `"best_value"`; it's omitted for every other sort.

### `GET /health`

Returns server health status.

```json
{ "status": "ok" }
```

---

## Running Tests

```bash
# Run all tests
go test ./...

# Verbose output
go test ./... -v

# Run one package at a time
go test ./internal/providers/...
go test ./internal/cache/...
go test ./internal/filters/...
go test ./internal/sorts/...
go test ./internal/aggregator/...
go test ./internal/utils/...

# With the race detector
go test -race ./...
```

**Test coverage per package:**

| Package | What's tested |
|---|---|
| `utils` | `CityFromAirport`, `FormatDuration`, `FormatIDR` |
| `filters` | `PriceRange`, `MaxStops`, `Airlines`, `MaxDuration`, `DepartureTime`, `ArrivalTime`, `Chain` |
| `sorts` | `price_asc`, `price_desc`, `duration_asc`, `duration_desc`, `departure_time`, `arrival_time`, `best_value`, unknown-key fallback |
| `providers` | All 4 adapters: `Name()`, `Fetch()` + context cancellation, `Normalize()` + malformed/missing-field edge cases |
| `cache` | `Store` (TTL expiry, overwrite), `cachingProvider` (hit/miss, error passthrough, per-request cache keys), `retryingProvider` (backoff, max-retries exhaustion) |
| `aggregator` | `Service` (parallel fan-out, filter+sort pipeline, partial provider failure), `Handler` (HTTP contract, defaults, validation, time-window filters) |

All provider responses are embedded fixtures, and all provider delays/failures are simulated with `time.Sleep`/`math/rand` — so the full suite runs in a few seconds with no flakiness from real I/O.

---

## Project Structure

```
app/
├── cmd/
│   └── api/
│       └── main.go               # Entry point: wires providers + cache, starts HTTP server
├── internal/
│   ├── models/
│   │   └── flight.go             # Shared domain types (Flight, SearchRequest, SearchResponse, ...)
│   ├── providers/
│   │   ├── provider.go           # Provider interface, FetchResult type, All() registry
│   │   ├── airasia.go            # AirAsia adapter (10% simulated failure rate)
│   │   ├── garuda.go             # Garuda Indonesia adapter
│   │   ├── lion.go                # Lion Air adapter (IANA timezone names)
│   │   ├── batik.go              # Batik Air adapter (non-colon UTC offsets)
│   │   ├── retry.go              # retryingProvider decorator (exponential backoff)
│   │   ├── providers_test.go
│   │   └── data/                 # Embedded per-provider JSON fixtures (go:embed)
│   ├── cache/
│   │   ├── cache.go              # In-memory TTL Store + cachingProvider decorator
│   │   └── cache_retry_test.go
│   ├── aggregator/
│   │   ├── service.go            # Parallel fan-out, per-provider timeout, response assembly
│   │   ├── handler.go            # HTTP handler: request parsing, defaults, filter-chain builder
│   │   └── aggregator_test.go
│   ├── filters/
│   │   ├── filter.go             # Filter types (PriceRange, MaxStops, Airlines, ...) + Chain
│   │   └── filter_test.go
│   ├── sorts/
│   │   ├── sortstrategy.go       # Sort strategy registry / GetStrategy(key)
│   │   ├── bestvalue.go          # "Best value" scoring algorithm
│   │   └── sorts_test.go
│   └── utils/
│       ├── airport.go            # IATA code → city name lookup
│       ├── timezone.go           # Duration formatter ("260" -> "4h 20m")
│       ├── currency.go           # IDR thousands-separator formatter
│       └── utils_test.go
├── .gitignore
└── go.mod                        # Module: app, Go 1.26.5, zero external deps
```

---

## Approach & Design Choices

### 1. Provider Adapter Pattern

Each airline exposes a completely different JSON schema. A `Provider` interface with three methods — `Name()`, `Fetch()`, `Normalize()` — decouples data retrieval from normalization. Each adapter owns the raw struct matching its vendor's format and maps it to the common `models.Flight` type. Adding a new airline means adding one new file that implements the interface — zero changes anywhere else.

### 2. Parallel Fan-out with a Bounded Timeout

`Service.fetchAll()` fires all provider `Fetch` calls concurrently in goroutines and collects results over a buffered channel. Total response time is bounded by the **slowest provider that responds within its timeout**, not the sum of all provider latencies. Each goroutine gets its own `context.WithTimeout` (configurable via `NewService(providers, timeout)`, defaulting to 2s), so one slow or hanging provider can't stall the others or the overall request.

### 3. Layered Middleware via Decorators

Caching and retrying are implemented as decorators that wrap the same `Provider` interface, rather than being baked into each adapter:

- `cache.Wrap(inner, store)` — adds TTL caching (default 60s), keyed by `sha1(providerName + searchRequest)`. `Fetch` returns a `FetchResult{Data, FromCache}` so the aggregator can tell whether a given provider's data came from cache; the response's `metadata.cache_hit` is `true` if **any** provider in that search was served from cache.
- `providers.WithRetry(inner, maxRetries)` — adds exponential backoff (`100ms * 2^attempt`) on transient failures.

They compose freely — in `main.go`, AirAsia is wrapped with retry *first*, then the whole chain (all four providers) is wrapped with caching, so a cache entry is only ever written after a successful (post-retry) fetch. This follows the open/closed principle: the core provider adapters never need to change to gain new cross-cutting behavior.

### 4. Composable Filter Chain

Filters implement a single `Apply([]Flight) []Flight` method and compose via `filters.NewChain(...)`, each filter's output feeding the next. `Handler` builds the chain from the request body, with sensible defaults (e.g. `maxStops <= 0` means "no cap", implemented as `10`).

### 5. Provider-Specific Datetime Normalization

Each airline formats timestamps differently:

| Provider | Raw format | Handling |
|---|---|---|
| **Garuda** | RFC3339 (`+07:00`) | Direct `time.Parse` |
| **AirAsia** | RFC3339 (`+07:00`) | Direct `time.Parse` |
| **Batik Air** | `+0700` (no colon) | Custom parser inserts the colon, then re-parses as RFC3339 |
| **Lion Air** | No offset; timezone given as an IANA name (e.g. `Asia/Jakarta`) | Looks up the IANA name in a small offset table, appends it, then parses |

All normalized output is stored as both an RFC3339 string (preserving the original local offset) and a Unix timestamp, so downstream filtering/sorting never has to re-parse strings.

### 6. "Best Value" Scoring

The assignment leaves "convenience" undefined, so this was a judgment call — documented in `internal/sorts/bestvalue.go` and summarized here. Each flight gets a 0–100 score from four weighted, normalized factors:

| Factor | Weight | Rationale |
|---|---|---|
| Price | 40% | The dominant driver for most travelers |
| Stops | 30% | Each connection adds real risk (delays, missed connections) |
| Departure time | 20% | Red-eye departures (23:00–06:00) penalized; daytime (09:00–21:00) scores highest |
| Duration | 10% | Already correlates with stops, so weighted lightly to avoid double-counting |

Price, stops, and duration are normalized against the min/max **within that search's own results** (not a fixed global scale), so the score always reflects what's actually on offer for that specific route and date. Departure-time convenience is read from the flight's *local* wall-clock hour (straight off the RFC3339 string, not converted to UTC first) — "red-eye" is inherently a local-time concept.

`sortBy` defaults to `price_asc` rather than `best_value`, to keep the default response shape minimal and predictable; `best_value_score` only appears when a caller explicitly opts into `sortBy: "best_value"`.

### 7. Embedded Test Fixtures, Zero Dependencies

All four provider JSON responses are embedded at compile time with `//go:embed` and reused by both production `Fetch()` and the test suite — no external files or network calls needed, even in CI. Combined with using only the Go standard library, the project builds and tests with a single `go build` / `go test ./...` and no `go mod tidy`.

---

## Known Limitations

Not implemented, out of scope for the time available:

- **Round-trip search** — `SearchRequest.ReturnDate` exists in the model but isn't used by any provider or the aggregation logic.
- **Rate limiting** on provider calls.
- **Multi-city search.**
- **Cross-provider price comparison** for the "same" flight (e.g. detecting that two providers are quoting the same route/schedule and diffing their prices) — each flight from each provider is currently treated as an independent result.
