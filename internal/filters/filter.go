package filters

import (
	"app/internal/models"
	"strings"
	"time"
)

type Filter interface {
	Apply(flights []models.Flight) []models.Flight
}

type PriceRange struct {
	Min, Max int64
}

func (f PriceRange) Apply(flights []models.Flight) []models.Flight {
	var out []models.Flight
	for _, fl := range flights {
		if fl.Price.Amount < f.Min {
			continue
		}
		if f.Max > 0 && fl.Price.Amount > f.Max {
			continue
		}
		out = append(out, fl)
	}
	return out
}

type MaxStops struct {
	Max int
}

func (f MaxStops) Apply(flights []models.Flight) []models.Flight {
	var out []models.Flight
	for _, fl := range flights {
		if fl.Stops <= f.Max {
			out = append(out, fl)
		}
	}
	return out
}

type DepartureTime struct {
	After  time.Time
	Before time.Time
}

func (f DepartureTime) Apply(flights []models.Flight) []models.Flight {
	var out []models.Flight
	for _, fl := range flights {
		if !f.After.IsZero() && fl.Departure.Timestamp < f.After.Unix() {
			continue
		}

		if !f.Before.IsZero() && fl.Departure.Timestamp > f.Before.Unix() {
			continue
		}
		out = append(out, fl)
	}
	return out
}

type ArrivalTime struct {
	After  time.Time
	Before time.Time
}

func (f ArrivalTime) Apply(flights []models.Flight) []models.Flight {
	var out []models.Flight
	for _, fl := range flights {
		if !f.After.IsZero() && fl.Arrival.Timestamp < f.After.Unix() {
			continue
		}

		if !f.Before.IsZero() && fl.Arrival.Timestamp > f.Before.Unix() {
			continue
		}
		out = append(out, fl)
	}
	return out
}

type Airlines struct {
	AirlineName []string
}

func (f Airlines) Apply(flights []models.Flight) []models.Flight {
	if len(f.AirlineName) == 0 {
		return flights
	}
	allowed := make(map[string]bool, len(f.AirlineName))
	for _, a := range f.AirlineName {
		allowed[strings.ToLower(a)] = true
	}
	var out []models.Flight
	for _, fl := range flights {
		if allowed[strings.ToLower(fl.Airline.Name)] {
			out = append(out, fl)
		}
	}
	return out
}

type MaxDuration struct {
	Duration int
}

func (f MaxDuration) Apply(flights []models.Flight) []models.Flight {
	if f.Duration <= 0 {
		return flights
	}

	var out []models.Flight
	for _, fl := range flights {
		if fl.Duration.TotalMinutes <= f.Duration {
			out = append(out, fl)
		}
	}
	return out
}

type Chain struct {
	filters []Filter
}

func NewChain(filters ...Filter) *Chain {
	return &Chain{filters: filters}
}

func (c *Chain) Apply(flights []models.Flight) []models.Flight {
	result := flights
	for _, f := range c.filters {
		result = f.Apply(result)
	}
	return result
}
