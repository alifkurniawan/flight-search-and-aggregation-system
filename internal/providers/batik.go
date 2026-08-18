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

type batikRaw struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Results []struct {
		FlightNumber      string `json:"flightNumber"`
		AirlineName       string `json:"airlineName"`
		AirlineIATA       string `json:"airlineIATA"`
		Origin            string `json:"origin"`
		Destination       string `json:"destination"`
		DepartureDateTime string `json:"departureDateTime"`
		ArrivalDateTime   string `json:"arrivalDateTime"`
		TravelTime        string `json:"travelTime"`
		NumberOfStops     int    `json:"numberOfStops"`
		Connections       []struct {
			StopAirport  string `json:"stopAirport"`
			StopDuration string `json:"stopDuration"`
		} `json:"connections"`
		Fare struct {
			BasePrice    int64  `json:"basePrice"`
			Taxes        int64  `json:"taxes"`
			TotalPrice   int64  `json:"totalPrice"`
			CurrencyCode string `json:"currencyCode"`
			Class        string `json:"class"`
		} `json:"fare"`
		SeatsAvailable  int      `json:"seatsAvailable"`
		AircraftModel   string   `json:"aircraftModel"`
		BaggageInfo     string   `json:"baggageInfo"`
		OnboardServices []string `json:"onboardServices"`
	} `json:"results"`
}

// parseBatikDateTime menangani format datetime Batik Air yang menggunakan offset tanpa titik dua
// Contoh: "2025-12-15T07:15:00+0700" → parse sebagai RFC3339 dengan titik dua: "+07:00"
func parseBatikDateTime(s string) (time.Time, error) {
	// Coba parse langsung dulu (jika sudah RFC3339 standar)
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t, nil
	}

	// Batik Air menggunakan "+0700" tanpa titik dua, konversi ke "+07:00"
	if len(s) > 5 {
		sign := s[len(s)-5]
		if sign == '+' || sign == '-' {
			normalized := s[:len(s)-5] + string(sign) + s[len(s)-4:len(s)-2] + ":" + s[len(s)-2:]
			return time.Parse(time.RFC3339, normalized)
		}
	}

	return time.Time{}, fmt.Errorf("batik: cannot parse datetime %q", s)
}

// parseBatikBaggage mengurai string baggageInfo Batik Air
// Contoh: "7kg cabin, 20kg checked" → CarryOn:"7kg cabin", Checked:"20kg checked"
func parseBatikBaggage(info string) models.Baggage {
	baggage := models.Baggage{CarryOn: "-", Checked: "-"}
	info = strings.TrimSpace(info)
	if info == "" {
		return baggage
	}

	parts := strings.SplitN(info, ",", 2)
	if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
		baggage.CarryOn = strings.TrimSpace(parts[0])
	}
	if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
		baggage.Checked = strings.TrimSpace(parts[1])
	}

	return baggage
}

// normalizeBatikServices mengkonversi onboardServices menjadi slice lowercase standar
func normalizeBatikServices(services []string) []string {
	out := make([]string, 0, len(services))
	for _, s := range services {
		out = append(out, strings.ToLower(s))
	}
	return out
}

//go:embed data/batik_air_search_response.json
var batikData []byte

type batikProvider struct{}

// Fetch implements [Provider].
func (b *batikProvider) Fetch(ctx context.Context, req models.SearchRequest) (FetchResult, error) {
	delay := time.Duration(200+rand.Intn(201)) * time.Millisecond // 200-400ms

	select {
	case <-time.After(delay):
		return FetchResult{Data: batikData}, nil
	case <-ctx.Done():
		return FetchResult{}, ctx.Err()
	}
}

// Name implements [Provider].
func (b *batikProvider) Name() string {
	return "Batik"
}

// Normalize implements [Provider].
func (b *batikProvider) Normalize(raw []byte, req models.SearchRequest) ([]models.Flight, error) {
	var parsed batikRaw
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("batik: unmarshal failed: %w", err)
	}

	var out []models.Flight
	for _, f := range parsed.Results {
		dep, err := parseBatikDateTime(f.DepartureDateTime)
		if err != nil {
			continue
		}

		arr, err := parseBatikDateTime(f.ArrivalDateTime)
		if err != nil {
			continue
		}

		if !arr.After(dep) {
			continue
		}

		durMin := int(arr.Sub(dep).Minutes())

		aircraft := f.AircraftModel
		var aircraftPtr *string
		if aircraft != "" {
			aircraftPtr = &aircraft
		}

		amenities := normalizeBatikServices(f.OnboardServices)
		formatted := utils.FormatIDR(f.Fare.TotalPrice)
		if f.Fare.CurrencyCode != "IDR" {
			formatted = fmt.Sprintf("%s %d", f.Fare.CurrencyCode, f.Fare.TotalPrice)
		}

		out = append(out, models.Flight{
			ID:           fmt.Sprintf("%s_Batik", f.FlightNumber),
			Provider:     "Batik",
			Airline:      models.Airline{Name: "Batik Air", Code: "ID"},
			FlightNumber: f.FlightNumber,
			Departure: models.Point{
				Airport:   f.Origin,
				City:      utils.CityFromAirport(f.Origin),
				DateTime:  dep.Format(time.RFC3339),
				Timestamp: dep.Unix(),
			},
			Arrival: models.Point{
				Airport:   f.Destination,
				City:      utils.CityFromAirport(f.Destination),
				DateTime:  arr.Format(time.RFC3339),
				Timestamp: arr.Unix(),
			},
			Duration:       models.Duration{TotalMinutes: durMin, Formatted: utils.FormatDuration(durMin)},
			Stops:          f.NumberOfStops,
			Price:          models.Price{Amount: f.Fare.TotalPrice, Currency: f.Fare.CurrencyCode, Formatted: formatted},
			AvailableSeats: f.SeatsAvailable,
			CabinClass:     strings.ToLower(f.Fare.Class),
			Aircraft:       aircraftPtr,
			Amenities:      amenities,
			Baggage:        parseBatikBaggage(f.BaggageInfo),
		})
	}

	return out, nil
}

func NewBatik() Provider { return &batikProvider{} }
