package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NIROOZbx/billing-service/internal/domain"
	"github.com/NIROOZbx/billing-service/internal/producer"
	"github.com/NIROOZbx/billing-service/pkg/apperrors"
	"github.com/NIROOZbx/billing-service/pkg/constants"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type usageService struct {
	usageRepo        domain.UsageRepository
	subscriptionRepo domain.SubscriptionRepository
	planRepo         domain.PlanRepository
	producer         producer.Producer
	log              zerolog.Logger
}

func NewUsageService(
	usageRepo domain.UsageRepository,
	subscriptionRepo domain.SubscriptionRepository,
	planRepo domain.PlanRepository,
	producer producer.Producer,
	log zerolog.Logger,
) *usageService {
	return &usageService{
		usageRepo:        usageRepo,
		subscriptionRepo: subscriptionRepo,
		planRepo:         planRepo,
		producer:         producer,
		log:              log,
	}
}

func (s *usageService) CheckLimit(ctx context.Context, workspaceID, environmentID uuid.UUID, channel string) (*domain.CheckLimitResult, error) {
	subscription, err := s.getOrRenewSubscription(ctx, workspaceID)
	if err != nil {
		return nil, apperrors.ErrNoActiveSubscription
	}

	plan, err := s.planRepo.GetPlanByID(ctx, subscription.PlanID)
	if err != nil {
		return nil, apperrors.ErrPlanNotFound
	}

	usage, err := s.usageRepo.GetUsageByChannel(ctx, workspaceID, environmentID, channel)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			usage = &domain.Usage{CurrentUsage: 0}
		} else {
			return nil, err
		}
	}

	limit := getLimitByChannel(plan, channel)

	res := &domain.CheckLimitResult{
		Allowed: true,
		Reason:  constants.ReasonAllowed,
		Limit:   limit,
		Current: usage.CurrentUsage,
		ResetAt: subscription.CurrentPeriodEnd,
	}

	if limit < 0 {
		res.Reason = constants.ReasonUnlimited
		return res, nil
	}

	if usage.CurrentUsage >= int64(limit) {
		res.Allowed = false
		res.Reason = constants.ReasonLimitReached
		return res, apperrors.ErrLimitReached
	}

	return res, nil
}

func (s *usageService) RecordUsage(ctx context.Context, input domain.UpsertProviderUsageInput) error {
	subscription, err := s.getOrRenewSubscription(ctx, input.WorkspaceID)
	if err != nil {
		return err
	}

	resetAt := subscription.CurrentPeriodEnd
	input.ResetAt = resetAt

	if err := s.usageRepo.UpsertProviderUsage(ctx, input); err != nil {
		return err
	}

	usage, err := s.usageRepo.UpsertWorkSpaceUsage(ctx, domain.UpsertUsageInput{
		WorkspaceID:   input.WorkspaceID,
		EnvironmentID: input.EnvironmentID,
		ChannelName:   input.ChannelName,
		ResetAt:       resetAt,
	})
	if err != nil {
		return err
	}

	plan, err := s.planRepo.GetPlanByID(ctx, subscription.PlanID)
	if err != nil {
		return nil
	}

	limit := getLimitByChannel(plan, input.ChannelName)
	if limit <= 0 {
		return nil
	}

	percentage := (float64(usage.CurrentUsage) / float64(limit)) * 100

	if percentage >= 100 && !usage.Limit100Sent {
		s.publishLimitEvent(ctx, usage, constants.EventSubscriptionLimitReached100)
		if err := s.usageRepo.SetLimit100Sent(ctx, usage.ID); err != nil {
			s.log.Error().Err(err).Str("usage_id", usage.ID.String()).Msg("failed to mark 100% limit alert as sent")
		}
	} else if percentage >= 80 && !usage.Limit80Sent {
		s.publishLimitEvent(ctx, usage, constants.EventSubscriptionLimitReached80)
		if err := s.usageRepo.SetLimit80Sent(ctx, usage.ID); err != nil {
			s.log.Error().Err(err).Str("usage_id", usage.ID.String()).Msg("failed to mark 80% limit alert as sent")
		}
	}

	return nil
}

func (s *usageService) publishLimitEvent(ctx context.Context, usage *domain.Usage, eventType string) {
	event := map[string]interface{}{
		"workspace_id":   usage.WorkspaceID.String(),
		"environment_id": usage.EnvironmentID.String(),
		"event_type":     eventType,
		"is_system":      true,
		"data": map[string]string{
			"channel":  usage.ChannelName,
			"current":  fmt.Sprintf("%d", usage.CurrentUsage),
			"reset_at": usage.ResetAt.Format(time.RFC3339),
		},
	}
	if err := s.producer.Publish(ctx, constants.TopicSystemNotification, event); err != nil {
		s.log.Error().Err(err).Str("event_type", eventType).Msg("failed to publish limit alert to kafka")
	}
}

func (s *usageService) GetUsageSummary(ctx context.Context, workspaceID, environmentID uuid.UUID) ([]*domain.Usage, error) {
	return s.usageRepo.GetUsage(ctx, workspaceID, environmentID)
}

func (s *usageService) GetProviderUsageSummary(ctx context.Context, workspaceID, environmentID uuid.UUID) ([]*domain.ProviderUsage, error) {
	return s.usageRepo.GetProviderUsage(ctx, workspaceID, environmentID)
}

func (s *usageService) getOrRenewSubscription(ctx context.Context, workspaceID uuid.UUID) (*domain.Subscription, error) {
	subscription, err := s.subscriptionRepo.GetActive(ctx, workspaceID)
	if err != nil {
		if !errors.Is(err, apperrors.ErrNotFound) {
			return nil, err
		}

		subscription, err = s.subscriptionRepo.RenewExpiredFreeSubscription(ctx, workspaceID)
		if err != nil {
			return nil, apperrors.ErrNoActiveSubscription
		}
	}
	return subscription, nil
}

func getLimitByChannel(plan *domain.Plan, channel string) int32 {
	switch channel {
	case "email":
		return plan.EmailLimitMonth
	case "sms":
		return plan.SmsLimitMonth
	case "push":
		return plan.PushLimitMonth
	case "slack":
		return plan.SlackLimitMonth
	case "whatsapp":
		return plan.WhatsappLimitMonth
	case "webhook":
		return plan.WebhookLimitMonth
	case "in_app":
		return plan.InAppLimitMonth
	default:
		return 0
	}
}
