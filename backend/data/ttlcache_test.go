package data

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTTLCacheLargeValuesAndExpiry(t *testing.T) {
	c := newTTLCache(10)
	now := time.Unix(0, 0)
	c.now = func() time.Time { return now }
	calls := 0
	fetch := func() ([]byte, error) { calls++; return make([]byte, 50_000), nil } // 50 KB: freecache would refuse this

	for i := 0; i < 3; i++ {
		v, err := c.Do("k", time.Minute, 0, fetch)
		if err != nil || len(v) != 50_000 {
			t.Fatalf("Do = %d bytes, %v", len(v), err)
		}
	}
	if calls != 1 {
		t.Fatalf("fetch called %d times, want 1 (value should be cached)", calls)
	}
	now = now.Add(61 * time.Second)
	_, _ = c.Do("k", time.Minute, 0, fetch)
	if calls != 2 {
		t.Fatalf("fetch called %d times after expiry, want 2", calls)
	}
}

func TestTTLCacheErrors(t *testing.T) {
	c := newTTLCache(10)
	now := time.Unix(0, 0)
	c.now = func() time.Time { return now }
	calls := 0
	boom := errors.New("boom")
	fetch := func() ([]byte, error) { calls++; return nil, boom }

	_, _ = c.Do("a", time.Minute, 0, fetch)
	_, _ = c.Do("a", time.Minute, 0, fetch)
	if calls != 2 {
		t.Fatalf("errors must not be cached when errTTL=0 (calls=%d)", calls)
	}
	_, err := c.Do("b", time.Minute, 30*time.Second, fetch)
	_, err2 := c.Do("b", time.Minute, 30*time.Second, fetch)
	if calls != 3 || !errors.Is(err, boom) || !errors.Is(err2, boom) {
		t.Fatalf("error should be cached for errTTL (calls=%d)", calls)
	}
}

func TestTTLCacheDedupesConcurrentFetches(t *testing.T) {
	c := newTTLCache(10)
	var calls int32
	release := make(chan struct{})
	fetch := func() ([]byte, error) {
		atomic.AddInt32(&calls, 1)
		<-release
		return []byte("v"), nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if v, _ := c.Do("k", time.Minute, 0, fetch); string(v) != "v" {
				t.Errorf("got %q", v)
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	if calls != 1 {
		t.Fatalf("20 concurrent callers caused %d fetches, want 1", calls)
	}
}

func TestTTLCacheEvicts(t *testing.T) {
	c := newTTLCache(4)
	now := time.Unix(0, 0)
	c.now = func() time.Time { return now }
	for i := 0; i < 50; i++ {
		key := string(rune('a'+i%26)) + string(rune('A'+i/26))
		_, _ = c.Do(key, time.Duration(i+1)*time.Second, 0, func() ([]byte, error) { return []byte("x"), nil })
		now = now.Add(time.Second)
	}
	if len(c.items) > 4 {
		t.Fatalf("cache grew to %d entries, max 4", len(c.items))
	}
}
