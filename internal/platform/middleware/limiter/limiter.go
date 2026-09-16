package limiter

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type SlidingWindowLimiter struct {
	window   time.Duration
	limit    int
	requests []time.Time
	mu       sync.Mutex
}

func NewSlidingWindowLimiter(limit int, window time.Duration) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		window: window,
		limit:  limit,
	}
}

func (l *SlidingWindowLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	var validRequests []time.Time

	for _, requestTime := range l.requests {
		if requestTime.After(cutoff) {
			validRequests = append(validRequests, requestTime)
		}
	}

	l.requests = validRequests

	if len(l.requests) >= l.limit {
		return false
	}

	l.requests = append(l.requests, now)

	return true
}

type RateLimiter struct {
	limiters map[string]*SlidingWindowLimiter
	mu       sync.Mutex
	limit    int
	window   time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*SlidingWindowLimiter),
		limit:    limit,
		window:   window,
	}
}

func (r *RateLimiter) GetLimiter(key string) *SlidingWindowLimiter {
	r.mu.Lock()
	defer r.mu.Unlock()
	Limiter, exists := r.limiters[key]
	if !exists {
		Limiter = NewSlidingWindowLimiter(r.limit, r.window)
		r.limiters[key] = Limiter
	}
	return Limiter
}

func RateLimitMiddleware() gin.HandlerFunc {
	rateLimiter := NewRateLimiter(
		10,
		time.Minute,
	)

	return func(c *gin.Context) {
		ip := c.ClientIP()

		limiter := rateLimiter.GetLimiter(ip)

		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
