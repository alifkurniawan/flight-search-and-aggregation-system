package providers

import (
	"app/internal/models"
	"context"
)

// FetchResult wraps the raw bytes returned by a provider's Fetch, plus
// metadata about how that data was obtained. Using a struct instead of
// extra positional return values means new metadata (e.g. a future
// RetryCount) can be added later without changing every implementation's
// call sites.
type FetchResult struct {
	Data      []byte
	FromCache bool // true only when a caching decorator served this from its store
}

type Provider interface {
	Name() string
	Fetch(ctx context.Context, req models.SearchRequest) (FetchResult, error)
	Normalize(raw []byte, req models.SearchRequest) ([]models.Flight, error)
}

func All() []Provider {
	return []Provider{
		NewGaruda(),
		NewLion(),
		NewBatik(),
		WithRetry(NewAirAsia(), 3),
	}
}
