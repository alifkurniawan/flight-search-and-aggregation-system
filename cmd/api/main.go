package main

import (
	"app/internal/aggregator"
	"app/internal/cache"
	"app/internal/filters"
	"app/internal/models"
	"app/internal/providers"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func runDemo(svc *aggregator.Service) {
	req := models.SearchRequest{
		Origin:        "CGK",
		Destination:   "DPS",
		DepartureDate: "2025-12-14",
		Passengers:    1,
		CabinClass:    "economy",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp := svc.Search(ctx, req, aggregator.SearchOptions{
		Filters: filters.NewChain(filters.MaxStops{Max: 10}),
		SortKey: "best_value",
	})

	data, err := json.MarshalIndent(resp.Metadata, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(data))
	flights, err := json.MarshalIndent(resp.Flights, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(flights))

}

func buildService() *aggregator.Service {
	store := cache.NewStore(60 * time.Second)

	var wrapped []providers.Provider
	for _, p := range providers.All() {
		wrapped = append(wrapped, cache.Wrap(p, store))
	}

	return aggregator.NewService(wrapped, 2*time.Second)
}

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	svc := buildService()
	handler := aggregator.NewHandler(svc)
	runDemo(svc)
	mux := http.NewServeMux()
	mux.HandleFunc("/search", handler.Search)
	mux.HandleFunc("/health", healthHandler)
	log.Printf("flight-aggregator listening on %s (POST /search, GET /health)", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))

}
