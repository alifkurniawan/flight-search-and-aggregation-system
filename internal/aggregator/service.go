package aggregator

import (
	"app/internal/filters"
	"app/internal/models"
	"app/internal/providers"
	"app/internal/sorts"
	"context"
	"sync"
	"time"
)

type SearchOptions struct {
	Filters *filters.Chain
	SortKey string // see sortstrategy.Get for accepted values
}

type Service struct {
	providers       []providers.Provider
	providerTimeout time.Duration
}

type providerResult struct {
	provider  string
	flights   []models.Flight
	err       error
	latency   time.Duration
	fromCache bool
}

func NewService(providers []providers.Provider, providerTimeout time.Duration) *Service {
	if providerTimeout <= 0 {
		providerTimeout = 2 * time.Second
	}
	return &Service{providers: providers, providerTimeout: providerTimeout}
}

func (s *Service) Search(ctx context.Context, req models.SearchRequest, opts SearchOptions) models.SearchResponse {
	start := time.Now()
	results := s.fetchAll(ctx, req, s.providerTimeout)
	var providersSucceeded int
	var providersFailed int
	var allFlights []models.Flight
	var cacheHit bool
	for _, r := range results {
		if r.err == nil {
			allFlights = append(allFlights, r.flights...)
			providersSucceeded++
		} else {
			providersFailed++
		}
		if r.fromCache {
			cacheHit = true
		}
	}
	filtered := opts.Filters.Apply(allFlights)
	sorts.GetStrategy(opts.SortKey).Sort(filtered)
	return models.SearchResponse{
		SearchCriteria: models.SearchCriteria{
			Origin:        req.Origin,
			Destination:   req.Destination,
			DepartureDate: req.DepartureDate,
			Passengers:    req.Passengers,
			CabinClass:    req.CabinClass,
		},
		Metadata: models.Metadata{
			TotalResults:       len(filtered),
			ProvidersQueried:   len(s.providers),
			ProvidersSucceeded: providersSucceeded,
			ProvidersFailed:    providersFailed,
			SearchTimeMs:       time.Since(start).Milliseconds(),
			CacheHit:           cacheHit,
		},
		Flights: filtered,
	}
}

func (s *Service) fetchAll(ctx context.Context, req models.SearchRequest, timeProvider time.Duration) []providerResult {
	ch := make(chan providerResult, len(s.providers))
	var wg sync.WaitGroup
	for _, p := range s.providers {
		wg.Add(1)
		go func(p providers.Provider) {
			defer wg.Done()

			pCtx, cancel := context.WithTimeout(ctx, timeProvider)
			defer cancel()

			start := time.Now()
			result, err := p.Fetch(pCtx, req)
			if err != nil {
				ch <- providerResult{provider: p.Name(), err: err, latency: time.Since(start)}
				return
			}
			flights, err := p.Normalize(result.Data, req)
			ch <- providerResult{provider: p.Name(), flights: flights, err: err, latency: time.Since(start), fromCache: result.FromCache}
		}(p)
	}

	wg.Wait()
	close(ch)

	var results []providerResult
	for r := range ch {
		results = append(results, r)
	}

	return results
}
