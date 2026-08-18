package providers_test

import (
	"app/internal/models"
	"app/internal/providers"
	"context"
	"testing"
	"time"
)

var dummyReq = models.SearchRequest{
	Origin:        "CGK",
	Destination:   "DPS",
	DepartureDate: "2025-12-15",
	Passengers:    1,
	CabinClass:    "economy",
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func assertFlight(t *testing.T, f models.Flight, label string) {
	t.Helper()
	if f.ID == "" {
		t.Errorf("%s: ID is empty", label)
	}
	if f.Provider == "" {
		t.Errorf("%s: Provider is empty", label)
	}
	if f.Airline.Name == "" || f.Airline.Code == "" {
		t.Errorf("%s: Airline incomplete: %+v", label, f.Airline)
	}
	if f.FlightNumber == "" {
		t.Errorf("%s: FlightNumber is empty", label)
	}
	if f.Departure.Airport == "" {
		t.Errorf("%s: Departure.Airport is empty", label)
	}
	if f.Arrival.Airport == "" {
		t.Errorf("%s: Arrival.Airport is empty", label)
	}
	if f.Departure.Timestamp == 0 {
		t.Errorf("%s: Departure.Timestamp is zero", label)
	}
	if f.Arrival.Timestamp == 0 {
		t.Errorf("%s: Arrival.Timestamp is zero", label)
	}
	if f.Arrival.Timestamp <= f.Departure.Timestamp {
		t.Errorf("%s: Arrival must be after Departure", label)
	}
	if f.Duration.TotalMinutes <= 0 {
		t.Errorf("%s: Duration.TotalMinutes must be > 0, got %d", label, f.Duration.TotalMinutes)
	}
	if f.Duration.Formatted == "" {
		t.Errorf("%s: Duration.Formatted is empty", label)
	}
	if f.Price.Amount <= 0 {
		t.Errorf("%s: Price.Amount must be > 0, got %d", label, f.Price.Amount)
	}
	if f.Price.Currency == "" {
		t.Errorf("%s: Price.Currency is empty", label)
	}
	if f.CabinClass == "" {
		t.Errorf("%s: CabinClass is empty", label)
	}
	if f.Amenities == nil {
		t.Errorf("%s: Amenities must not be nil (use empty slice)", label)
	}
}

func fetchAndNormalize(t *testing.T, p providers.Provider) []models.Flight {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := p.Fetch(ctx, dummyReq)
	if err != nil {
		t.Fatalf("%s Fetch failed: %v", p.Name(), err)
	}
	if len(result.Data) == 0 {
		t.Fatalf("%s Fetch returned empty data", p.Name())
	}

	flights, err := p.Normalize(result.Data, dummyReq)
	if err != nil {
		t.Fatalf("%s Normalize failed: %v", p.Name(), err)
	}
	return flights
}

// ─── AirAsia ─────────────────────────────────────────────────────────────────

func TestAirAsiaProvider_Name(t *testing.T) {
	p := providers.NewAirAsia()
	if p.Name() != "AirAsia" {
		t.Errorf("expected Name()=AirAsia, got %q", p.Name())
	}
}

func TestAirAsiaProvider_FetchAndNormalize(t *testing.T) {
	p := providers.NewAirAsia()
	flights := fetchAndNormalize(t, p)

	if len(flights) == 0 {
		t.Fatal("AirAsia: expected at least 1 flight")
	}
	for i, f := range flights {
		assertFlight(t, f, "AirAsia["+string(rune('0'+i))+"]")
		if f.Provider != "AirAsia" {
			t.Errorf("AirAsia: flight Provider = %q, want AirAsia", f.Provider)
		}
		if f.Airline.Code != "QZ" {
			t.Errorf("AirAsia: airline code = %q, want QZ", f.Airline.Code)
		}
	}
}

func TestAirAsiaProvider_FetchRespectsContextCancellation(t *testing.T) {
	p := providers.NewAirAsia()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := p.Fetch(ctx, dummyReq)
	if err == nil {
		t.Error("AirAsia Fetch: expected error on cancelled context, got nil")
	}
}

func TestAirAsiaProvider_NormalizeInvalidJSON(t *testing.T) {
	p := providers.NewAirAsia()
	_, err := p.Normalize([]byte("not-json{{{"), dummyReq)
	if err == nil {
		t.Error("AirAsia Normalize: expected error for invalid JSON, got nil")
	}
}

func TestAirAsiaProvider_NormalizeEmptyFlights(t *testing.T) {
	p := providers.NewAirAsia()
	flights, err := p.Normalize([]byte(`{"status":"ok","flights":[]}`), dummyReq)
	if err != nil {
		t.Fatalf("AirAsia Normalize empty flights: unexpected error: %v", err)
	}
	if len(flights) != 0 {
		t.Errorf("expected 0 flights, got %d", len(flights))
	}
}

func TestAirAsiaProvider_NormalizeSkipsInvalidTimes(t *testing.T) {
	p := providers.NewAirAsia()
	raw := []byte(`{"status":"ok","flights":[
		{"flight_code":"QZ999","airline":"AirAsia","from_airport":"CGK","to_airport":"DPS",
		 "depart_time":"invalid","arrive_time":"also-invalid","direct_flight":true,
		 "price_idr":500000,"seats":50,"cabin_class":"economy","baggage_note":""}
	]}`)
	flights, err := p.Normalize(raw, dummyReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(flights) != 0 {
		t.Errorf("expected 0 flights (invalid times skipped), got %d", len(flights))
	}
}

func TestAirAsiaProvider_NormalizeSkipsWhenArrivalBeforeDeparture(t *testing.T) {
	p := providers.NewAirAsia()
	raw := []byte(`{"status":"ok","flights":[
		{"flight_code":"QZ998","airline":"AirAsia","from_airport":"CGK","to_airport":"DPS",
		 "depart_time":"2025-12-15T10:00:00+07:00","arrive_time":"2025-12-15T09:00:00+07:00",
		 "direct_flight":true,"price_idr":500000,"seats":50,"cabin_class":"economy","baggage_note":""}
	]}`)
	flights, err := p.Normalize(raw, dummyReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(flights) != 0 {
		t.Errorf("expected 0 flights (arrival before departure), got %d", len(flights))
	}
}

// ─── Garuda ──────────────────────────────────────────────────────────────────

func TestGarudaProvider_Name(t *testing.T) {
	p := providers.NewGaruda()
	if p.Name() != "Garuda" {
		t.Errorf("expected Name()=Garuda, got %q", p.Name())
	}
}

func TestGarudaProvider_FetchAndNormalize(t *testing.T) {
	p := providers.NewGaruda()
	flights := fetchAndNormalize(t, p)

	if len(flights) == 0 {
		t.Fatal("Garuda: expected at least 1 flight")
	}
	for i, f := range flights {
		assertFlight(t, f, "Garuda["+string(rune('0'+i))+"]")
		if f.Provider != "Garuda" {
			t.Errorf("Garuda: flight Provider = %q, want Garuda", f.Provider)
		}
		if f.Airline.Code != "GA" {
			t.Errorf("Garuda: airline code = %q, want GA", f.Airline.Code)
		}
	}
}

func TestGarudaProvider_FetchRespectsContextCancellation(t *testing.T) {
	p := providers.NewGaruda()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.Fetch(ctx, dummyReq)
	if err == nil {
		t.Error("Garuda Fetch: expected error on cancelled context, got nil")
	}
}

func TestGarudaProvider_NormalizeInvalidJSON(t *testing.T) {
	p := providers.NewGaruda()
	_, err := p.Normalize([]byte("{bad json"), dummyReq)
	if err == nil {
		t.Error("Garuda Normalize: expected error for invalid JSON, got nil")
	}
}

func TestGarudaProvider_NormalizePreservesAircraft(t *testing.T) {
	p := providers.NewGaruda()
	flights := fetchAndNormalize(t, p)

	hasAircraft := false
	for _, f := range flights {
		if f.Aircraft != nil && *f.Aircraft != "" {
			hasAircraft = true
			break
		}
	}
	if !hasAircraft {
		t.Error("Garuda: expected at least one flight with Aircraft set")
	}
}

func TestGarudaProvider_NormalizeSegmentStops(t *testing.T) {
	p := providers.NewGaruda()
	raw := []byte(`{"status":"success","flights":[{
		"flight_id":"GA315","airline":"Garuda Indonesia","airline_code":"GA",
		"departure":{"airport":"CGK","city":"Jakarta","time":"2025-12-15T14:00:00+07:00","terminal":"3"},
		"arrival":{"airport":"DPS","city":"Denpasar","time":"2025-12-15T18:45:00+08:00","terminal":"I"},
		"duration_minutes":285,"stops":1,"aircraft":"Boeing 737",
		"price":{"amount":1850000,"currency":"IDR"},
		"available_seats":22,"fare_class":"economy",
		"baggage":{"carry_on":1,"checked":2},
		"segments":[
			{"flight_number":"GA315","departure":{"airport":"CGK","time":"2025-12-15T14:00:00+07:00"},"arrival":{"airport":"SUB","time":"2025-12-15T15:30:00+07:00"},"duration_minutes":90},
			{"flight_number":"GA332","departure":{"airport":"SUB","time":"2025-12-15T17:15:00+07:00"},"arrival":{"airport":"DPS","time":"2025-12-15T18:45:00+08:00"},"duration_minutes":90,"layover_minutes":105}
		]
	}]}`)
	flights, err := p.Normalize(raw, dummyReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(flights) != 1 {
		t.Fatalf("expected 1 flight, got %d", len(flights))
	}
	// 2 segments → 1 stop (len-1)
	if flights[0].Stops != 1 {
		t.Errorf("expected Stops=1 (from 2 segments), got %d", flights[0].Stops)
	}
}

// ─── Batik ───────────────────────────────────────────────────────────────────

func TestBatikProvider_Name(t *testing.T) {
	p := providers.NewBatik()
	if p.Name() != "Batik" {
		t.Errorf("expected Name()=Batik, got %q", p.Name())
	}
}

func TestBatikProvider_FetchAndNormalize(t *testing.T) {
	p := providers.NewBatik()
	flights := fetchAndNormalize(t, p)

	if len(flights) == 0 {
		t.Fatal("Batik: expected at least 1 flight")
	}
	for i, f := range flights {
		assertFlight(t, f, "Batik["+string(rune('0'+i))+"]")
		if f.Provider != "Batik" {
			t.Errorf("Batik: flight Provider = %q, want Batik", f.Provider)
		}
		if f.Airline.Code != "ID" {
			t.Errorf("Batik: airline code = %q, want ID", f.Airline.Code)
		}
	}
}

func TestBatikProvider_FetchRespectsContextCancellation(t *testing.T) {
	p := providers.NewBatik()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.Fetch(ctx, dummyReq)
	if err == nil {
		t.Error("Batik Fetch: expected error on cancelled context, got nil")
	}
}

func TestBatikProvider_NormalizeInvalidJSON(t *testing.T) {
	p := providers.NewBatik()
	_, err := p.Normalize([]byte("!!!"), dummyReq)
	if err == nil {
		t.Error("Batik Normalize: expected error for invalid JSON, got nil")
	}
}

func TestBatikProvider_NormalizeParsesNonStandardDatetime(t *testing.T) {
	p := providers.NewBatik()
	// Batik menggunakan +0700 (tanpa titik dua)
	raw := []byte(`{"code":200,"message":"OK","results":[{
		"flightNumber":"ID9999","airlineName":"Batik Air","airlineIATA":"ID",
		"origin":"CGK","destination":"DPS",
		"departureDateTime":"2025-12-15T07:15:00+0700",
		"arrivalDateTime":"2025-12-15T10:00:00+0800",
		"travelTime":"1h 45m","numberOfStops":0,
		"fare":{"basePrice":980000,"taxes":120000,"totalPrice":1100000,"currencyCode":"IDR","class":"Y"},
		"seatsAvailable":32,"aircraftModel":"Airbus A320","baggageInfo":"7kg cabin, 20kg checked",
		"onboardServices":["Snack","Beverage"]
	}]}`)

	flights, err := p.Normalize(raw, dummyReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(flights) != 1 {
		t.Fatalf("expected 1 flight, got %d", len(flights))
	}
	f := flights[0]
	if f.Baggage.CarryOn != "7kg cabin" {
		t.Errorf("CarryOn = %q, want %q", f.Baggage.CarryOn, "7kg cabin")
	}
	if f.Baggage.Checked != "20kg checked" {
		t.Errorf("Checked = %q, want %q", f.Baggage.Checked, "20kg checked")
	}
	if len(f.Amenities) != 2 {
		t.Errorf("expected 2 amenities, got %d", len(f.Amenities))
	}
	// Amenities harus lowercase
	for _, a := range f.Amenities {
		if a != "snack" && a != "beverage" {
			t.Errorf("unexpected amenity %q (should be lowercase)", a)
		}
	}
}

func TestBatikProvider_NormalizeUsesTotalPrice(t *testing.T) {
	p := providers.NewBatik()
	raw := []byte(`{"code":200,"message":"OK","results":[{
		"flightNumber":"ID0001","airlineName":"Batik Air","airlineIATA":"ID",
		"origin":"CGK","destination":"DPS",
		"departureDateTime":"2025-12-15T07:15:00+0700",
		"arrivalDateTime":"2025-12-15T10:00:00+0800",
		"travelTime":"1h 45m","numberOfStops":0,
		"fare":{"basePrice":900000,"taxes":100000,"totalPrice":1000000,"currencyCode":"IDR","class":"Y"},
		"seatsAvailable":10,"aircraftModel":"","baggageInfo":"","onboardServices":[]
	}]}`)

	flights, err := p.Normalize(raw, dummyReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if flights[0].Price.Amount != 1000000 {
		t.Errorf("expected totalPrice=1000000, got %d", flights[0].Price.Amount)
	}
}

// ─── Lion ────────────────────────────────────────────────────────────────────

func TestLionProvider_Name(t *testing.T) {
	p := providers.NewLion()
	if p.Name() != "Lion" {
		t.Errorf("expected Name()=Lion, got %q", p.Name())
	}
}

func TestLionProvider_FetchAndNormalize(t *testing.T) {
	p := providers.NewLion()
	flights := fetchAndNormalize(t, p)

	if len(flights) == 0 {
		t.Fatal("Lion: expected at least 1 flight")
	}
	for i, f := range flights {
		assertFlight(t, f, "Lion["+string(rune('0'+i))+"]")
		if f.Provider != "Lion" {
			t.Errorf("Lion: flight Provider = %q, want Lion", f.Provider)
		}
		if f.Airline.Code != "JT" {
			t.Errorf("Lion: airline code = %q, want JT", f.Airline.Code)
		}
	}
}

func TestLionProvider_FetchRespectsContextCancellation(t *testing.T) {
	p := providers.NewLion()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.Fetch(ctx, dummyReq)
	if err == nil {
		t.Error("Lion Fetch: expected error on cancelled context, got nil")
	}
}

func TestLionProvider_NormalizeInvalidJSON(t *testing.T) {
	p := providers.NewLion()
	_, err := p.Normalize([]byte("garbage"), dummyReq)
	if err == nil {
		t.Error("Lion Normalize: expected error for invalid JSON, got nil")
	}
}

func TestLionProvider_NormalizeFailsWhenSuccessFalse(t *testing.T) {
	p := providers.NewLion()
	raw := []byte(`{"success":false,"data":{"available_flights":[]}}`)
	_, err := p.Normalize(raw, dummyReq)
	if err == nil {
		t.Error("Lion Normalize: expected error when success=false, got nil")
	}
}

func TestLionProvider_NormalizeParsesTimezoneOffset(t *testing.T) {
	p := providers.NewLion()
	// Lion menyimpan datetime tanpa offset, timezone disimpan terpisah
	raw := []byte(`{"success":true,"data":{"available_flights":[{
		"id":"JT999",
		"carrier":{"name":"Lion Air","iata":"JT"},
		"route":{"from":{"code":"CGK","name":"Soekarno-Hatta","city":"Jakarta"},"to":{"code":"DPS","name":"Ngurah Rai","city":"Denpasar"}},
		"schedule":{"departure":"2025-12-15T05:30:00","departure_timezone":"Asia/Jakarta","arrival":"2025-12-15T08:15:00","arrival_timezone":"Asia/Makassar"},
		"flight_time":105,"is_direct":true,
		"pricing":{"total":950000,"currency":"IDR","fare_type":"ECONOMY"},
		"seats_left":45,"plane_type":"Boeing 737-900ER",
		"services":{"wifi_available":false,"meals_included":false,"baggage_allowance":{"cabin":"7 kg","hold":"20 kg"}}
	}]}}`)

	flights, err := p.Normalize(raw, dummyReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(flights) != 1 {
		t.Fatalf("expected 1 flight, got %d", len(flights))
	}
	f := flights[0]
	if f.Stops != 0 {
		t.Errorf("expected 0 stops (direct), got %d", f.Stops)
	}
	if f.Duration.TotalMinutes != 105 {
		t.Errorf("expected flight_time=105, got %d", f.Duration.TotalMinutes)
	}
	if f.Baggage.CarryOn != "7 kg" {
		t.Errorf("CarryOn = %q, want %q", f.Baggage.CarryOn, "7 kg")
	}
	if f.Baggage.Checked != "20 kg" {
		t.Errorf("Checked = %q, want %q", f.Baggage.Checked, "20 kg")
	}
}

func TestLionProvider_NormalizeStopsFromLayovers(t *testing.T) {
	p := providers.NewLion()
	raw := []byte(`{"success":true,"data":{"available_flights":[{
		"id":"JT650",
		"carrier":{"name":"Lion Air","iata":"JT"},
		"route":{"from":{"code":"CGK","name":"Soekarno-Hatta","city":"Jakarta"},"to":{"code":"DPS","name":"Ngurah Rai","city":"Denpasar"}},
		"schedule":{"departure":"2025-12-15T16:20:00","departure_timezone":"Asia/Jakarta","arrival":"2025-12-15T21:10:00","arrival_timezone":"Asia/Makassar"},
		"flight_time":230,"is_direct":false,"stop_count":1,
		"layovers":[{"airport":"SUB","duration_minutes":75}],
		"pricing":{"total":780000,"currency":"IDR","fare_type":"ECONOMY"},
		"seats_left":52,"plane_type":"Boeing 737-800",
		"services":{"wifi_available":false,"meals_included":false,"baggage_allowance":{"cabin":"7 kg","hold":"20 kg"}}
	}]}}`)

	flights, err := p.Normalize(raw, dummyReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(flights) != 1 {
		t.Fatalf("expected 1 flight, got %d", len(flights))
	}
	if flights[0].Stops != 1 {
		t.Errorf("expected Stops=1, got %d", flights[0].Stops)
	}
}
