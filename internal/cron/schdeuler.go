package cron

import (
	"context"
	"time"

	"github.com/NIROOZbx/billing-service/internal/domain"
	"github.com/NIROOZbx/billing-service/internal/producer"
	"github.com/NIROOZbx/billing-service/pkg/constants"
	"github.com/rs/zerolog"
)

type Scheduler struct {
	repo     domain.SubscriptionRepository
	log      zerolog.Logger
	interval time.Duration
	producer producer.Producer
}

func NewScheduler(repo domain.SubscriptionRepository, log zerolog.Logger, interval time.Duration,producer producer.Producer) *Scheduler {
	return &Scheduler{
		repo:     repo,
		log:      log,
		interval: interval,
		producer: producer,
	}
}

func (s *Scheduler) pollExpiringSubscription(ctx context.Context) {
	subs, err := s.repo.GetExpiringSubscription(ctx, 15)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to fetch expiring subscriptions")
		return
	}

	if len(subs) == 0 {
		return
	}

	for _, sub := range subs {
		s.log.Info().
			Str("subscription_id", sub.ID.String()).
			Str("workspace_id", sub.WorkspaceID.String()).
			Time("expires_at", sub.CurrentPeriodEnd).
			Msg("sending expiry notification")

		if err := s.repo.MarkExpiryEmailSent(ctx, sub.ID); err != nil {
			s.log.Error().
				Err(err).
				Str("subscription_id", sub.ID.String()).
				Msg("failed to mark expiry notification as sent")
			continue
		}

		event := map[string]interface{}{
			"workspace_id":   sub.WorkspaceID.String(),
			"environment_id": constants.FallBackUUID,
			"event_type":     constants.EventSubscriptionExpiryReminder,
			"is_system":      true,
			"data": map[string]string{
				"subscription_id": sub.ID.String(),
				"expiry_date":     sub.CurrentPeriodEnd.Format(time.RFC3339),
			},
		}

		if err := s.producer.Publish(ctx, constants.TopicSystemNotification, event); err != nil {
			s.log.Error().
				Err(err).
				Str("subscription_id", sub.ID.String()).
				Msg("failed to publish expiry notification to kafka")
		}
	}
}


func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	s.log.Info().Dur("interval", s.interval).Msg("scheduler started")

	for {
		select {
		case <-ticker.C:
			s.pollExpiringSubscription(ctx)
		case <-ctx.Done():
			s.log.Info().Msg("scheduler stopped")
			return
		}
	}
}