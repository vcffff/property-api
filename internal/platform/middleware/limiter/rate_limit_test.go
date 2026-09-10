package limiter

import (
	"sync"
	"testing"
	"time"
)

func TestRateLimiterUnderLoad(t *testing.T) {
	rateLimiter := NewRateLimiter(100, time.Minute)

	const totalRequests = 1000

	var wg sync.WaitGroup
	var mu sync.Mutex

	allowed := 0
	rejected := 0

	wg.Add(totalRequests)

	for i := 0; i < totalRequests; i++ {
		go func() {
			defer wg.Done()

			limiter := rateLimiter.GetLimiter("127.0.0.1")

			if limiter.Allow() {
				mu.Lock()
				allowed++
				mu.Unlock()
			} else {
				mu.Lock()
				rejected++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	t.Logf("Allowed: %d", allowed)
	t.Logf("Rejected: %d", rejected)

	if allowed != 100 {
		t.Errorf("expected 100 allowed requests, got %d", allowed)
	}

	if rejected != 900 {
		t.Errorf("expected 900 rejected requests, got %d", rejected)
	}
}
