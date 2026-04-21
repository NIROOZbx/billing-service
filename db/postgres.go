package db

import (
	"context"
	"fmt"
	"time"

	"github.com/NIROOZbx/billing-service/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ConnectDB(cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database URL: %v", err)
	}

	poolConfig.MaxConns = int32(cfg.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.MinOpenConns)

	lifetime, err := time.ParseDuration(cfg.MaxConnLifetime.String())
	if err == nil {
		poolConfig.MaxConnLifetime = lifetime
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %v", err)
	}

	return pool, nil
}
