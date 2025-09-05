package middleware

import (
	"smarthome/auth"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type MiddlewareManager struct {
	pgClient    *pgxpool.Pool
	redisClient *redis.Client
	auth        *auth.AuthModule
}

func NewMiddlewareManager(pgClient *pgxpool.Pool, redisClient *redis.Client, auth *auth.AuthModule) *MiddlewareManager {
	return &MiddlewareManager{
		pgClient:    pgClient,
		redisClient: redisClient,
		auth:        auth,
	}
}
