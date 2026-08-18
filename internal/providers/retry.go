package providers

import (
	"app/internal/models"
	"context"
	"time"
)

type retryingProvider struct {
	inner      Provider
	maxRetries int
}

func WithRetry(inner Provider, maxRetries int) Provider {
	return &retryingProvider{inner: inner, maxRetries: maxRetries}
}

func (r *retryingProvider) Name() string {
	return r.inner.Name()
}

func (r *retryingProvider) Fetch(ctx context.Context, req models.SearchRequest) (FetchResult, error) {
	var lastErr error
	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		result, err := r.inner.Fetch(ctx, req)
		if err == nil {
			return result, nil
		}
		lastErr = err
		time.Sleep(backoffDuration(attempt))
	}
	return FetchResult{}, lastErr
}

func (r *retryingProvider) Normalize(raw []byte, req models.SearchRequest) ([]models.Flight, error) {
	return r.inner.Normalize(raw, req)
}

func backoffDuration(attempt int) time.Duration {
	const base = 100 * time.Millisecond

	return base * time.Duration(1<<attempt)
}
