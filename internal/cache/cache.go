package cache

import (
	"app/internal/models"
	"app/internal/providers"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

type entry struct {
	data      []byte
	expiresAt time.Time
}

type Store struct {
	mu   sync.RWMutex
	data map[string]entry
	ttl  time.Duration
}

func NewStore(ttl time.Duration) *Store {
	return &Store{data: make(map[string]entry), ttl: ttl}
}

func (s *Store) Get(key string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.data[key]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.data, true
}

// Set stores bytes under key with the store's configured TTL.
func (s *Store) Set(key string, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = entry{data: data, expiresAt: time.Now().Add(s.ttl)}
}

type cachingProvider struct {
	inner providers.Provider
	store *Store
}

func (c *cachingProvider) Name() string { return c.inner.Name() }

func (c *cachingProvider) Fetch(ctx context.Context, req models.SearchRequest) (providers.FetchResult, error) {
	key := cacheKey(c.inner.Name(), req)
	if cached, ok := c.store.Get(key); ok {
		return providers.FetchResult{Data: cached, FromCache: true}, nil
	}
	result, err := c.inner.Fetch(ctx, req)
	if err != nil {
		return providers.FetchResult{}, err
	}
	c.store.Set(key, result.Data)
	return result, nil // FromCache stays false: this was a fresh fetch, not a cache hit
}

func (c *cachingProvider) Normalize(raw []byte, req models.SearchRequest) ([]models.Flight, error) {
	return c.inner.Normalize(raw, req)
}

func cacheKey(providerName string, req models.SearchRequest) string {
	b, _ := json.Marshal(req)
	h := sha1.New()
	h.Write([]byte(providerName))
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

func Wrap(inner providers.Provider, store *Store) providers.Provider {
	return &cachingProvider{inner: inner, store: store}
}
