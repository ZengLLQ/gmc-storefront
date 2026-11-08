package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

type Cache interface {
	Get(key string) (any, bool)
	Set(key string, val any, ttl time.Duration)
	DeletePrefix(prefix string)
	Clear()
	Stats() (hits, misses uint64)
}

type cacheEntry struct {
	val       any
	expiresAt time.Time
}

type InMemoryCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
	hits    atomic.Uint64
	misses  atomic.Uint64
	sf      singleflight.Group
}

func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{
		entries: make(map[string]cacheEntry),
	}
}

func (c *InMemoryCache) Get(key string) (any, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		c.misses.Add(1)
		return nil, false
	}
	c.hits.Add(1)
	return e.val, true
}

func (c *InMemoryCache) Set(key string, val any, ttl time.Duration) {
	c.mu.Lock()
	c.entries[key] = cacheEntry{
		val:       val,
		expiresAt: time.Now().Add(ttl),
	}
	c.mu.Unlock()
}

func (c *InMemoryCache) DeletePrefix(prefix string) {
	c.mu.Lock()
	for k := range c.entries {
		if strings.HasPrefix(k, prefix) {
			delete(c.entries, k)
		}
	}
	c.mu.Unlock()
}

func (c *InMemoryCache) Clear() {
	c.mu.Lock()
	c.entries = make(map[string]cacheEntry)
	c.mu.Unlock()
}

func (c *InMemoryCache) Stats() (uint64, uint64) {
	return c.hits.Load(), c.misses.Load()
}

func (c *InMemoryCache) DoSingleflight(key string, ttl time.Duration, fn func() (any, error)) (any, error) {
	if v, ok := c.Get(key); ok {
		return v, nil
	}
	v, err, _ := c.sf.Do(key, func() (any, error) {
		if v2, ok := c.Get(key); ok {
			return v2, nil
		}
		res, err := fn()
		if err == nil {
			c.Set(key, res, ttl)
		}
		return res, err
	})
	return v, err
}

// Revalidator sends asynchronous ISR cache revalidation webhooks to Next.js.
type Revalidator struct {
	webURL   string
	secret   string
	client   *http.Client
	failures atomic.Uint64
}

func NewRevalidator(webURL, secret string) *Revalidator {
	return &Revalidator{
		webURL: strings.TrimRight(webURL, "/"),
		secret: secret,
		client: &http.Client{Timeout: 3 * time.Second},
	}
}

func (r *Revalidator) Trigger(tags ...string) {
	if r == nil || r.webURL == "" || len(tags) == 0 {
		return
	}
	go func() {
		bodyBytes, _ := json.Marshal(map[string]any{"tags": tags})
		for attempt := 1; attempt <= 3; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.webURL+"/internal/revalidate", bytes.NewReader(bodyBytes))
			if err != nil {
				cancel()
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Revalidate-Secret", r.secret)
			resp, err := r.client.Do(req)
			cancel()
			if err == nil && resp != nil && resp.StatusCode < 300 {
				resp.Body.Close()
				return
			}
			if resp != nil {
				resp.Body.Close()
			}
			time.Sleep(time.Duration(attempt*250) * time.Millisecond)
		}
		r.failures.Add(1)
		slog.Warn("isr revalidation webhook failed after 3 attempts", "tags", tags)
	}()
}

func (r *Revalidator) Failures() uint64 {
	if r == nil {
		return 0
	}
	return r.failures.Load()
}
