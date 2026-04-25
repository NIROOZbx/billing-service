package domain

import (
	"context"

	"github.com/google/uuid"
)

// --- Domain Model ---

type Plan struct {
	ID                 uuid.UUID
	Name               string
	EmailLimitMonth    int32
	SmsLimitMonth      int32
	PushLimitMonth     int32
	SlackLimitMonth    int32
	WhatsappLimitMonth int32
	WebhookLimitMonth  int32
	InAppLimitMonth    int32
	IsActive           bool
	ExternalPriceID string
}

// --- Interfaces ---

type PlanRepository interface {
	GetPlanByID(ctx context.Context, id uuid.UUID) (*Plan, error)
	GetPlanByName(ctx context.Context, name string) (*Plan, error)
}

type PlanService interface {
	GetPlanByID(ctx context.Context, id uuid.UUID) (*Plan, error)
	GetPlanByName(ctx context.Context, name string) (*Plan, error)
}
