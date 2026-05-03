package handlers

import (
	"context"
	"io"
	"net/http"

	"github.com/NIROOZbx/billing-service/internal/domain"
	"github.com/NIROOZbx/billing-service/pkg/constants"
	"github.com/NIROOZbx/billing-service/pkg/helpers"
	"github.com/rs/zerolog"
)

type WebhookHandler struct {
	provider        domain.BillingProvider
	subscriptionSvc domain.SubscriptionService
	logger          zerolog.Logger
}

func NewWebHookHandler(provider domain.BillingProvider, svc domain.SubscriptionService, logger zerolog.Logger) *WebhookHandler {
	return &WebhookHandler{
		provider:        provider,
		subscriptionSvc: svc,
		logger:          logger,
	}
}

// ==========================================
// 1. ROUTING LAYER
// Reconstructs the event and decides which handler to call.
// ==========================================
func (h *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	bytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error().Err(err).Msg("❌ Failed to read webhook request body")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	event, err := h.provider.ParseEvent(bytes, r.Header)
	if err != nil {
		h.logger.Error().Err(err).Msg("❌ Failed to parse billing event")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if event == nil {
		h.logger.Debug().Msg("☁️ Received unhandled or noise event from provider (Ignoring)")
		w.WriteHeader(http.StatusOK)
		return
	}

	h.logger.Info().
		Str("event_type", string(event.Type)).
		Msg("📥 Received billing event")

	switch event.Type {
	case domain.EventSubscriptionCreated:
		err = h.handleSubscriptionCreated(r.Context(), event.Subscription)
	case domain.EventSubscriptionUpdated:
		err = h.handleSubscriptionUpdated(r.Context(), event.Subscription)
	case domain.EventSubscriptionCancelled:
		err = h.handleSubscriptionCancelled(r.Context(), event.Subscription)
	case domain.EventPaymentSucceeded:
		err = h.handlePaymentSucceeded(r.Context(), event.Subscription)
	case domain.EventPaymentFailed:
		err = h.handlePaymentFailed(r.Context(), event.Subscription)
	default:
		h.logger.Debug().Str("type", string(event.Type)).Msg("Ignored unhandled event")
		w.WriteHeader(http.StatusOK)
		return
	}

	if err != nil {
		h.logger.Error().
			Err(err).
			Str("event_type", string(event.Type)).
			Str("external_subscription_id", event.Subscription.ExternalSubscriptionID).
			Msg("❌ Failed to process billing event")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	h.logger.Info().
		Str("event_type", string(event.Type)).
		Str("external_subscription_id", event.Subscription.ExternalSubscriptionID).
		Msg("✅ Successfully processed billing event")
	w.WriteHeader(http.StatusOK)
}

// ==========================================
// 2. LIFECYCLE HANDLERS (CREATION / UPDATE / CANCEL)
// Handlers for standard subscription lifecycle events.
// ==========================================

func (h *WebhookHandler) handleSubscriptionCreated(ctx context.Context, event *domain.SubscriptionEvent) error {
	h.logger.Info().
		Str("external_subscription_id", event.ExternalSubscriptionID).
		Msg("⏳ Creating new subscription from webhook...")

	_, err := h.subscriptionSvc.Subscribe(ctx, domain.CreateSubscriptionInput{
		WorkspaceID:            helpers.ParseUUID(event.WorkspaceID),
		PlanID:                 helpers.ParseUUID(event.PlanID),
		PaymentProvider:        event.PaymentProvider,
		ExternalCustomerID:     event.ExternalCustomerID,
		ExternalSubscriptionID: event.ExternalSubscriptionID,
	})
	if err != nil {
		return err
	}

	h.logger.Info().
		Str("external_subscription_id", event.ExternalSubscriptionID).
		Msg("🆕 Subscription created successfully")
	return nil
}

func (h *WebhookHandler) handleSubscriptionUpdated(ctx context.Context, event *domain.SubscriptionEvent) error {
	err := h.subscriptionSvc.SyncSubscription(ctx, domain.SyncSubscriptionInput{
		ExternalSubscriptionID: event.ExternalSubscriptionID,
		WorkspaceID:            helpers.ParseUUID(event.WorkspaceID),
		PlanID:                 helpers.ParseUUID(event.PlanID),
		PaymentProvider:        event.PaymentProvider,
		ExternalCustomerID:     event.ExternalCustomerID,
		Status:                 event.Status,
		CurrentPeriodStart:     event.CurrentPeriodStart,
		CurrentPeriodEnd:       event.CurrentPeriodEnd,
		CancelledAt:            event.CancelledAt,
	})

	if err != nil {
		return err
	}

	h.logger.Info().
		Str("external_subscription_id", event.ExternalSubscriptionID).
		Str("status", event.Status).
		Msg("🔄 Subscription updated successfully")
	return nil
}

func (h *WebhookHandler) handleSubscriptionCancelled(ctx context.Context, event *domain.SubscriptionEvent) error {
	err := h.subscriptionSvc.SyncSubscription(ctx, domain.SyncSubscriptionInput{
		ExternalSubscriptionID: event.ExternalSubscriptionID,
		WorkspaceID:            helpers.ParseUUID(event.WorkspaceID),
		PlanID:                 helpers.ParseUUID(event.PlanID),
		PaymentProvider:        event.PaymentProvider,
		ExternalCustomerID:     event.ExternalCustomerID,
		Status:                 constants.SubscriptionStatusCancelled,
		CurrentPeriodStart:     event.CurrentPeriodStart,
		CurrentPeriodEnd:       event.CurrentPeriodEnd,
		CancelledAt:            event.CancelledAt,
	})

	if err != nil {
		return err
	}

	h.logger.Info().
		Str("external_subscription_id", event.ExternalSubscriptionID).
		Msg("🚫 Subscription cancelled successfully")
	return nil
}

// ==========================================
// 3. REVENUE HANDLERS (SUCCESS / FAIL)
// Handlers specifically for recurring charges.
// ==========================================

func (h *WebhookHandler) handlePaymentSucceeded(ctx context.Context, event *domain.SubscriptionEvent) error {
	err := h.subscriptionSvc.SyncSubscription(ctx, domain.SyncSubscriptionInput{
		ExternalSubscriptionID: event.ExternalSubscriptionID,

		WorkspaceID:            helpers.ParseUUID(event.WorkspaceID),
		PlanID:                 helpers.ParseUUID(event.PlanID),
		PaymentProvider:        event.PaymentProvider,
		ExternalCustomerID:     event.ExternalCustomerID,
		Status:                 event.Status,
		CurrentPeriodStart:     event.CurrentPeriodStart,
		CurrentPeriodEnd:       event.CurrentPeriodEnd,
	})

	if err != nil {
		return err
	}

	h.logger.Info().
		Str("external_subscription_id", event.ExternalSubscriptionID).
		Msg("💰 Recurring payment succeeded and subscription synced")
	return nil
}

func (h *WebhookHandler) handlePaymentFailed(ctx context.Context, event *domain.SubscriptionEvent) error {
	err := h.subscriptionSvc.SyncSubscription(ctx, domain.SyncSubscriptionInput{
		ExternalSubscriptionID: event.ExternalSubscriptionID,
		WorkspaceID:            helpers.ParseUUID(event.WorkspaceID),
		PlanID:                 helpers.ParseUUID(event.PlanID),
		PaymentProvider:        event.PaymentProvider,
		ExternalCustomerID:     event.ExternalCustomerID,
		Status:                 event.Status,
		CurrentPeriodStart:     event.CurrentPeriodStart,
		CurrentPeriodEnd:       event.CurrentPeriodEnd,
	})

	if err != nil {
		return err
	}

	h.logger.Warn().
		Str("external_subscription_id", event.ExternalSubscriptionID).
		Str("status", event.Status).
		Msg("⚠️ Payment failed and subscription status updated")
	return nil
}
