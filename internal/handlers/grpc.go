package handlers

import (
	"context"

	// pb "github.com/NIROOZbx/billing-service/proto"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GRPCHandler struct {
	db *pgxpool.Pool
}

func NewGRPCHandler(db *pgxpool.Pool) *GRPCHandler {
	return &GRPCHandler{
		db: db,
	}
}

func (h *GRPCHandler) HealthCheck(ctx context.Context, req interface{}) (interface{}, error) {
	return nil, nil
}
