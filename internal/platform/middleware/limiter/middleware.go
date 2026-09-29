package limiter

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Middleware(limiter *RedisSlidingWindowLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip:= c.ClientIP()

		allowed, err := limiter.Allow(c.Request.Context(),ip)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "rate limiter unavailable"})
			return
		}
		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
