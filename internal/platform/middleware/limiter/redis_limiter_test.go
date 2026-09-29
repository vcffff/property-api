package limiter

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisLimiterAtomicity(t *testing.T) {
	ctx := context.Background()

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis is not available:", err)
	}

	const (
		limit         = 100
		totalRequests = 1000
	)

	key := "atomic-test"

	// Чистое состояние перед тестом
	client.Del(ctx, "rate_limit:"+key)

	limiter := NewRedisSlidingWindowLimiter(
		client,
		limit,
		time.Minute,
	)

	var allowed int64
	var rejected int64

	var wg sync.WaitGroup

	// Барьер, чтобы goroutines стартовали максимально одновременно
	start := make(chan struct{})

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			<-start

			ok, err := limiter.Allow(ctx, key)
			if err != nil {
				t.Errorf("Allow failed: %v", err)
				return
			}

			if ok {
				atomic.AddInt64(&allowed, 1)
			} else {
				atomic.AddInt64(&rejected, 1)
			}
		}()
	}

	// Одновременно отпускаем 1000 goroutines
	close(start)

	wg.Wait()

	t.Logf("Allowed: %d", allowed)
	t.Logf("Rejected: %d", rejected)

	if allowed != limit {
		t.Errorf(
			"atomicity broken: expected %d allowed, got %d",
			limit,
			allowed,
		)
	}

	if rejected != totalRequests-limit {
		t.Errorf(
			"expected %d rejected, got %d",
			totalRequests-limit,
			rejected,
		)
	}

	// Проверяем ещё и реальное состояние Redis
	count, err := client.ZCard(
		ctx,
		"rate_limit:"+key,
	).Result()

	if err != nil {
		t.Fatal(err)
	}

	if count != limit {
		t.Errorf(
			"expected Redis ZSET size %d, got %d",
			limit,
			count,
		)
	}
}
