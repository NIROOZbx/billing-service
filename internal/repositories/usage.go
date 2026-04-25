package repositories

import (
	"context"

	"github.com/NIROOZbx/billing-service/db/sqlc"
	"github.com/NIROOZbx/billing-service/internal/domain"
	"github.com/NIROOZbx/billing-service/pkg/apperrors"
	"github.com/NIROOZbx/billing-service/pkg/helpers"
	"github.com/google/uuid"
)

type usageRepository struct {
	queries *sqlc.Queries
}

func NewUsageRepository(queries *sqlc.Queries) *usageRepository {
	return &usageRepository{queries: queries}
}

func (u *usageRepository) GetUsageByChannel(ctx context.Context, workspaceID, environmentID uuid.UUID, channel string) (*domain.Usage, error) {
	row, err := u.queries.GetUsageByChannel(ctx, sqlc.GetUsageByChannelParams{
		WorkspaceID:   helpers.ToPgUUID(workspaceID),
		EnvironmentID: helpers.ToPgUUID(environmentID),
		ChannelName:   channel,
	})
	if err != nil {
		return nil,apperrors.MapDBError(err)
	}

	return mapToUsage(row), nil
}

func (u *usageRepository) GetUsage(ctx context.Context, workspaceID, environmentID uuid.UUID) ([]*domain.Usage, error) {
	rows, err := u.queries.GetUsageByWorkspace(ctx, sqlc.GetUsageByWorkspaceParams{
		WorkspaceID:   helpers.ToPgUUID(workspaceID),
		EnvironmentID: helpers.ToPgUUID(environmentID),
	})
	if err != nil {
		return nil, apperrors.MapDBError(err)
	}

	usages := make([]*domain.Usage, len(rows))
	for i, row := range rows {
		usages[i] = mapToUsage(row)
	}
	return usages, nil
}

func (u *usageRepository) GetProviderUsage(ctx context.Context, workspaceID, environmentID uuid.UUID) ([]*domain.ProviderUsage, error) {
	rows, err := u.queries.GetProviderUsageByWorkspace(ctx, sqlc.GetProviderUsageByWorkspaceParams{
		WorkspaceID:   helpers.ToPgUUID(workspaceID),
		EnvironmentID: helpers.ToPgUUID(environmentID),
	})
	if err != nil {
		return nil, apperrors.MapDBError(err)
	}

	providerUsages := make([]*domain.ProviderUsage, len(rows))
	for i, row := range rows {
		providerUsages[i] = mapToProviderUsage(row)
	}
	return providerUsages, nil
}

func (u *usageRepository) UpsertWorkSpaceUsage(ctx context.Context, input domain.UpsertUsageInput) (*domain.Usage, error) {
	row, err := u.queries.UpsertUsage(ctx, sqlc.UpsertUsageParams{
		WorkspaceID:   helpers.ToPgUUID(input.WorkspaceID),
		EnvironmentID: helpers.ToPgUUID(input.EnvironmentID),
		ChannelName:   input.ChannelName,
		CurrentUsage:  1,
		ResetAt:       helpers.ToPgTimestamp(input.ResetAt),
	})
	if err != nil {
		return nil, apperrors.MapDBError(err)
	}
	return mapToUsage(row), nil
}

func (u *usageRepository) SetLimit80Sent(ctx context.Context, id uuid.UUID) error {
	return u.queries.SetLimit80Sent(ctx, helpers.ToPgUUID(id))
}

func (u *usageRepository) SetLimit100Sent(ctx context.Context, id uuid.UUID) error {
	return u.queries.SetLimit100Sent(ctx, helpers.ToPgUUID(id))
}

func (u *usageRepository) UpsertProviderUsage(ctx context.Context, input domain.UpsertProviderUsageInput) error {
	var successCount, failureCount int64
	if input.Success {
		successCount = 1
	} else {
		failureCount = 1
	}

	err := u.queries.UpsertProviderUsage(ctx, sqlc.UpsertProviderUsageParams{
		WorkspaceID:     helpers.ToPgUUID(input.WorkspaceID),
		EnvironmentID:   helpers.ToPgUUID(input.EnvironmentID),
		ChannelConfigID: helpers.ToPgUUID(input.ChannelConfigID),
		ProviderName:    input.ProviderName,
		ChannelName:     input.ChannelName,
		SuccessCount:    successCount,
		FailureCount:    failureCount,
		ResetAt:         helpers.ToPgTimestamp(input.ResetAt),
	})
	return apperrors.MapDBError(err)
}

func mapToUsage(row sqlc.BillingUsage) *domain.Usage {
	return &domain.Usage{
		ID:            helpers.FromPgUUID(row.ID),
		WorkspaceID:   helpers.FromPgUUID(row.WorkspaceID),
		EnvironmentID: helpers.FromPgUUID(row.EnvironmentID),
		ChannelName:   row.ChannelName,
		CurrentUsage:  row.CurrentUsage,
		ResetAt:       helpers.PgToTime(row.ResetAt),
		Limit80Sent:   row.Limit80Sent.Bool,
		Limit100Sent:  row.Limit100Sent.Bool,
	}
}

func mapToProviderUsage(row sqlc.BillingProviderUsage) *domain.ProviderUsage {
	return &domain.ProviderUsage{
		ID:              helpers.FromPgUUID(row.ID),
		WorkspaceID:     helpers.FromPgUUID(row.WorkspaceID),
		EnvironmentID:   helpers.FromPgUUID(row.EnvironmentID),
		ChannelConfigID: helpers.FromPgUUID(row.ChannelConfigID),
		ProviderName:    row.ProviderName,
		ChannelName:     row.ChannelName,
		SuccessCount:    row.SuccessCount,
		FailureCount:    row.FailureCount,
		ResetAt:         helpers.PgToTime(row.ResetAt),
	}
}
