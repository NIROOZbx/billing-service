package services

import (
	"context"
	"fmt"

	"github.com/NIROOZbx/billing-service/config"
	"github.com/NIROOZbx/billing-service/internal/domain"
	"github.com/google/uuid"
)

type subscriptionService struct {
	repo     domain.SubscriptionRepository
	planRepo domain.PlanRepository
	provider domain.BillingProvider
	config   *config.Config
}

func NewSubscriptionService(repo domain.SubscriptionRepository, planRepo domain.PlanRepository, provider domain.BillingProvider, cfg *config.Config) *subscriptionService {
	return &subscriptionService{repo: repo, planRepo: planRepo, provider: provider, config: cfg}
}

func (s *subscriptionService) CreateCheckoutSession(ctx context.Context, workspaceID, planID uuid.UUID, customerEmail string) (string, error) {
	plan, err := s.planRepo.GetPlanByID(ctx, planID)
	if err != nil {
		return "", fmt.Errorf("fetch plan: %w", err)
	}
	fmt.Println("plan details",plan.ExternalPriceID)



	if plan.ExternalPriceID == "" {
		return "", fmt.Errorf("plan %s has no stripe price id configured", plan.Name)
	}

	return s.provider.CreateCheckoutSession(domain.CheckoutSessionParams{
		WorkspaceID:   workspaceID.String(),
		PlanID:        planID.String(),
		PriceID:       plan.ExternalPriceID,
		SuccessURL:    s.config.Stripe.SuccessURL,
		CancelURL:     s.config.Stripe.CancelURL,
		CustomerEmail: customerEmail,
	})
}

func (s *subscriptionService) GetCheckoutSession(ctx context.Context, sessionID string) (*domain.CheckoutSessionDetails, error) {
	return s.provider.GetCheckoutSession(sessionID)
}

func (s *subscriptionService) GetActiveSubscription(ctx context.Context, workspaceID uuid.UUID) (*domain.Subscription, error) {
	return s.repo.GetActive(ctx, workspaceID)
}

func (s *subscriptionService) Subscribe(ctx context.Context, input domain.CreateSubscriptionInput) (*domain.Subscription, error) {
	if err := s.repo.CancelActiveSubscription(ctx, input.WorkspaceID); err != nil {
		return nil, fmt.Errorf("cancel existing subscription: %w", err)
	}
	return s.repo.Create(ctx, input)
}

func (s *subscriptionService) Cancel(ctx context.Context, workspaceID, subscriptionID uuid.UUID) error {
	return s.repo.Cancel(ctx, workspaceID, subscriptionID)
}

func (s *subscriptionService) SyncSubscription(ctx context.Context, input domain.SyncSubscriptionInput) error {
	return s.repo.SyncSubscription(ctx, input)
}

func (s *subscriptionService) GetSubscriptionByExternalID(ctx context.Context, externalID string) (*domain.Subscription, error) {
	return s.repo.GetByExternalID(ctx, externalID)
}
