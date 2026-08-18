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

type lionRaw struct {
	Success bool `json:"success"`
	Data    struct {
		AvailableFlights []struct {
			ID      string `json:"id"`
			Carrier struct {
				Name string `json:"name"`
				IATA string `json:"iata"`
			} `json:"carrier"`
			Route struct {
				From struct {
					Code string `json:"code"`
					Name string `json:"name"`
					City string `json:"city"`
				} `json:"from"`
				To struct {
					Code string `json:"code"`
					Name string `json:"name"`
					City string `json:"city"`
				} `json:"to"`
			} `json:"route"`
			Schedule struct {
				Departure         string `json:"departure"`
				DepartureTimezone string `json:"departure_timezone"`
				Arrival           string `json:"arrival"`
				ArrivalTimezone   string `json:"arrival_timezone"`
			} `json:"schedule"`
			FlightTime int  `json:"flight_time"`
			IsDirect   bool `json:"is_direct"`
			StopCount  int  `json:"stop_count"`
			Layovers   []struct {
				Airport         string `json:"airport"`
				DurationMinutes int    `json:"duration_minutes"`
			} `json:"layovers"`
			Pricing struct {
				Total    int64  `json:"total"`
				Currency string `json:"currency"`
				FareType string `json:"fare_type"`
			} `json:"pricing"`
			SeatsLeft int    `json:"seats_left"`
			PlaneType string `json:"plane_type"`
			Services  struct {
				WifiAvailable    bool `json:"wifi_available"`
				MealsIncluded    bool `json:"meals_included"`
				BaggageAllowance struct {
					Cabin string `json:"cabin"`
					Hold  string `json:"hold"`
				} `json:"baggage_allowance"`
			} `json:"services"`
		} `json:"available_flights"`
	} `json:"data"`
}

// tzOffsets memetakan nama timezone IANA yang umum digunakan Lion Air ke UTC offset
var tzOffsets = map[string]string{
	"Asia/Jakarta":   "+07:00",
	"Asia/Makassar":  "+08:00",
	"Asia/Jayapura":  "+09:00",
	"Asia/Pontianak": "+07:00",
}

// parseLionDateTime menggabungkan datetime string tanpa offset ("2025-12-15T05:30:00")
// dengan nama timezone IANA, lalu menghasilkan time.Time yang benar.
func parseLionDateTime(datetimeStr, timezone string) (time.Time, error) {
	// Coba parse langsung sebagai RFC3339 (jika sudah ada offset)
	t, err := time.Parse(time.RFC3339, datetimeStr)
	if err == nil {
		return t, nil
	}

	// Cari offset dari map timezone
	offset, ok := tzOffsets[timezone]
	if !ok {
		// Fallback ke WIB jika timezone tidak dikenal
		offset = "+07:00"
	}

	// Gabungkan datetime dengan offset → RFC3339
	normalized := datetimeStr + offset
	return time.Parse(time.RFC3339, normalized)
}

// buildLionAmenities menyusun daftar amenities dari flag services Lion Air
func buildLionAmenities(wifi, meals bool) []string {
	amenities := []string{}
	if wifi {
		amenities = append(amenities, "wifi")
	}
	if meals {
		amenities = append(amenities, "meal")
	}
	return amenities
}

//go:embed data/lion_air_search_response.json
var lionData []byte

type lionProvider struct{}

// Fetch implements [Provider].
func (l *lionProvider) Fetch(ctx context.Context, req models.SearchRequest) (FetchResult, error) {
	delay := time.Duration(100+rand.Intn(101)) * time.Millisecond // 100-200ms

	select {
	case <-time.After(delay):
		return FetchResult{Data: lionData}, nil
	case <-ctx.Done():
		return FetchResult{}, ctx.Err()
	}
}

// Name implements [Provider].
func (l *lionProvider) Name() string {
	return "Lion"
}

// Normalize implements [Provider].
func (l *lionProvider) Normalize(raw []byte, req models.SearchRequest) ([]models.Flight, error) {
	var parsed lionRaw
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("lion: unmarshal failed: %w", err)
	}

	if !parsed.Success {
		return nil, fmt.Errorf("lion: response indicates failure")
	}

	var out []models.Flight
	for _, f := range parsed.Data.AvailableFlights {
		dep, err := parseLionDateTime(f.Schedule.Departure, f.Schedule.DepartureTimezone)
		if err != nil {
			continue
		}

		arr, err := parseLionDateTime(f.Schedule.Arrival, f.Schedule.ArrivalTimezone)
		if err != nil {
			continue
		}

		if !arr.After(dep) {
			continue
		}

		// Gunakan flight_time dari data jika tersedia, fallback ke kalkulasi
		durMin := f.FlightTime
		if durMin == 0 {
			durMin = int(arr.Sub(dep).Minutes())
		}

		// Stops: jika bukan direct, gunakan stop_count atau jumlah layovers
		stops := 0
		if !f.IsDirect {
			stops = f.StopCount
			if stops == 0 {
				stops = len(f.Layovers)
			}
			if stops == 0 {
				stops = 1 // minimal 1 stop jika memang bukan direct
			}
		}

		planeType := f.PlaneType
		var aircraftPtr *string
		if planeType != "" {
			aircraftPtr = &planeType
		}

		amenities := buildLionAmenities(f.Services.WifiAvailable, f.Services.MealsIncluded)

		formatted := utils.FormatIDR(f.Pricing.Total)
		if f.Pricing.Currency != "IDR" {
			formatted = fmt.Sprintf("%s %d", f.Pricing.Currency, f.Pricing.Total)
		}

		out = append(out, models.Flight{
			ID:           fmt.Sprintf("%s_Lion", f.ID),
			Provider:     "Lion",
			Airline:      models.Airline{Name: "Lion Air", Code: "JT"},
			FlightNumber: f.ID,
			Departure: models.Point{
				Airport:   f.Route.From.Code,
				City:      utils.CityFromAirport(f.Route.From.Code),
				DateTime:  dep.Format(time.RFC3339),
				Timestamp: dep.Unix(),
			},
			Arrival: models.Point{
				Airport:   f.Route.To.Code,
				City:      utils.CityFromAirport(f.Route.To.Code),
				DateTime:  arr.Format(time.RFC3339),
				Timestamp: arr.Unix(),
			},
			Duration:       models.Duration{TotalMinutes: durMin, Formatted: utils.FormatDuration(durMin)},
			Stops:          stops,
			Price:          models.Price{Amount: f.Pricing.Total, Currency: f.Pricing.Currency, Formatted: formatted},
			AvailableSeats: f.SeatsLeft,
			CabinClass:     "economy",
			Aircraft:       aircraftPtr,
			Amenities:      amenities,
			Baggage: models.Baggage{
				CarryOn: f.Services.BaggageAllowance.Cabin,
				Checked: f.Services.BaggageAllowance.Hold,
			},
		})
	}

	return out, nil
}

func NewLion() Provider { return &lionProvider{} }
