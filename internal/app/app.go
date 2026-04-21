package app

import (
	"fmt"

	"github.com/NIROOZbx/billing-service/config"
	"github.com/NIROOZbx/billing-service/db"
	"github.com/NIROOZbx/billing-service/internal/handlers"
	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	Config      *config.Config
	DB          *pgxpool.Pool
	GRPCHandler *handlers.GRPCHandler
}

func StartApp(cfg *config.Config) (*App, error) {
	pool, err := db.ConnectDB(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to DB: %w", err)
	}

	grpcHandler := handlers.NewGRPCHandler(pool)

	return &App{
		Config:      cfg,
		DB:          pool,
		GRPCHandler: grpcHandler,
	}, nil
}

func (a *App) Close() {
	if a.DB != nil {
		a.DB.Close()
	}
}
