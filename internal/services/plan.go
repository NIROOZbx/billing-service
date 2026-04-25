package services

import (
	"context"

	"github.com/NIROOZbx/billing-service/internal/domain"
	"github.com/google/uuid"
)

type planService struct {
	repo domain.PlanRepository
}

func NewPlanService(repo domain.PlanRepository) *planService {
	return &planService{repo: repo}
}

func (s *planService) GetPlanByID(ctx context.Context, id uuid.UUID) (*domain.Plan, error) {
	return s.repo.GetPlanByID(ctx, id)
}

func (s *planService) GetPlanByName(ctx context.Context, name string) (*domain.Plan, error) {
	return s.repo.GetPlanByName(ctx, name)
}
