package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// --- Domain Models ---

type Usage struct {
	ID            uuid.UUID
	WorkspaceID   uuid.UUID
	EnvironmentID uuid.UUID
	ChannelName   string
	CurrentUsage  int64
	ResetAt       time.Time
	Limit80Sent   bool
	Limit100Sent  bool
}

type ProviderUsage struct {
	ID              uuid.UUID
	WorkspaceID     uuid.UUID
	EnvironmentID   uuid.UUID
	ChannelConfigID uuid.UUID
	ProviderName    string
	ChannelName     string
	SuccessCount    int64
	FailureCount    int64
	ResetAt         time.Time
}

// --- Input DTOs ---

type UpsertUsageInput struct {
	WorkspaceID   uuid.UUID
	EnvironmentID uuid.UUID
	ChannelName   string
	ResetAt       time.Time
}

type UpsertProviderUsageInput struct {
	WorkspaceID     uuid.UUID
	EnvironmentID   uuid.UUID
	ChannelConfigID uuid.UUID
	ChannelName     string
	ProviderName    string
	Success         bool
	ResetAt         time.Time
}

// --- Interfaces ---

type UsageRepository interface {
	GetUsage(ctx context.Context, workspaceID, environmentID uuid.UUID) ([]*Usage, error)
	GetUsageByChannel(ctx context.Context, workspaceID, environmentID uuid.UUID, channel string) (*Usage, error)
	GetProviderUsage(ctx context.Context, workspaceID, environmentID uuid.UUID) ([]*ProviderUsage, error)
	UpsertWorkSpaceUsage(ctx context.Context, input UpsertUsageInput) (*Usage, error)
	UpsertProviderUsage(ctx context.Context, input UpsertProviderUsageInput) error
	SetLimit80Sent(ctx context.Context, id uuid.UUID) error
	SetLimit100Sent(ctx context.Context, id uuid.UUID) error
}

type CheckLimitResult struct {
	Allowed bool
	Reason  string
	Limit   int32
	Current int64
	ResetAt time.Time
}

type UsageService interface {
	CheckLimit(ctx context.Context, workspaceID, environmentID uuid.UUID, channel string) (*CheckLimitResult, error)
	RecordUsage(ctx context.Context, input UpsertProviderUsageInput) error
	GetUsageSummary(ctx context.Context, workspaceID, environmentID uuid.UUID) ([]*Usage, error)
	GetProviderUsageSummary(ctx context.Context, workspaceID, environmentID uuid.UUID) ([]*ProviderUsage, error)
}
