package cache_test

import (
	"app/internal/cache"
	"app/internal/models"
	"app/internal/providers"
	"context"
	"testing"
	"time"
)

var cacheReq = models.SearchRequest{
	Origin:        "CGK",
	Destination:   "DPS",
	DepartureDate: "2025-12-15",
	Passengers:    1,
	CabinClass:    "economy",
}

// ─── mockProvider ─────────────────────────────────────────────────────────────

type mockProvider struct {
	name      string
	callCount int
	returnErr error
	data      []byte
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) Fetch(_ context.Context, _ models.SearchRequest) (providers.FetchResult, error) {
	m.callCount++
	if m.returnErr != nil {
		return providers.FetchResult{}, m.returnErr
	}
	return providers.FetchResult{Data: m.data}, nil
}

func (m *mockProvider) Normalize(raw []byte, _ models.SearchRequest) ([]models.Flight, error) {
	return nil, nil
}

// ─── Store ───────────────────────────────────────────────────────────────────

func TestStore_SetAndGet(t *testing.T) {
	store := cache.NewStore(1 * time.Minute)
	store.Set("key1", []byte("hello"))

	data, ok := store.Get("key1")
	if !ok {
		t.Fatal("expected cache hit, got miss")
	}
	if string(data) != "hello" {
		t.Errorf("expected %q, got %q", "hello", data)
	}
}

func TestStore_MissOnUnknownKey(t *testing.T) {
	store := cache.NewStore(1 * time.Minute)
	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("expected cache miss, got hit")
	}
}

func TestStore_ExpiresAfterTTL(t *testing.T) {
	store := cache.NewStore(50 * time.Millisecond)
	store.Set("key", []byte("data"))

	time.Sleep(100 * time.Millisecond)

	_, ok := store.Get("key")
	if ok {
		t.Error("expected cache miss after TTL expiry, got hit")
	}
}

func TestStore_OverwriteUpdatesEntry(t *testing.T) {
	store := cache.NewStore(1 * time.Minute)
	store.Set("key", []byte("v1"))
	store.Set("key", []byte("v2"))

	data, ok := store.Get("key")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if string(data) != "v2" {
		t.Errorf("expected %q, got %q", "v2", data)
	}
}

// ─── cachingProvider (via Wrap) ──────────────────────────────────────────────

func TestCachingProvider_Name(t *testing.T) {
	inner := &mockProvider{name: "Mock"}
	store := cache.NewStore(1 * time.Minute)
	p := cache.Wrap(inner, store)
	if p.Name() != "Mock" {
		t.Errorf("expected Name()=Mock, got %q", p.Name())
	}
}

func TestCachingProvider_FetchCallsInnerOnce(t *testing.T) {
	inner := &mockProvider{name: "Mock", data: []byte(`{"test":true}`)}
	store := cache.NewStore(1 * time.Minute)
	p := cache.Wrap(inner, store)

	ctx := context.Background()
	// First call — should miss the cache and go to inner.
	r1, err := p.Fetch(ctx, cacheReq)
	if err != nil {
		t.Fatalf("unexpected error on first fetch: %v", err)
	}
	if r1.FromCache {
		t.Error("expected first fetch to be a cache miss (FromCache=false)")
	}
	// Second call – should be served from cache.
	r2, err := p.Fetch(ctx, cacheReq)
	if err != nil {
		t.Fatalf("unexpected error on second fetch: %v", err)
	}
	if !r2.FromCache {
		t.Error("expected second fetch to be a cache hit (FromCache=true)")
	}

	if inner.callCount != 1 {
		t.Errorf("expected inner.Fetch called once, got %d", inner.callCount)
	}
	if string(r1.Data) != string(r2.Data) {
		t.Errorf("cached data mismatch: %q vs %q", r1.Data, r2.Data)
	}
}

func TestCachingProvider_FetchCallsInnerAgainAfterTTL(t *testing.T) {
	inner := &mockProvider{name: "Mock", data: []byte(`{}`)}
	store := cache.NewStore(50 * time.Millisecond)
	p := cache.Wrap(inner, store)

	ctx := context.Background()
	p.Fetch(ctx, cacheReq)
	time.Sleep(100 * time.Millisecond)
	p.Fetch(ctx, cacheReq)

	if inner.callCount != 2 {
		t.Errorf("expected inner.Fetch called twice (cache expired), got %d", inner.callCount)
	}
}

