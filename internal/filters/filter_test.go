package filters_test

import (
	"app/internal/filters"
	"app/internal/models"
	"testing"
	"time"
)

// helper untuk membuat flight sederhana
func makeFlight(id string, price int64, stops int, durationMin int, airline string, depTS int64, arrTS int64) models.Flight {
	return models.Flight{
		ID:       id,
		Provider: "Test",
		Airline:  models.Airline{Name: airline, Code: "XX"},
		Price:    models.Price{Amount: price, Currency: "IDR"},
		Stops:    stops,
		Duration: models.Duration{TotalMinutes: durationMin},
		Departure: models.Point{
			Airport:   "CGK",
			City:      "Jakarta",
			Timestamp: depTS,
		},
		Arrival: models.Point{
			Airport:   "DPS",
			City:      "Denpasar",
			Timestamp: arrTS,
		},
		CabinClass:     "economy",
		AvailableSeats: 10,
		Amenities:      []string{},
		Baggage:        models.Baggage{CarryOn: "-", Checked: "-"},
	}
}

// ─── PriceRange ──────────────────────────────────────────────────────────────

func TestPriceRangeFilter(t *testing.T) {
	flights := []models.Flight{
		makeFlight("F1", 500000, 0, 90, "TestAir", 0, 0),
		makeFlight("F2", 1000000, 0, 90, "TestAir", 0, 0),
		makeFlight("F3", 1500000, 0, 90, "TestAir", 0, 0),
	}

	t.Run("filters below min", func(t *testing.T) {
		f := filters.PriceRange{Min: 700000, Max: 0}
		got := f.Apply(flights)
		if len(got) != 2 {
			t.Errorf("expected 2 flights, got %d", len(got))
		}
	})

	t.Run("filters above max", func(t *testing.T) {
		f := filters.PriceRange{Min: 0, Max: 1000000}
		got := f.Apply(flights)
		if len(got) != 2 {
			t.Errorf("expected 2 flights, got %d", len(got))
		}
	})

	t.Run("filters within range", func(t *testing.T) {
		f := filters.PriceRange{Min: 600000, Max: 1200000}
		got := f.Apply(flights)
		if len(got) != 1 {
			t.Errorf("expected 1 flight, got %d", len(got))
		}
		if got[0].ID != "F2" {
			t.Errorf("expected F2, got %s", got[0].ID)
		}
	})

	t.Run("max 0 means no upper limit", func(t *testing.T) {
		f := filters.PriceRange{Min: 0, Max: 0}
		got := f.Apply(flights)
		if len(got) != 3 {
			t.Errorf("expected 3 flights, got %d", len(got))
		}
	})

	t.Run("empty input returns empty", func(t *testing.T) {
		f := filters.PriceRange{Min: 0, Max: 2000000}
		got := f.Apply(nil)
		if len(got) != 0 {
			t.Errorf("expected 0 flights, got %d", len(got))
		}
	})
}

// ─── MaxStops ────────────────────────────────────────────────────────────────

func TestMaxStopsFilter(t *testing.T) {
	flights := []models.Flight{
		makeFlight("Direct", 1000000, 0, 90, "TestAir", 0, 0),
		makeFlight("OneStop", 800000, 1, 180, "TestAir", 0, 0),
		makeFlight("TwoStop", 600000, 2, 300, "TestAir", 0, 0),
	}

	t.Run("max 0 allows only direct", func(t *testing.T) {
		f := filters.MaxStops{Max: 0}
		got := f.Apply(flights)
		if len(got) != 1 || got[0].ID != "Direct" {
			t.Errorf("expected only Direct flight, got %d flights", len(got))
		}
	})

	t.Run("max 1 allows direct and one stop", func(t *testing.T) {
		f := filters.MaxStops{Max: 1}
		got := f.Apply(flights)
		if len(got) != 2 {
			t.Errorf("expected 2 flights, got %d", len(got))
		}
	})

	t.Run("max 10 allows all", func(t *testing.T) {
		f := filters.MaxStops{Max: 10}
		got := f.Apply(flights)
		if len(got) != 3 {
			t.Errorf("expected 3 flights, got %d", len(got))
		}
	})
}

// ─── Airlines ────────────────────────────────────────────────────────────────

func TestAirlinesFilter(t *testing.T) {
	flights := []models.Flight{
		makeFlight("GA1", 1000000, 0, 90, "Garuda Indonesia", 0, 0),
		makeFlight("JT1", 800000, 0, 90, "Lion Air", 0, 0),
		makeFlight("QZ1", 600000, 0, 90, "AirAsia", 0, 0),
	}

	t.Run("empty filter returns all", func(t *testing.T) {
		f := filters.Airlines{AirlineName: nil}
		got := f.Apply(flights)
		if len(got) != 3 {
			t.Errorf("expected 3 flights, got %d", len(got))
		}
	})

	t.Run("filter by one airline", func(t *testing.T) {
		f := filters.Airlines{AirlineName: []string{"Lion Air"}}
		got := f.Apply(flights)
		if len(got) != 1 || got[0].ID != "JT1" {
			t.Errorf("expected JT1, got %v", got)
		}
	})

	t.Run("filter by multiple airlines", func(t *testing.T) {
		f := filters.Airlines{AirlineName: []string{"Garuda Indonesia", "AirAsia"}}
		got := f.Apply(flights)
		if len(got) != 2 {
			t.Errorf("expected 2 flights, got %d", len(got))
		}
	})

	t.Run("case insensitive match", func(t *testing.T) {
		f := filters.Airlines{AirlineName: []string{"airasia"}}
		got := f.Apply(flights)
		if len(got) != 1 || got[0].ID != "QZ1" {
			t.Errorf("expected QZ1, got %v", got)
		}
	})

	t.Run("no match returns empty", func(t *testing.T) {
		f := filters.Airlines{AirlineName: []string{"Citilink"}}
		got := f.Apply(flights)
		if len(got) != 0 {
			t.Errorf("expected 0 flights, got %d", len(got))
		}
	})
}

