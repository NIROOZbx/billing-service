package repositories

import (
	"context"

	"github.com/NIROOZbx/billing-service/db/sqlc"
	"github.com/NIROOZbx/billing-service/internal/domain"
	"github.com/NIROOZbx/billing-service/pkg/apperrors"
	"github.com/NIROOZbx/billing-service/pkg/helpers"
	"github.com/google/uuid"
)

type planRepository struct {
	queries *sqlc.Queries
}

func NewPlanRepository(queries *sqlc.Queries) *planRepository {
	return &planRepository{queries: queries}
}

func (p *planRepository) GetPlanByID(ctx context.Context, id uuid.UUID) (*domain.Plan, error) {
	row, err := p.queries.GetPlanByID(ctx, helpers.ToPgUUID(id))
	if err != nil {
		return nil, apperrors.MapDBError(err)
	}

	return mapToPlanDomain(&row), nil
}

func (p *planRepository) GetPlanByName(ctx context.Context, name string) (*domain.Plan, error) {
	row, err := p.queries.GetPlanByName(ctx, name)
	if err != nil {
		return nil, apperrors.MapDBError(err)
	}

	return mapToPlanDomain(&row), nil
}

func mapToPlanDomain(row *sqlc.Plan) *domain.Plan {
	if row == nil {
		return nil
	}

	return &domain.Plan{
		ID:                 helpers.FromPgUUID(row.ID),
		Name:               row.Name,
		EmailLimitMonth:    row.EmailLimitMonth,
		SmsLimitMonth:      row.SmsLimitMonth,
		PushLimitMonth:     row.PushLimitMonth,
		SlackLimitMonth:    row.SlackLimitMonth,
		WhatsappLimitMonth: row.WhatsappLimitMonth,
		WebhookLimitMonth:  row.WebhookLimitMonth,
		InAppLimitMonth:    row.InAppLimitMonth,
		IsActive:           row.IsActive,
	}
}
