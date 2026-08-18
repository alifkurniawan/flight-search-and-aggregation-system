package providers

import (
	"app/internal/models"
	"app/internal/utils"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

type airasiaRaw struct {
	Status  string `json:"status"`
	Flights []struct {
		FlightCode   string `json:"flight_code"`
		Airline      string `json:"airline"`
		From         string `json:"from_airport"`
		To           string `json:"to_airport"`
		DepartTime   string `json:"depart_time"`
		ArriveTime   string `json:"arrive_time"`
		DirectFlight bool   `json:"direct_flight"`
		PriceIDR     int64  `json:"price_idr"`
		Seats        int    `json:"seats"`
		CabinClass   string `json:"cabin_class"`
		BaggageNote  string `json:"baggage_note"`
		Stops        []struct {
			Airport         string `json:"airport"`
			WaitTimeMinutes int    `json:"wait_time_minutes"`
		} `json:"stops"`
	} `json:"flights"`
}

func parseBaggage(note string) models.Baggage {
	baggage := models.Baggage{
		CarryOn: "-",
		Checked: "-",
	}

	note = strings.TrimSpace(note)
	if note == "" {
		return baggage
	}

	parts := strings.SplitN(note, ",", 2)

	if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
		baggage.CarryOn = strings.TrimSpace(parts[0])
	}

	if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
		baggage.Checked = strings.TrimSpace(parts[1])
	}

	return baggage
}

//go:embed data/airasia_search_response.json
var airasiaData []byte

type airAsiaProvider struct{}

// Fetch implements [Provider].
func (a *airAsiaProvider) Fetch(ctx context.Context, req models.SearchRequest) (FetchResult, error) {
	delay := time.Duration(50+rand.Intn(101)) * time.Millisecond // 50-150ms

	select {
	case <-time.After(delay):
		if rand.Float64() < 0.10 { // 10% gagal
			return FetchResult{}, fmt.Errorf("airasia: provider timeout/unavailable")
		}
		return FetchResult{Data: airasiaData}, nil
	case <-ctx.Done():
		return FetchResult{}, ctx.Err()
	}
}

// Name implements [Provider].
func (a *airAsiaProvider) Name() string {
	return "AirAsia"
}

// Normalize implements [Provider].
func (a *airAsiaProvider) Normalize(raw []byte, req models.SearchRequest) ([]models.Flight, error) {
	var parsed airasiaRaw
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("airasia: unmarshal failed: %w", err)
	}

	var out []models.Flight
	for _, f := range parsed.Flights {
		dep, err := time.Parse(time.RFC3339, f.DepartTime)
		if err != nil {
			continue
		}

		arr, err := time.Parse(time.RFC3339, f.ArriveTime)
		if err != nil {
			continue
		}

		if !arr.After(dep) {
			continue
		}

		stops := 0
		if !f.DirectFlight {
			stops = len(f.Stops)
			if stops == 0 {
				stops = 1
			}
		}

		durMin := int(arr.Sub(dep).Minutes())
		out = append(out, models.Flight{
			ID:           fmt.Sprintf("%s_AirAsia", f.FlightCode),
			Provider:     "AirAsia",
			Airline:      models.Airline{Name: "AirAsia", Code: "QZ"},
			FlightNumber: f.FlightCode,
			Departure: models.Point{
				Airport:   f.From,
				City:      utils.CityFromAirport(f.From),
				DateTime:  dep.Format(time.RFC3339),
				Timestamp: dep.Unix(),
			},
			Arrival: models.Point{
				Airport:   f.To,
				City:      utils.CityFromAirport(f.To),
				DateTime:  arr.Format(time.RFC3339),
				Timestamp: arr.Unix(),
			},
			Duration:       models.Duration{TotalMinutes: durMin, Formatted: utils.FormatDuration(durMin)},
			Stops:          stops,
			Price:          models.Price{Amount: f.PriceIDR, Currency: "IDR", Formatted: utils.FormatIDR(f.PriceIDR)},
			AvailableSeats: f.Seats,
			CabinClass:     f.CabinClass,
			Aircraft:       nil, // AirAsia tidak kasih data ini
			Amenities:      []string{},
			Baggage:        parseBaggage(f.BaggageNote),
		})

	}

	return out, nil
}

func NewAirAsia() Provider { return &airAsiaProvider{} }