// ─── MaxDuration ─────────────────────────────────────────────────────────────

func TestMaxDurationFilter(t *testing.T) {
	flights := []models.Flight{
		makeFlight("Short", 1000000, 0, 90, "TestAir", 0, 0),
		makeFlight("Med", 800000, 0, 150, "TestAir", 0, 0),
		makeFlight("Long", 600000, 1, 300, "TestAir", 0, 0),
	}

	t.Run("duration 0 means no limit", func(t *testing.T) {
		f := filters.MaxDuration{Duration: 0}
		got := f.Apply(flights)
		if len(got) != 3 {
			t.Errorf("expected 3 flights, got %d", len(got))
		}
	})

	t.Run("filters flights exceeding max", func(t *testing.T) {
		f := filters.MaxDuration{Duration: 150}
		got := f.Apply(flights)
		if len(got) != 2 {
			t.Errorf("expected 2 flights, got %d", len(got))
		}
	})

	t.Run("tight limit allows only short", func(t *testing.T) {
		f := filters.MaxDuration{Duration: 90}
		got := f.Apply(flights)
		if len(got) != 1 || got[0].ID != "Short" {
			t.Errorf("expected only Short, got %d flights", len(got))
		}
	})
}

// ─── DepartureTime ───────────────────────────────────────────────────────────

func TestDepartureTimeFilter(t *testing.T) {
	base := time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC)
	morning := base.Add(6 * time.Hour)   // 06:00
	noon := base.Add(12 * time.Hour)     // 12:00
	evening := base.Add(18 * time.Hour)  // 18:00

	flights := []models.Flight{
		makeFlight("Morning", 1000000, 0, 90, "TestAir", morning.Unix(), morning.Add(90*time.Minute).Unix()),
		makeFlight("Noon", 1000000, 0, 90, "TestAir", noon.Unix(), noon.Add(90*time.Minute).Unix()),
		makeFlight("Evening", 1000000, 0, 90, "TestAir", evening.Unix(), evening.Add(90*time.Minute).Unix()),
	}

	t.Run("after filter", func(t *testing.T) {
		f := filters.DepartureTime{After: noon}
		got := f.Apply(flights)
		if len(got) != 2 {
			t.Errorf("expected 2 flights (noon+evening), got %d", len(got))
		}
	})

	t.Run("before filter", func(t *testing.T) {
		f := filters.DepartureTime{Before: noon}
		got := f.Apply(flights)
		if len(got) != 2 {
			t.Errorf("expected 2 flights (morning+noon), got %d", len(got))
		}
	})

	t.Run("after and before", func(t *testing.T) {
		f := filters.DepartureTime{After: morning.Add(time.Second), Before: evening.Add(-time.Second)}
		got := f.Apply(flights)
		if len(got) != 1 || got[0].ID != "Noon" {
			t.Errorf("expected only Noon, got %v", got)
		}
	})

	t.Run("zero values pass all", func(t *testing.T) {
		f := filters.DepartureTime{}
		got := f.Apply(flights)
		if len(got) != 3 {
			t.Errorf("expected 3 flights, got %d", len(got))
		}
	})
}

// ─── Chain ───────────────────────────────────────────────────────────────────

func TestChain(t *testing.T) {
	flights := []models.Flight{
		makeFlight("F1", 500000, 0, 90, "Garuda Indonesia", 0, 0),
		makeFlight("F2", 1000000, 1, 150, "Lion Air", 0, 0),
		makeFlight("F3", 1500000, 2, 300, "Garuda Indonesia", 0, 0),
	}

	t.Run("chain applies all filters", func(t *testing.T) {
		chain := filters.NewChain(
			filters.PriceRange{Min: 0, Max: 1200000},
			filters.MaxStops{Max: 0},
		)
		got := chain.Apply(flights)
		// Hanya F1 yang lolos: harga <= 1.2jt DAN direct
		if len(got) != 1 || got[0].ID != "F1" {
			t.Errorf("expected F1, got %v", got)
		}
	})

	t.Run("empty chain returns all", func(t *testing.T) {
		chain := filters.NewChain()
		got := chain.Apply(flights)
		if len(got) != 3 {
			t.Errorf("expected 3 flights, got %d", len(got))
		}
	})

	t.Run("chain with no match returns empty", func(t *testing.T) {
		chain := filters.NewChain(
			filters.Airlines{AirlineName: []string{"AirAsia"}},
		)
		got := chain.Apply(flights)
		if len(got) != 0 {
			t.Errorf("expected 0 flights, got %d", len(got))
		}
	})
}
