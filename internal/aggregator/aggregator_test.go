package aggregator_test

import (
	"app/internal/aggregator"
	"app/internal/filters"
	"app/internal/models"
	"app/internal/providers"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ─── mockProvider ─────────────────────────────────────────────────────────────
// Implements providers.Provider via a concrete struct.

type mockAggProvider struct {
	name    string
	flights []models.Flight
	err     error
}

func (m *mockAggProvider) Name() string { return m.name }

func (m *mockAggProvider) Fetch(_ context.Context, _ models.SearchRequest) (providers.FetchResult, error) {
	if m.err != nil {
		return providers.FetchResult{}, m.err
	}
	return providers.FetchResult{Data: []byte(`{}`)}, nil
}

func (m *mockAggProvider) Normalize(_ []byte, _ models.SearchRequest) ([]models.Flight, error) {
	return m.flights, m.err
}

// slowProvider simulates a provider that takes a very long time.
type slowProvider struct {
	delay time.Duration
}

func (s *slowProvider) Name() string { return "Slow" }
func (s *slowProvider) Fetch(ctx context.Context, _ models.SearchRequest) (providers.FetchResult, error) {
	select {
	case <-time.After(s.delay):
		return providers.FetchResult{Data: []byte(`{}`)}, nil
	case <-ctx.Done():
		return providers.FetchResult{}, ctx.Err()
	}
}
func (s *slowProvider) Normalize(_ []byte, _ models.SearchRequest) ([]models.Flight, error) {
	return nil, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func makeAggFlight(id string, price int64, stops int, durationMin int, airline string) models.Flight {
	return models.Flight{
		ID:             id,
		Provider:       "Test",
		Airline:        models.Airline{Name: airline, Code: "XX"},
		FlightNumber:   id,
		Price:          models.Price{Amount: price, Currency: "IDR"},
		Stops:          stops,
		Duration:       models.Duration{TotalMinutes: durationMin, Formatted: "1h 30m"},
		Departure:      models.Point{Airport: "CGK", City: "Jakarta", DateTime: "2025-12-15T06:00:00+07:00", Timestamp: 1000000},
		Arrival:        models.Point{Airport: "DPS", City: "Denpasar", DateTime: "2025-12-15T08:00:00+08:00", Timestamp: 1010000},
		CabinClass:     "economy",
		AvailableSeats: 10,
		Amenities:      []string{},
		Baggage:        models.Baggage{CarryOn: "-", Checked: "-"},
	}
}

func defaultOpts() aggregator.SearchOptions {
	return aggregator.SearchOptions{
		Filters: filters.NewChain(),
		SortKey: "price_asc",
	}
}

func defaultReq() models.SearchRequest {
	return models.SearchRequest{
		Origin:        "CGK",
		Destination:   "DPS",
		DepartureDate: "2025-12-15",
		Passengers:    1,
		CabinClass:    "economy",
	}
}

func newSvc(ps ...providers.Provider) *aggregator.Service {
	return aggregator.NewService(ps, 0)
}

// ─── Service tests ────────────────────────────────────────────────────────────

func TestService_SearchReturnsCombinedFlights(t *testing.T) {
	p1 := &mockAggProvider{name: "P1", flights: []models.Flight{
		makeAggFlight("F1", 500000, 0, 90, "Airline A"),
	}}
	p2 := &mockAggProvider{name: "P2", flights: []models.Flight{
		makeAggFlight("F2", 800000, 0, 120, "Airline B"),
		makeAggFlight("F3", 300000, 1, 200, "Airline C"),
	}}

	svc := newSvc(p1, p2)
	resp := svc.Search(context.Background(), defaultReq(), defaultOpts())

	if resp.Metadata.ProvidersQueried != 2 {
		t.Errorf("expected ProvidersQueried=2, got %d", resp.Metadata.ProvidersQueried)
	}
	if resp.Metadata.TotalResults != 3 {
		t.Errorf("expected TotalResults=3, got %d", resp.Metadata.TotalResults)
	}
	if len(resp.Flights) != 3 {
		t.Errorf("expected 3 flights, got %d", len(resp.Flights))
	}
}

func TestService_SearchSortsByPriceAsc(t *testing.T) {
	p := &mockAggProvider{name: "P", flights: []models.Flight{
		makeAggFlight("Expensive", 1500000, 0, 90, "Air X"),
		makeAggFlight("Cheap", 300000, 0, 90, "Air X"),
		makeAggFlight("Medium", 800000, 0, 90, "Air X"),
	}}

	svc := newSvc(p)
	resp := svc.Search(context.Background(), defaultReq(), defaultOpts())

	for i := 1; i < len(resp.Flights); i++ {
		if resp.Flights[i].Price.Amount < resp.Flights[i-1].Price.Amount {
			t.Errorf("flights not sorted by price asc at index %d", i)
		}
	}
}

func TestService_SearchAppliesFilters(t *testing.T) {
	p := &mockAggProvider{name: "P", flights: []models.Flight{
		makeAggFlight("Cheap", 300000, 0, 90, "Air X"),
		makeAggFlight("Expensive", 1500000, 0, 90, "Air X"),
	}}

	svc := newSvc(p)
	opts := aggregator.SearchOptions{
		Filters: filters.NewChain(filters.PriceRange{Min: 0, Max: 500000}),
		SortKey: "price_asc",
	}
	resp := svc.Search(context.Background(), defaultReq(), opts)

	if len(resp.Flights) != 1 || resp.Flights[0].ID != "Cheap" {
		t.Errorf("expected only Cheap flight, got %d flights", len(resp.Flights))
	}
}

func TestService_SearchEchoesSearchCriteria(t *testing.T) {
	svc := newSvc()
	req := defaultReq()
	resp := svc.Search(context.Background(), req, defaultOpts())

	if resp.SearchCriteria.Origin != req.Origin {
		t.Errorf("Origin: got %q, want %q", resp.SearchCriteria.Origin, req.Origin)
	}
	if resp.SearchCriteria.Destination != req.Destination {
		t.Errorf("Destination: got %q, want %q", resp.SearchCriteria.Destination, req.Destination)
	}
	if resp.SearchCriteria.DepartureDate != req.DepartureDate {
		t.Errorf("DepartureDate: got %q, want %q", resp.SearchCriteria.DepartureDate, req.DepartureDate)
	}
}

func TestService_SearchWithNoProviders(t *testing.T) {
	svc := newSvc()
	resp := svc.Search(context.Background(), defaultReq(), defaultOpts())

	if resp.Metadata.ProvidersQueried != 0 {
		t.Errorf("expected 0 ProvidersQueried, got %d", resp.Metadata.ProvidersQueried)
	}
	if len(resp.Flights) != 0 {
		t.Errorf("expected 0 flights, got %d", len(resp.Flights))
	}
}

func TestService_SearchWithFailingProvider(t *testing.T) {
	failing := &mockAggProvider{name: "Fail", err: context.DeadlineExceeded}
	ok := &mockAggProvider{name: "OK", flights: []models.Flight{
		makeAggFlight("F1", 500000, 0, 90, "Air X"),
	}}

	svc := newSvc(failing, ok)
	resp := svc.Search(context.Background(), defaultReq(), defaultOpts())

	// Flights dari provider yang berhasil tetap muncul
	if len(resp.Flights) != 1 {
		t.Errorf("expected 1 flight from successful provider, got %d", len(resp.Flights))
	}
}

func TestService_SearchRecordsElapsedTime(t *testing.T) {
	svc := newSvc()
	resp := svc.Search(context.Background(), defaultReq(), defaultOpts())

	if resp.Metadata.SearchTimeMs < 0 {
		t.Error("SearchTimeMs must not be negative")
	}
}

func TestService_SearchMultipleProvidersSorted(t *testing.T) {
	p1 := &mockAggProvider{name: "P1", flights: []models.Flight{
		makeAggFlight("A", 1000000, 0, 90, "Air A"),
	}}
	p2 := &mockAggProvider{name: "P2", flights: []models.Flight{
		makeAggFlight("B", 500000, 0, 120, "Air B"),
	}}

	svc := newSvc(p1, p2)
	resp := svc.Search(context.Background(), defaultReq(), aggregator.SearchOptions{
		Filters: filters.NewChain(),
		SortKey: "price_asc",
	})

	if len(resp.Flights) != 2 {
		t.Fatalf("expected 2 flights, got %d", len(resp.Flights))
	}
	if resp.Flights[0].Price.Amount != 500000 {
		t.Errorf("first flight should be cheapest (500000), got %d", resp.Flights[0].Price.Amount)
	}
}

// ─── Handler tests ────────────────────────────────────────────────────────────

func TestHandler_SearchRejectsNonPOST(t *testing.T) {
	svc := newSvc()
	h := aggregator.NewHandler(svc)

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/search", nil)
		w := httptest.NewRecorder()
		h.Search(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("method %s: expected 405, got %d", method, w.Code)
		}
	}
}

func TestHandler_SearchRejectsMissingFields(t *testing.T) {
	svc := newSvc()
	h := aggregator.NewHandler(svc)

	cases := []struct {
		name string
		body string
	}{
		{"missing origin", `{"destination":"DPS","departureDate":"2025-12-15","passengers":1}`},
		{"missing destination", `{"origin":"CGK","departureDate":"2025-12-15","passengers":1}`},
		{"missing departureDate", `{"origin":"CGK","destination":"DPS","passengers":1}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(tc.body))
			w := httptest.NewRecorder()
			h.Search(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("%s: expected 400, got %d", tc.name, w.Code)
			}
		})
	}
}

func TestHandler_SearchRejectsInvalidJSON(t *testing.T) {
	svc := newSvc()
	h := aggregator.NewHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader("not-json"))
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestHandler_SearchReturnsJSON(t *testing.T) {
	p := &mockAggProvider{name: "P", flights: []models.Flight{
		makeAggFlight("F1", 500000, 0, 90, "Air X"),
	}}
	svc := newSvc(p)
	h := aggregator.NewHandler(svc)

	body := `{"origin":"CGK","destination":"DPS","departureDate":"2025-12-15","passengers":1,"cabinClass":"economy"}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var resp models.SearchResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Flights) != 1 {
		t.Errorf("expected 1 flight, got %d", len(resp.Flights))
	}
}

func TestHandler_SearchDefaultsPassengersTo1(t *testing.T) {
	svc := newSvc()
	h := aggregator.NewHandler(svc)

	body := `{"origin":"CGK","destination":"DPS","departureDate":"2025-12-15"}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp models.SearchResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.SearchCriteria.Passengers != 1 {
		t.Errorf("expected Passengers=1, got %d", resp.SearchCriteria.Passengers)
	}
}

func TestHandler_SearchDefaultsCabinClassToEconomy(t *testing.T) {
	svc := newSvc()
	h := aggregator.NewHandler(svc)

	body := `{"origin":"CGK","destination":"DPS","departureDate":"2025-12-15","passengers":1}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp models.SearchResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.SearchCriteria.CabinClass != "economy" {
		t.Errorf("expected CabinClass=economy, got %q", resp.SearchCriteria.CabinClass)
	}
}

func TestHandler_SearchRespectsTimeout(t *testing.T) {
	slow := &slowProvider{delay: 10 * time.Second}
	svc := newSvc(slow)
	h := aggregator.NewHandler(svc)

	body := `{"origin":"CGK","destination":"DPS","departureDate":"2025-12-15","passengers":1}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))

	start := time.Now()
	w := httptest.NewRecorder()
	h.Search(w, req)
	elapsed := time.Since(start)

	// Handler timeout 5s, jadi harus selesai jauh sebelum 10s
	if elapsed > 7*time.Second {
		t.Errorf("handler took too long (%v), should respect 5s timeout", elapsed)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (empty result ok), got %d", w.Code)
	}
}

func TestHandler_SearchWithFiltersAndSort(t *testing.T) {
	p := &mockAggProvider{name: "P", flights: []models.Flight{
		makeAggFlight("F1", 300000, 2, 300, "Air X"), // 2 stops – filtered by maxStops=1
		makeAggFlight("F2", 800000, 0, 90, "Air X"),
		makeAggFlight("F3", 500000, 0, 120, "Air X"),
	}}
	svc := newSvc(p)
	h := aggregator.NewHandler(svc)

	// maxStops=1 (filters out F1 with 2 stops), sortBy=price_desc
	// Note: handler treats maxStops<=0 as unlimited (10), so we use 1 here.
	body := `{"origin":"CGK","destination":"DPS","departureDate":"2025-12-15","passengers":1,"maxStops":1,"sortBy":"price_desc"}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.Search(w, req)

	var resp models.SearchResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Flights) != 2 {
		t.Fatalf("expected 2 flights after maxStops=1 filter, got %d", len(resp.Flights))
	}
	// Sorted price_desc → F2 (800k) first
	if resp.Flights[0].Price.Amount != 800000 {
		t.Errorf("expected 800000 first (price_desc), got %d", resp.Flights[0].Price.Amount)
	}
}

func TestHandler_SearchWithTimeFilters(t *testing.T) {
	// Create mock flights with different departure/arrival timestamps.
	// F1: Depart Noon, Arrive Evening
	// F2: Depart Morning, Arrive Noon
	f1 := makeAggFlight("F1", 500000, 0, 90, "Air X")
	f1.Departure.DateTime = "2025-12-15T12:00:00Z"
	tDep1, _ := time.Parse(time.RFC3339, f1.Departure.DateTime)
	f1.Departure.Timestamp = tDep1.Unix()
	f1.Arrival.DateTime = "2025-12-15T18:00:00Z"
	tArr1, _ := time.Parse(time.RFC3339, f1.Arrival.DateTime)
	f1.Arrival.Timestamp = tArr1.Unix()

	f2 := makeAggFlight("F2", 600000, 0, 90, "Air X")
	f2.Departure.DateTime = "2025-12-15T06:00:00Z"
	tDep2, _ := time.Parse(time.RFC3339, f2.Departure.DateTime)
	f2.Departure.Timestamp = tDep2.Unix()
	f2.Arrival.DateTime = "2025-12-15T12:00:00Z"
	tArr2, _ := time.Parse(time.RFC3339, f2.Arrival.DateTime)
	f2.Arrival.Timestamp = tArr2.Unix()

	p := &mockAggProvider{name: "P", flights: []models.Flight{f1, f2}}
	svc := newSvc(p)
	h := aggregator.NewHandler(svc)

	// Filter departure after 2025-12-15T10:00:00Z
	body := `{"origin":"CGK","destination":"DPS","departureDate":"2025-12-15","passengers":1,"departureAfter":"2025-12-15T10:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.Search(w, req)

	var resp models.SearchResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Flights) != 1 {
		t.Fatalf("expected 1 flight after departureAfter filter, got %d", len(resp.Flights))
	}
	if resp.Flights[0].ID != "F1" {
		t.Errorf("expected F1 (departing at 12:00), got %s", resp.Flights[0].ID)
	}

	// Filter arrival before 2025-12-15T14:00:00Z
	body2 := `{"origin":"CGK","destination":"DPS","departureDate":"2025-12-15","passengers":1,"arrivalBefore":"2025-12-15T14:00:00Z"}`
	req2 := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body2))
	w2 := httptest.NewRecorder()
	h.Search(w2, req2)

	var resp2 models.SearchResponse
	json.NewDecoder(w2.Body).Decode(&resp2)

	if len(resp2.Flights) != 1 {
		t.Fatalf("expected 1 flight after arrivalBefore filter, got %d", len(resp2.Flights))
	}
	if resp2.Flights[0].ID != "F2" {
		t.Errorf("expected F2 (arriving at 12:00), got %s", resp2.Flights[0].ID)
	}
}

