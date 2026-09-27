package data

import (
	"sync"
	"time"
)

// ttlCache is a tiny in-memory cache with per-entry TTL and request
// de-duplication: concurrent callers asking for the same key while it is being
// fetched wait for that one fetch instead of each calling the API.
//
// It replaces freecache, which silently refuses values larger than 1/1024 of
// its capacity (4 KB for a 4 MB cache) — so the ~30 KB price-history and
// top-50 responses were never cached.
type ttlCache struct {
	mu         sync.Mutex
	items      map[string]cacheEntry
	inflight   map[string]*cacheCall
	maxEntries int
	now        func() time.Time
}

type cacheEntry struct {
	val []byte
	err error
	exp time.Time
}

type cacheCall struct {
	done chan struct{}
	val  []byte
	err  error
}

func newTTLCache(maxEntries int) *ttlCache {
	return &ttlCache{
		items:      map[string]cacheEntry{},
		inflight:   map[string]*cacheCall{},
		maxEntries: maxEntries,
		now:        time.Now,
	}
}

// Do returns the cached value for key, or calls fetch once and caches its
// result for ttl. Errors are cached for errTTL (0 = not cached), so a failing
// endpoint is not hammered.
func (c *ttlCache) Do(key string, ttl, errTTL time.Duration, fetch func() ([]byte, error)) ([]byte, error) {
	c.mu.Lock()
	if e, ok := c.items[key]; ok && c.now().Before(e.exp) {
		c.mu.Unlock()
		return e.val, e.err
	}
	if call, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		<-call.done
		return call.val, call.err
	}
	call := &cacheCall{done: make(chan struct{})}
	c.inflight[key] = call
	c.mu.Unlock()

	call.val, call.err = fetch()

	c.mu.Lock()
	delete(c.inflight, key)
	keep := ttl
	if call.err != nil {
		keep = errTTL
	}
	if keep > 0 {
		if len(c.items) >= c.maxEntries {
			c.evictLocked()
		}
		c.items[key] = cacheEntry{val: call.val, err: call.err, exp: c.now().Add(keep)}
	}
	c.mu.Unlock()
	close(call.done)
	return call.val, call.err
}

// evictLocked drops expired entries; if still full, drops the soonest-to-expire half.
func (c *ttlCache) evictLocked() {
	now := c.now()
	for k, e := range c.items {
		if !now.Before(e.exp) {
			delete(c.items, k)
		}
	}
	if len(c.items) < c.maxEntries {
		return
	}
	var cutoff time.Time
	for _, e := range c.items {
		if cutoff.IsZero() || e.exp.After(cutoff) {
			cutoff = e.exp
		}
	}
	// Remove everything expiring before the midpoint between now and the latest expiry.
	mid := now.Add(cutoff.Sub(now) / 2)
	for k, e := range c.items {
		if e.exp.Before(mid) || len(c.items) >= c.maxEntries {
			delete(c.items, k)
		}
	}
}
