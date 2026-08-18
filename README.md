# ✈️ Flight Aggregator API

A backend service that aggregates flight search results from multiple airline providers (Garuda Indonesia, Lion Air, Batik Air, AirAsia) into a single unified API response.

---

## Table of Contents

- [Requirements](#requirements)
- [Setup & Run](#setup--run)
- [API Usage](#api-usage)
- [Running Tests](#running-tests)
- [Project Structure](#project-structure)
- [Design Choices](#design-choices)

---

## Requirements

- **Go 1.21+** (the project uses Go 1.26.5 as declared in `go.mod`)
- No external dependencies — standard library only

---

## Setup & Run

### 2. Run directly

```bash
go run ./cmd/api
```

The server starts on **`:8080`** by default. A demo search result is printed to stdout on startup.

To use a custom port:

```bash
go run ./cmd/api -addr :9090
```

### 3. Build and run the binary

```bash
go build -o flight-aggregator ./cmd/api
./flight-aggregator
```



## API Usage

### `POST /search`

Search for available flights across all providers.

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
  "sortBy": "price_asc"
}
```

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
| `airlines` | []string | ❌ | all | Filter by airline names |
| `maxDurationMinutes` | int | ❌ | `0` (no limit) | Max flight duration |
| `departureAfter` | string | ❌ | — | Filter departure times after this ISO-8601/RFC3339 datetime |
| `departureBefore` | string | ❌ | — | Filter departure times before this ISO-8601/RFC3339 datetime |
| `arrivalAfter` | string | ❌ | — | Filter arrival times after this ISO-8601/RFC3339 datetime |
| `arrivalBefore` | string | ❌ | — | Filter arrival times before this ISO-8601/RFC3339 datetime |
| `sortBy` | string | ❌ | `"price_asc"` | `price_asc` \| `price_desc` \| `duration_asc` |

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
        "timestamp": 1765766700
      },
      "arrival": {
        "airport": "DPS",
        "city": "Denpasar",
        "datetime": "2025-12-15T07:25:00+08:00",
        "timestamp": 1765777500
      },
      "duration": { "total_minutes": 100, "formatted": "1h 40m" },
      "stops": 0,
      "price": { "amount": 650000, "currency": "IDR" },
      "available_seats": 67,
      "cabin_class": "economy",
      "aircraft": null,
      "amenities": [],
      "baggage": { "carry_on": "Cabin baggage only", "checked": "checked bags additional fee" }
    }
  ]
}
```

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

# Run with verbose output
go test ./... -v

# Run specific package
go test ./internal/providers/...
go test ./internal/filters/...
go test ./internal/aggregator/...

# Run with race detector
go test -race ./...
```

**Test coverage per package:**

| Package | What's tested |
|---|---|
| `utils` | `CityFromAirport`, `FormatDuration` |
| `filters` | `PriceRange`, `MaxStops`, `Airlines`, `MaxDuration`, `DepartureTime`, `Chain` |
| `sorts` | `price_asc`, `price_desc`, `duration_asc`, unknown key fallback |
| `providers` | All 4 adapters: `Name()`, `Fetch()` + context cancel, `Normalize()` + edge cases |
| `providers` | `Store` (cache TTL), `cachingProvider`, `retryingProvider` |
| `aggregator` | `Service` (fan-out, filter+sort, partial failure), `Handler` (HTTP contract) |

---

## Project Structure

```
app/
├── cmd/
│   └── api/
│       └── main.go              # Entry point: wires providers, service, HTTP mux
├── internal/
│   ├── models/
│   │   └── flight.go            # Shared domain types (Flight, SearchRequest, etc.)
│   ├── providers/
│   │   ├── provider.go          # Provider interface
│   │   ├── airasia.go           # AirAsia adapter
│   │   ├── garuda.go            # Garuda Indonesia adapter
│   │   ├── lion.go              # Lion Air adapter
│   │   ├── batik.go             # Batik Air adapter
│   │   ├── cache.go             # In-memory TTL cache + cachingProvider wrapper
│   │   ├── retry.go             # retryingProvider wrapper (exponential backoff)
│   │   ├── providers_test.go    # Provider adapter unit tests
│   │   ├── cache_retry_test.go  # Cache & retry unit tests
│   │   └── data/                # Embedded JSON fixture files per provider
│   ├── aggregator/
│   │   ├── service.go           # Parallel fan-out search across all providers
│   │   ├── handler.go           # HTTP handler + request parsing + filter chain builder
│   │   └── aggregator_test.go   # Service & Handler unit tests
│   ├── filters/
│   │   ├── filter.go            # Filter types + Chain
│   │   └── filter_test.go       # Filter unit tests
│   ├── sorts/
│   │   ├── sortstrategy.go      # Sort strategy registry
│   │   └── sorts_test.go        # Sort unit tests
│   └── utils/
│       ├── airport.go           # IATA code → city name lookup
│       ├── timezone.go          # Duration formatter
│       └── utils_test.go        # Utils unit tests
└── go.mod                       # Module: app, Go 1.26.5, zero external deps
```

---

## Design Choices

### 1. Provider Adapter Pattern

Each airline exposes a completely different JSON schema. A `Provider` interface with three methods — `Name()`, `Fetch()`, and `Normalize()` — decouples data retrieval from normalization. Each adapter owns the raw struct that matches its vendor's format and maps it to the common `models.Flight` type.

This means adding a new airline requires only creating a new file that implements the interface — zero changes to the aggregator or any other package.

### 2. Parallel Fan-out with `sync.WaitGroup`

`Service.fetchAll()` fires all provider `Fetch` calls concurrently in goroutines and collects results through a buffered channel. The total response time is bounded by the **slowest provider that responds within the timeout**, not the sum of all provider latencies.

Per-provider goroutines each receive their own derived `context.WithTimeout(2s)`, isolating one slow or failing provider from the others.

### 3. Layered Middleware via Wrapping

Caching and retrying are implemented as **decorator wrappers** that implement the same `Provider` interface:

- `providers.Wrap(inner, store)` → adds TTL caching keyed by `sha1(providerName + searchRequest)`. For the aggregated response metadata, `cache_hit` is calculated using an **OR logic**: if at least one of the queried providers serves its results from the cache, the overall search response metadata flags `cache_hit` as `true`.
- `providers.WithRetry(inner, maxRetries)` → adds exponential backoff on transient failures.

These can be composed freely: `Wrap(WithRetry(NewAirAsia(), 3), store)`. This follows the open/closed principle — the core providers are not modified to gain new capabilities.

### 4. Filter Chain (Composable Filters)

Filters implement a single `Apply([]Flight) []Flight` method and are composed via `filters.NewChain(...)`. The chain runs each filter sequentially, passing the output of one as the input to the next. Adding a new filter type requires no changes to existing code.

The handler builds the filter chain from request fields, with sensible defaults (e.g. `maxStops ≤ 0` becomes `10` to mean "no cap").

### 5. Provider-specific Datetime Normalization

Each airline uses a different datetime format:

| Provider | Format | Challenge |
|---|---|---|
| **AirAsia** | RFC3339 (`+07:00`) | Standard — direct `time.Parse` |
| **Garuda** | RFC3339 (`+07:00`) | Standard — direct `time.Parse` |
| **Batik Air** | `+0700` (no colon) | Custom parser normalizes to RFC3339 |
| **Lion Air** | No offset, timezone as IANA name | Maps IANA name → UTC offset, then parses |

All normalized output uses `time.RFC3339` and Unix timestamps for consistency and easy comparison.

### 6. Best-Value Scoring

The assignment leaves "convenience" undefined, so this is a deliberate design decision. As documented in `internal/sorts/bestvalue.go`, weights reflect passenger priority, sorted from most to least important:

**Price > Number of Stops > Departure-time convenience > Total Duration**

These are a judgment call to make the reasoning explicit, not just the numbers, combined into a weighted 0-100 score:

| Factor | Weight | Rationale |
|---|---|---|
| Price | 40% | The dominant driver for most travelers (`weightPrice = 0.40`) |
| Stops | 30% | Each connection adds real risk (`weightStops = 0.30`) |
| Departure time | 20% | Red-eye departures (23:00–06:00) are penalized; daytime (09:00–21:00) scores highest (`weightDeparture = 0.20`) |
| Duration | 10% | Already correlates with stops, so weighted lightly to avoid double-counting (`weightDuration = 0.10`) |

Each factor is normalized against the min/max observed *in that specific search's results* (not a fixed global scale), so the score is always relative to what is actually on offer for this specific search. Departure-time convenience is read from the *local* wall-clock hour embedded in each flight's RFC3339 departure string — never converted to UTC first, since "red-eye" is inherently a local-time concept.

### 7. Embedded Test Fixtures

Provider JSON responses are embedded at compile time with `//go:embed`. This makes tests fully self-contained — no external files need to be present at test runtime, and it avoids network calls. The same embedded data is used in both production `Fetch()` and unit tests.

### 8. Zero External Dependencies

The entire service — HTTP server, JSON parsing, concurrency, caching, testing — uses only the Go standard library. This keeps the binary small, eliminates supply chain risk, and ensures the project compiles with a single `go build` and no `go mod tidy` required.
