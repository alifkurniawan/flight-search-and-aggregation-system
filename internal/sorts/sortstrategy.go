package sorts

import (
	"app/internal/models"
	"sort"
)

type SortStrategy interface {
	Sort(flights []models.Flight)
}

type byPriceAsc struct{}

func (byPriceAsc) Sort(flights []models.Flight) {
	sort.Slice(flights, func(i, j int) bool {
		return flights[i].Price.Amount < flights[j].Price.Amount
	})
}

type byPriceDesc struct{}

func (byPriceDesc) Sort(flights []models.Flight) {
	sort.Slice(flights, func(i, j int) bool {
		return flights[i].Price.Amount > flights[j].Price.Amount
	})
}

type byDurationAsc struct{}

func (byDurationAsc) Sort(flights []models.Flight) {
	sort.Slice(flights, func(i, j int) bool {
		return flights[i].Duration.TotalMinutes < flights[j].Duration.TotalMinutes
	})
}

type byDurationDesc struct{}

func (byDurationDesc) Sort(flights []models.Flight) {
	sort.Slice(flights, func(i, j int) bool {
		return flights[i].Duration.TotalMinutes > flights[j].Duration.TotalMinutes
	})
}

type departureAsc struct{}

func (departureAsc) Sort(f []models.Flight) {
	sort.SliceStable(f, func(i, j int) bool { return f[i].Departure.Timestamp < f[j].Departure.Timestamp })
}

type arrivalAsc struct{}

func (arrivalAsc) Sort(f []models.Flight) {
	sort.SliceStable(f, func(i, j int) bool { return f[i].Arrival.Timestamp < f[j].Arrival.Timestamp })
}

type byBestValue struct{}

func (byBestValue) Sort(flights []models.Flight) {
	ApplyBestValueScores(flights)
	sort.SliceStable(flights, func(i, j int) bool {
		return flights[i].BestValueScore > flights[j].BestValueScore
	})
}

var registry = map[string]SortStrategy{
	"price_asc":      byPriceAsc{},
	"price_desc":     byPriceDesc{},
	"duration_asc":   byDurationAsc{},
	"duration_desc":  byDurationDesc{},
	"departure_time": departureAsc{},
	"arrival_time":   arrivalAsc{},
	"best_value":     byBestValue{},
}

func GetStrategy(key string) SortStrategy {
	if s, ok := registry[key]; ok {
		return s
	}
	return byPriceAsc{}
}
