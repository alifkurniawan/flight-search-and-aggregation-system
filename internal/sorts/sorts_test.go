package sorts_test

import (
	"app/internal/models"
	"app/internal/sorts"
	"testing"
)

func makeFlightWithDuration(price int64, durationMin int) models.Flight {
	return models.Flight{
		Price:    models.Price{Amount: price, Currency: "IDR"},
		Duration: models.Duration{TotalMinutes: durationMin},
		Amenities: []string{},
		Baggage:   models.Baggage{},
	}
}

func TestGetStrategy_PriceAsc(t *testing.T) {
	flights := []models.Flight{
		makeFlightWithDuration(1500000, 90),
		makeFlightWithDuration(500000, 120),
		makeFlightWithDuration(1000000, 60),
	}

	sorts.GetStrategy("price_asc").Sort(flights)

	prices := []int64{flights[0].Price.Amount, flights[1].Price.Amount, flights[2].Price.Amount}
	expected := []int64{500000, 1000000, 1500000}
	for i, p := range prices {
		if p != expected[i] {
			t.Errorf("price_asc: index %d = %d, want %d", i, p, expected[i])
		}
	}
}

func TestGetStrategy_PriceDesc(t *testing.T) {
	flights := []models.Flight{
		makeFlightWithDuration(1500000, 90),
		makeFlightWithDuration(500000, 120),
		makeFlightWithDuration(1000000, 60),
	}

	sorts.GetStrategy("price_desc").Sort(flights)

	prices := []int64{flights[0].Price.Amount, flights[1].Price.Amount, flights[2].Price.Amount}
	expected := []int64{1500000, 1000000, 500000}
	for i, p := range prices {
		if p != expected[i] {
			t.Errorf("price_desc: index %d = %d, want %d", i, p, expected[i])
		}
	}
}

func TestGetStrategy_DurationAsc(t *testing.T) {
	flights := []models.Flight{
		makeFlightWithDuration(1000000, 300),
		makeFlightWithDuration(1000000, 90),
		makeFlightWithDuration(1000000, 150),
	}

	sorts.GetStrategy("duration_asc").Sort(flights)

	durations := []int{flights[0].Duration.TotalMinutes, flights[1].Duration.TotalMinutes, flights[2].Duration.TotalMinutes}
	expected := []int{90, 150, 300}
	for i, d := range durations {
		if d != expected[i] {
			t.Errorf("duration_asc: index %d = %d, want %d", i, d, expected[i])
		}
	}
}

func TestGetStrategy_UnknownKeyDefaultsToPriceAsc(t *testing.T) {
	flights := []models.Flight{
		makeFlightWithDuration(1500000, 90),
		makeFlightWithDuration(500000, 120),
	}

	// key yang tidak dikenal harus fallback ke price_asc
	sorts.GetStrategy("unknown_key").Sort(flights)

	if flights[0].Price.Amount != 500000 {
		t.Errorf("expected fallback to price_asc, first flight price = %d", flights[0].Price.Amount)
	}
}

func TestGetStrategy_EmptyKeyDefaultsToPriceAsc(t *testing.T) {
	flights := []models.Flight{
		makeFlightWithDuration(2000000, 90),
		makeFlightWithDuration(800000, 90),
	}

	sorts.GetStrategy("").Sort(flights)

	if flights[0].Price.Amount != 800000 {
		t.Errorf("expected fallback to price_asc, first flight price = %d", flights[0].Price.Amount)
	}
}

func TestGetStrategy_SingleFlight(t *testing.T) {
	flights := []models.Flight{
		makeFlightWithDuration(1000000, 90),
	}

	// Tidak boleh panic dengan 1 elemen
	sorts.GetStrategy("price_asc").Sort(flights)
	sorts.GetStrategy("price_desc").Sort(flights)
	sorts.GetStrategy("duration_asc").Sort(flights)
}

func TestGetStrategy_EmptySlice(t *testing.T) {
	var flights []models.Flight

	// Tidak boleh panic dengan slice kosong
	sorts.GetStrategy("price_asc").Sort(flights)
	sorts.GetStrategy("price_desc").Sort(flights)
	sorts.GetStrategy("duration_asc").Sort(flights)
}
