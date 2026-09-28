package redis

import (
	"context"
	 "fmt"
	goredis "github.com/redis/go-redis/v9"
)

func ConnectRedis(redisHost string) (*goredis.Client, error) {

	client := goredis.NewClient(&goredis.Options{
		Addr: redisHost,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return client, nil
}
