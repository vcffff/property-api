package limiter

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)
type RedisSlidingWindowLimiter struct {
	client *redis.Client
	limit  int64
	window time.Duration
}


var slidingWindowScript = redis.NewScript(`
	local key = KEYS[1]

	local now = tonumber(ARGV[1])
	local window = tonumber(ARGV[2])
	local limit = tonumber(ARGV[3])
	local member = ARGV[4]

	local cutoff = now - window

	
	redis.call(
		"ZREMRANGEBYSCORE",
		key,
		"-inf",
		cutoff
	)

	
	local count = redis.call("ZCARD", key)

	
	if count >= limit then
		return 0
	end

	
	redis.call(
		"ZADD",
		key,
		now,
		member
	)

	
	redis.call(
		"PEXPIRE",
		key,
		window
	)

	return 1
`)

func NewRedisSlidingWindowLimiter(
	client *redis.Client,
	limit int64,
	window time.Duration,
) *RedisSlidingWindowLimiter {
	return &RedisSlidingWindowLimiter{
		client: client,
		limit:  limit,
		window: window,
	}
}

func randomMember() (string, error) {
	b := make([]byte, 16)

	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

func (l *RedisSlidingWindowLimiter) Allow(
	ctx context.Context,
	key string,
) (bool, error) {

	now := time.Now()

	redisKey := "rate_limit:" + key

	member, err:=randomMember();
	if err != nil {
		return false, fmt.Errorf("failed to generate random member: %w", err)
	}

	result, err := slidingWindowScript.Run(
		ctx,
		l.client,

		[]string{redisKey}, 

		now.UnixMilli(),        
		l.window.Milliseconds(), 
		l.limit,                 
		member,                  
	).Int()

	if err != nil {
		return false, fmt.Errorf(
			"failed to execute rate limiter script: %w",
			err,
		)
	}

	return result == 1, nil
}