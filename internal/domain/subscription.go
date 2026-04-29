package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// --- Domain Model ---

type Subscription struct {
	ID                     uuid.UUID
	WorkspaceID            uuid.UUID
	PlanID                 uuid.UUID
	PaymentProvider        string
	ExternalSubscriptionID string
	ExternalCustomerID     string
	Status                 string
	CurrentPeriodStart     time.Time
	CurrentPeriodEnd       time.Time
	CancelledAt            time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// --- Input DTOs ---

type CreateSubscriptionInput struct {
	WorkspaceID            uuid.UUID
	PlanID                 uuid.UUID
	PaymentProvider        string
	ExternalSubscriptionID string
	ExternalCustomerID     string
}

type SyncSubscriptionInput struct {
	ExternalSubscriptionID string
	Status                 string
	CurrentPeriodStart     time.Time
	CurrentPeriodEnd       time.Time
	CancelledAt            *time.Time
}

type CheckoutSessionDetails struct {
	ID             string
	CustomerEmail  string
	AmountTotal    int64
	Currency       string
	PaymentStatus  string
	PlanName       string
	SubscriptionID string
}

// --- Interfaces ---

type SubscriptionRepository interface {
	GetActive(ctx context.Context, workspaceID uuid.UUID) (*Subscription, error)
	Create(ctx context.Context, input CreateSubscriptionInput) (*Subscription, error)
	Cancel(ctx context.Context, workspaceID, subscriptionID uuid.UUID) error
	RenewExpiredFreeSubscription(ctx context.Context, workspaceID uuid.UUID) (*Subscription, error)
	CancelActiveSubscription(ctx context.Context, workspaceID uuid.UUID) error
	SyncSubscription(ctx context.Context, input SyncSubscriptionInput) error
	GetExpiringSubscription(ctx context.Context, limit int) ([]*Subscription, error)
	MarkExpiryEmailSent(ctx context.Context, id uuid.UUID) error
	GetByExternalID(ctx context.Context, externalID string) (*Subscription, error)
}

type SubscriptionService interface {
	GetActiveSubscription(ctx context.Context, workspaceID uuid.UUID) (*Subscription, error)
	Subscribe(ctx context.Context, input CreateSubscriptionInput) (*Subscription, error)
	Cancel(ctx context.Context, workspaceID, subscriptionID uuid.UUID) error
	SyncSubscription(ctx context.Context, input SyncSubscriptionInput) error
	CreateCheckoutSession(ctx context.Context, workspaceID, planID uuid.UUID, customerEmail string) (string, error)
	GetCheckoutSession(ctx context.Context, sessionID string) (*CheckoutSessionDetails, error)
	GetSubscriptionByExternalID(ctx context.Context, externalID string) (*Subscription, error)
}
