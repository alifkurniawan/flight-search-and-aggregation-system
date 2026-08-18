package aggregator

import (
	"app/internal/filters"
	"app/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type searchAPIRequest struct {
	models.SearchRequest
	MinPrice        int64    `json:"minPrice"`
	MaxPrice        int64    `json:"maxPrice"`
	MaxStops        int      `json:"maxStops"`
	Airlines        []string `json:"airlines"`
	MaxDuration     int      `json:"maxDurationMinutes"`
	DepartureAfter  string   `json:"departureAfter"`
	DepartureBefore string   `json:"departureBefore"`
	ArrivalAfter    string   `json:"arrivalAfter"`
	ArrivalBefore   string   `json:"arrivalBefore"`
	SortBy          string   `json:"sortBy"` // price_asc|price_desc|duration_asc|duration_desc|departure_time|arrival_time|best_value
}

func buildFilterChain(req searchAPIRequest) *filters.Chain {
	maxStops := req.MaxStops
	if maxStops <= 0 {
		maxStops = 10 // effectively unlimited if caller didn't ask for a cap
	}

	var depAfterTime, depBeforeTime, arrAfterTime, arrBeforeTime time.Time
	if req.DepartureAfter != "" {
		depAfterTime, _ = time.Parse(time.RFC3339, req.DepartureAfter)
	}
	if req.DepartureBefore != "" {
		depBeforeTime, _ = time.Parse(time.RFC3339, req.DepartureBefore)
	}
	if req.ArrivalAfter != "" {
		arrAfterTime, _ = time.Parse(time.RFC3339, req.ArrivalAfter)
	}
	if req.ArrivalBefore != "" {
		arrBeforeTime, _ = time.Parse(time.RFC3339, req.ArrivalBefore)
	}

	return filters.NewChain(
		filters.PriceRange{Min: req.MinPrice, Max: req.MaxPrice},
		filters.MaxStops{Max: maxStops},
		filters.Airlines{AirlineName: req.Airlines},
		filters.MaxDuration{Duration: req.MaxDuration},
		filters.DepartureTime{After: depAfterTime, Before: depBeforeTime},
		filters.ArrivalTime{After: arrAfterTime, Before: arrBeforeTime},
	)
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var apiReq searchAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&apiReq); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	if apiReq.Origin == "" || apiReq.Destination == "" || apiReq.DepartureDate == "" {
		http.Error(w, "origin, destination, and departureDate are required", http.StatusBadRequest)
		return
	}
	if apiReq.Passengers <= 0 {
		apiReq.Passengers = 1
	}
	if apiReq.CabinClass == "" {
		apiReq.CabinClass = "economy"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp := h.svc.Search(ctx, apiReq.SearchRequest, SearchOptions{
		Filters: buildFilterChain(apiReq),
		SortKey: apiReq.SortBy,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

}
