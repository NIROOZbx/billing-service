package repositories

import (
	"context"
	"fmt"

	"github.com/NIROOZbx/billing-service/db/sqlc"
	"github.com/NIROOZbx/billing-service/internal/domain"
	"github.com/NIROOZbx/billing-service/pkg/apperrors"
	"github.com/NIROOZbx/billing-service/pkg/helpers"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type subscriptionRepository struct {
	queries *sqlc.Queries
}

func NewSubscriptionRepository(queries *sqlc.Queries) *subscriptionRepository {
	return &subscriptionRepository{queries: queries}
}

func (s *subscriptionRepository) GetActive(ctx context.Context, workspaceID uuid.UUID) (*domain.Subscription, error) {
	row, err := s.queries.GetActiveSubscription(ctx, helpers.ToPgUUID(workspaceID))
	if err != nil {
		return nil, apperrors.MapDBError(err)
	}

	return mapToDomain(&row), nil
}

func (s *subscriptionRepository) Create(ctx context.Context, input domain.CreateSubscriptionInput) (*domain.Subscription, error) {
	row, err := s.queries.CreateSubscription(ctx, sqlc.CreateSubscriptionParams{
		WorkspaceID:            helpers.ToPgUUID(input.WorkspaceID),
		PlanID:                 helpers.ToPgUUID(input.PlanID),
		PaymentProvider:        input.PaymentProvider,
		ExternalSubscriptionID: helpers.ToPgText(input.ExternalSubscriptionID),
		ExternalCustomerID:     helpers.ToPgText(input.ExternalCustomerID),
	})
	if err != nil {
		return nil, apperrors.MapDBError(err)
	}

	return mapToDomain(&row), nil
}

func (s *subscriptionRepository) CancelActiveSubscription(ctx context.Context, workspaceID uuid.UUID) error {
	return s.queries.CancelActiveSubscription(ctx, helpers.ToPgUUID(workspaceID))
}

func (s *subscriptionRepository) Cancel(ctx context.Context, workspaceID, subscriptionID uuid.UUID) error {
	cmd, err := s.queries.CancelSubscription(ctx, sqlc.CancelSubscriptionParams{
		WorkspaceID: helpers.ToPgUUID(workspaceID),
		ID:          helpers.ToPgUUID(subscriptionID),
	})
	if err != nil {
		return apperrors.MapDBError(err)
	}

	if cmd.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}

	return nil
}

func (s *subscriptionRepository) RenewExpiredFreeSubscription(ctx context.Context, workspaceID uuid.UUID) (*domain.Subscription, error) {
	row, err := s.queries.RenewExpiredFreeSubscription(ctx, helpers.ToPgUUID(workspaceID))
	if err != nil {
		return nil, apperrors.MapDBError(err)
	}

	return mapToDomain(&row), nil
}

func (s *subscriptionRepository) SyncSubscription(ctx context.Context, input domain.SyncSubscriptionInput) error {

	fmt.Println("sync subscriprion called",input)
	var pgCancelledAt pgtype.Timestamptz
	if input.CancelledAt != nil && !input.CancelledAt.IsZero() {
		pgCancelledAt = helpers.ToPgTimestamp(*input.CancelledAt)
	}

	arg := sqlc.SyncSubscriptionParams{
		ExternalSubscriptionID: helpers.ToPgText(input.ExternalSubscriptionID),
		Status:                 input.Status,
		CurrentPeriodStart:     helpers.ToPgTimestamp(input.CurrentPeriodStart),
		CurrentPeriodEnd:       helpers.ToPgTimestamp(input.CurrentPeriodEnd),
		CancelledAt:            pgCancelledAt,
	}

	_, err := s.queries.SyncSubscription(ctx, arg)
	return apperrors.MapDBError(err)
}

func (s *subscriptionRepository) GetExpiringSubscription(ctx context.Context, limit int) ([]*domain.Subscription, error) {
	expiringSubscription, err := s.queries.GetExpiringSubscriptions(ctx, int32(limit))

	if err != nil {
		return nil, apperrors.MapDBError(err)
	}

	res := make([]*domain.Subscription, len(expiringSubscription))
	for i, sub := range expiringSubscription {
		res[i] = mapToDomain(&sub)
	}
	return res, nil
}

func (s *subscriptionRepository) MarkExpiryEmailSent(ctx context.Context, id uuid.UUID) error {
	return s.queries.MarkExpiryEmailSent(ctx, helpers.ToPgUUID(id))
}

func (s *subscriptionRepository) GetByExternalID(ctx context.Context, externalID string) (*domain.Subscription, error) {
	row, err := s.queries.GetSubscriptionByExternalID(ctx, helpers.ToPgText(externalID))
	if err != nil {
		return nil, apperrors.MapDBError(err)
	}
	return mapToDomain(&row), nil
}

func mapToDomain(row *sqlc.BillingSubscription) *domain.Subscription {
	if row == nil {
		return nil
	}

	sub := &domain.Subscription{
		ID:                     helpers.FromPgUUID(row.ID),
		WorkspaceID:            helpers.FromPgUUID(row.WorkspaceID),
		PlanID:                 helpers.FromPgUUID(row.PlanID),
		PaymentProvider:        row.PaymentProvider,
		ExternalSubscriptionID: row.ExternalSubscriptionID.String,
		ExternalCustomerID:     row.ExternalCustomerID.String,
		Status:                 row.Status,
		CurrentPeriodStart:     row.CurrentPeriodStart.Time,
		CurrentPeriodEnd:       row.CurrentPeriodEnd.Time,
		CreatedAt:              row.CreatedAt.Time,
		UpdatedAt:              row.UpdatedAt.Time,
	}

	if row.CancelledAt.Valid {
		sub.CancelledAt = row.CancelledAt.Time
	}

	return sub
}