func TestCachingProvider_FetchPropagatesError(t *testing.T) {
	inner := &mockProvider{name: "Mock", returnErr: context.DeadlineExceeded}
	store := cache.NewStore(1 * time.Minute)
	p := cache.Wrap(inner, store)

	_, err := p.Fetch(context.Background(), cacheReq)
	if err == nil {
		t.Error("expected error to propagate from inner.Fetch")
	}
}

func TestCachingProvider_DifferentRequestsDifferentCacheKeys(t *testing.T) {
	inner := &mockProvider{name: "Mock", data: []byte(`{}`)}
	store := cache.NewStore(1 * time.Minute)
	p := cache.Wrap(inner, store)

	ctx := context.Background()
	req1 := models.SearchRequest{Origin: "CGK", Destination: "DPS", DepartureDate: "2025-12-15", Passengers: 1, CabinClass: "economy"}
	req2 := models.SearchRequest{Origin: "CGK", Destination: "SUB", DepartureDate: "2025-12-15", Passengers: 1, CabinClass: "economy"}

	p.Fetch(ctx, req1)
	p.Fetch(ctx, req1) // cache hit
	p.Fetch(ctx, req2) // different key, cache miss

	if inner.callCount != 2 {
		t.Errorf("expected 2 inner calls (req1 once + req2 once), got %d", inner.callCount)
	}
}

// ─── retryingProvider (via WithRetry) ────────────────────────────────────────

func TestRetryingProvider_Name(t *testing.T) {
	inner := &mockProvider{name: "Retry"}
	p := providers.WithRetry(inner, 3)
	if p.Name() != "Retry" {
		t.Errorf("expected Name()=Retry, got %q", p.Name())
	}
}

func TestRetryingProvider_SucceedsFirstAttempt(t *testing.T) {
	inner := &mockProvider{name: "Mock", data: []byte(`ok`)}
	p := providers.WithRetry(inner, 3)

	result, err := p.Fetch(context.Background(), cacheReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result.Data) != "ok" {
		t.Errorf("expected %q, got %q", "ok", result.Data)
	}
	if inner.callCount != 1 {
		t.Errorf("expected 1 call, got %d", inner.callCount)
	}
}

type flakyProvider struct {
	name        string
	failTimes   int
	callCount   int
	successData []byte
}

func (f *flakyProvider) Name() string { return f.name }
func (f *flakyProvider) Fetch(_ context.Context, _ models.SearchRequest) (providers.FetchResult, error) {
	f.callCount++
	if f.callCount <= f.failTimes {
		return providers.FetchResult{}, context.DeadlineExceeded
	}
	return providers.FetchResult{Data: f.successData}, nil
}
func (f *flakyProvider) Normalize(raw []byte, _ models.SearchRequest) ([]models.Flight, error) {
	return nil, nil
}

func TestRetryingProvider_RetriesUntilSuccess(t *testing.T) {
	inner := &flakyProvider{name: "Flaky", failTimes: 2, successData: []byte("success")}
	p := providers.WithRetry(inner, 3)

	result, err := p.Fetch(context.Background(), cacheReq)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if string(result.Data) != "success" {
		t.Errorf("expected %q, got %q", "success", result.Data)
	}
	if inner.callCount != 3 {
		t.Errorf("expected 3 calls (2 fail + 1 success), got %d", inner.callCount)
	}
}

func TestRetryingProvider_FailsAfterMaxRetries(t *testing.T) {
	inner := &flakyProvider{name: "AlwaysFail", failTimes: 99, successData: []byte{}}
	p := providers.WithRetry(inner, 2)

	_, err := p.Fetch(context.Background(), cacheReq)
	if err == nil {
		t.Error("expected error after max retries exhausted")
	}
	// maxRetries=2 → attempts: 0,1,2 → 3 calls total
	if inner.callCount != 3 {
		t.Errorf("expected 3 calls (1 + 2 retries), got %d", inner.callCount)
	}
}

func TestRetryingProvider_ZeroRetriesMeansOneAttempt(t *testing.T) {
	inner := &flakyProvider{name: "Flaky", failTimes: 1, successData: []byte{}}
	p := providers.WithRetry(inner, 0)

	_, err := p.Fetch(context.Background(), cacheReq)
	if err == nil {
		t.Error("expected error with 0 retries and flaky provider")
	}
	if inner.callCount != 1 {
		t.Errorf("expected exactly 1 call with 0 retries, got %d", inner.callCount)
	}
}
