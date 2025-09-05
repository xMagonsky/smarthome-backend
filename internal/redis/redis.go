package redis

import "github.com/redis/go-redis/v9"

// NewRedisClient creates a Redis client
func NewRedisClient(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{Addr: addr})
}

// Expand with wrapper methods for streams, caches, etc.
