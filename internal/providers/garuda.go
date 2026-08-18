package providers

import (
	"app/internal/models"
	"app/internal/utils"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

type garudaRaw struct {
	Status  string `json:"status"`
	Flights []struct {
		FlightID    string `json:"flight_id"`
		Airline     string `json:"airline"`
		AirlineCode string `json:"airline_code"`
		Departure   struct {
			Airport  string `json:"airport"`
			City     string `json:"city"`
			Time     string `json:"time"`
			Terminal string `json:"terminal"`
		} `json:"departure"`
		Arrival struct {
			Airport  string `json:"airport"`
			City     string `json:"city"`
			Time     string `json:"time"`
			Terminal string `json:"terminal"`
		} `json:"arrival"`
		DurationMinutes int    `json:"duration_minutes"`
		Stops           int    `json:"stops"`
		Aircraft        string `json:"aircraft"`
		Price           struct {
			Amount   int64  `json:"amount"`
			Currency string `json:"currency"`
		} `json:"price"`
		AvailableSeats int    `json:"available_seats"`
		FareClass      string `json:"fare_class"`
		Baggage        struct {
			CarryOn int `json:"carry_on"`
			Checked int `json:"checked"`
		} `json:"baggage"`
		Amenities []string `json:"amenities"`
		Segments  []struct {
			FlightNumber string `json:"flight_number"`
			Departure    struct {
				Airport string `json:"airport"`
				Time    string `json:"time"`
			} `json:"departure"`
			Arrival struct {
				Airport string `json:"airport"`
				Time    string `json:"time"`
			} `json:"arrival"`
			DurationMinutes int `json:"duration_minutes"`
			LayoverMinutes  int `json:"layover_minutes"`
		} `json:"segments"`
	} `json:"flights"`
}

//go:embed data/garuda_indonesia_search_response.json
var garudaData []byte

type garudaProvider struct{}

// Fetch implements [Provider].
func (g *garudaProvider) Fetch(ctx context.Context, req models.SearchRequest) (FetchResult, error) {
	delay := time.Duration(50+rand.Intn(51)) * time.Millisecond // 50-100ms

	select {
	case <-time.After(delay):
		return FetchResult{Data: garudaData}, nil
	case <-ctx.Done():
		return FetchResult{}, ctx.Err()
	}
}

// Name implements [Provider].
func (g *garudaProvider) Name() string {
	return "Garuda"
}

// Normalize implements [Provider].
func (g *garudaProvider) Normalize(raw []byte, req models.SearchRequest) ([]models.Flight, error) {
	var parsed garudaRaw
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("garuda: unmarshal failed: %w", err)
	}

	var out []models.Flight
	for _, f := range parsed.Flights {
		dep, err := time.Parse(time.RFC3339, f.Departure.Time)
		if err != nil {
			continue
		}

		arr, err := time.Parse(time.RFC3339, f.Arrival.Time)
		if err != nil {
			continue
		}

		if !arr.After(dep) {
			continue
		}

		// Hitung durasi dari timestamp jika duration_minutes tidak tersedia
		durMin := f.DurationMinutes
		if durMin == 0 {
			durMin = int(arr.Sub(dep).Minutes())
		}

		// Tentukan jumlah stops dari segments jika ada
		stops := f.Stops
		if len(f.Segments) > 1 {
			stops = len(f.Segments) - 1
		}

		aircraft := f.Aircraft
		var aircraftPtr *string
		if aircraft != "" {
			aircraftPtr = &aircraft
		}

		amenities := f.Amenities
		if amenities == nil {
			amenities = []string{}
		}

		// Format baggage: Garuda menyimpan count, konversi ke string deskriptif
		carryOnStr := fmt.Sprintf("%d pcs cabin baggage", f.Baggage.CarryOn)
		checkedStr := fmt.Sprintf("%d pcs checked baggage", f.Baggage.Checked)
		if f.Baggage.CarryOn == 0 {
			carryOnStr = "-"
		}
		if f.Baggage.Checked == 0 {
			checkedStr = "-"
		}

		formatted := utils.FormatIDR(f.Price.Amount)
		if f.Price.Currency != "IDR" {
			formatted = fmt.Sprintf("%s %d", f.Price.Currency, f.Price.Amount)
		}

		out = append(out, models.Flight{
			ID:           fmt.Sprintf("%s_Garuda", f.FlightID),
			Provider:     "Garuda",
			Airline:      models.Airline{Name: "Garuda Indonesia", Code: "GA"},
			FlightNumber: f.FlightID,
			Departure: models.Point{
				Airport:   f.Departure.Airport,
				City:      f.Departure.City,
				DateTime:  dep.Format(time.RFC3339),
				Timestamp: dep.Unix(),
			},
			Arrival: models.Point{
				Airport:   f.Arrival.Airport,
				City:      f.Arrival.City,
				DateTime:  arr.Format(time.RFC3339),
				Timestamp: arr.Unix(),
			},
			Duration:       models.Duration{TotalMinutes: durMin, Formatted: utils.FormatDuration(durMin)},
			Stops:          stops,
			Price:          models.Price{Amount: f.Price.Amount, Currency: f.Price.Currency, Formatted: formatted},
			AvailableSeats: f.AvailableSeats,
			CabinClass:     f.FareClass,
			Aircraft:       aircraftPtr,
			Amenities:      amenities,
			Baggage: models.Baggage{
				CarryOn: carryOnStr,
				Checked: checkedStr,
			},
		})
	}

	return out, nil
}

func NewGaruda() Provider { return &garudaProvider{} }
