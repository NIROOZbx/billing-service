package stripe

import (
	"fmt"
	"net/http"
	"time"

	"github.com/NIROOZbx/billing-service/internal/domain"
	"github.com/NIROOZbx/billing-service/pkg/constants"
	"github.com/NIROOZbx/billing-service/pkg/helpers"
	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	stripe "github.com/stripe/stripe-go/v85"
	"github.com/stripe/stripe-go/v85/checkout/session"
	"github.com/stripe/stripe-go/v85/subscription"
	"github.com/stripe/stripe-go/v85/webhook"
)

type StripeProvider struct {
	webhookSecret string
}

func NewStripeProvider(secret string) *StripeProvider {
	return &StripeProvider{webhookSecret: secret}
}

func (p *StripeProvider) ParseEvent(body []byte, header http.Header) (*domain.BillingEvent, error) {
	signature := header.Get("Stripe-Signature")

	event, err := webhook.ConstructEvent(body, signature, p.webhookSecret)
	if err != nil {
		return nil, fmt.Errorf("invalid stripe signature: %w", err)
	}

	switch event.Type {
	case "checkout.session.completed":
		return p.handleCheckoutSession(event)
	case "customer.subscription.updated":
		return p.handleSubscriptionUpdated(event)
	case "customer.subscription.deleted":
		return p.handleSubscriptionDeleted(event)
	case "invoice.payment_succeeded":
		return p.handleInvoicePaymentSucceeded(event)
	case "invoice.payment_failed":
		return p.handleInvoicePaymentFailed(event)
	default:
		return nil, nil
	}
}
func (p *StripeProvider) CreateCheckoutSession(params domain.CheckoutSessionParams) (string, error) {
	param := &stripe.CheckoutSessionParams{
		SuccessURL:        stripe.String(params.SuccessURL),
		CancelURL:         stripe.String(params.CancelURL),
		CustomerEmail:     stripe.String(params.CustomerEmail),
		ClientReferenceID: stripe.String(params.WorkspaceID),
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		Metadata: map[string]string{
			"plan_id": params.PlanID,
		},
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(params.PriceID),
				Quantity: stripe.Int64(1),
			},
		},
	}

	session, err := session.New(param)
	if err != nil {
		return "", fmt.Errorf("could not create stripe checkout session: %w", err)
	}

	return session.URL, nil
}

func (p *StripeProvider) handleCheckoutSession(event stripe.Event) (*domain.BillingEvent, error) {
	var session stripe.CheckoutSession
	var workspaceID, planID uuid.UUID

	if err := sonic.Unmarshal(event.Data.Raw, &session); err != nil {
		return nil, fmt.Errorf("unmarshal checkout session: %w", err)
	}
	if err := helpers.ParseUUIDs(
		helpers.UUIDField{Value: session.ClientReferenceID, Name: "workspace id", Dest: &workspaceID},
		helpers.UUIDField{Value: session.Metadata["plan_id"], Name: "plan id", Dest: &planID},
	); err != nil {
		return nil, fmt.Errorf("invalid metadata variables: %w", err)
	}

	stripeSub, err := subscription.Get(session.Subscription.ID, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch stripe subscription: %w", err)
	}

	return &domain.BillingEvent{
		Type: domain.EventSubscriptionCreated,
		Subscription: &domain.SubscriptionEvent{
			ExternalSubscriptionID: stripeSub.ID,
			ExternalCustomerID:     session.Customer.ID,
			WorkspaceID:            workspaceID.String(),
			PlanID:                 planID.String(),
			PaymentProvider:        constants.ProviderStripe,
		},
	}, nil
}

func (p *StripeProvider) handleSubscriptionUpdated(event stripe.Event) (*domain.BillingEvent, error) {
	var sub stripe.Subscription
	if err := sonic.Unmarshal(event.Data.Raw, &sub); err != nil {
		return nil, fmt.Errorf("unmarshal stripe subscription: %w", err)
	}

	var cancelledAt *time.Time
	if sub.CanceledAt > 0 {
		t := helpers.UnixToTime(sub.CanceledAt)
		cancelledAt = &t
	}
	if len(sub.Items.Data) == 0 {
		return nil, fmt.Errorf("subscription has no items: %s", sub.ID)
	}

	return &domain.BillingEvent{
		Type: domain.EventSubscriptionUpdated,
		Subscription: &domain.SubscriptionEvent{
			ExternalSubscriptionID: sub.ID,
			Status:                 mapStripeStatus(sub.Status),
			CurrentPeriodStart:     helpers.UnixToTime(sub.Items.Data[0].CurrentPeriodStart),
			CurrentPeriodEnd:       helpers.UnixToTime(sub.Items.Data[0].CurrentPeriodEnd),
			CancelledAt:            cancelledAt,
		},
	}, nil
}

func (p *StripeProvider) handleSubscriptionDeleted(event stripe.Event) (*domain.BillingEvent, error) {
	var sub stripe.Subscription
	if err := sonic.Unmarshal(event.Data.Raw, &sub); err != nil {
		return nil, fmt.Errorf("unmarshal stripe subscription: %w", err)
	}

	cancelledAt := time.Unix(sub.CanceledAt, 0)

	if len(sub.Items.Data) == 0 {
		return nil, fmt.Errorf("subscription has no items: %s", sub.ID)
	}

	return &domain.BillingEvent{
		Type: domain.EventSubscriptionCancelled,
		Subscription: &domain.SubscriptionEvent{
			ExternalSubscriptionID: sub.ID,
			Status:                 constants.SubscriptionStatusCancelled,
			CurrentPeriodStart:     helpers.UnixToTime(sub.Items.Data[0].CurrentPeriodStart),
			CurrentPeriodEnd:       helpers.UnixToTime(sub.Items.Data[0].CurrentPeriodEnd),
			CancelledAt:            &cancelledAt,
		},
	}, nil
}

func (p *StripeProvider) handleInvoicePaymentSucceeded(event stripe.Event) (*domain.BillingEvent, error) {
	var inv stripe.Invoice
	if err := sonic.Unmarshal(event.Data.Raw, &inv); err != nil {
		return nil, fmt.Errorf("unmarshal stripe invoice: %w", err)
	}
	if inv.Parent == nil || inv.Parent.SubscriptionDetails == nil || inv.Parent.SubscriptionDetails.Subscription == nil {
		return nil, nil
	}

	sub := inv.Parent.SubscriptionDetails.Subscription

	return &domain.BillingEvent{
		Type: domain.EventPaymentSucceeded,
		Subscription: &domain.SubscriptionEvent{
			ExternalSubscriptionID: sub.ID,
			Status:                 constants.SubscriptionStatusActive,
			CurrentPeriodStart:     helpers.UnixToTime(inv.PeriodStart),
			CurrentPeriodEnd:       helpers.UnixToTime(inv.PeriodEnd),
		},
	}, nil
}

func (p *StripeProvider) handleInvoicePaymentFailed(event stripe.Event) (*domain.BillingEvent, error) {
	var inv stripe.Invoice
	if err := sonic.Unmarshal(event.Data.Raw, &inv); err != nil {
		return nil, fmt.Errorf("unmarshal stripe invoice: %w", err)
	}
	if inv.Parent == nil || inv.Parent.SubscriptionDetails == nil || inv.Parent.SubscriptionDetails.Subscription == nil {
		return nil, nil
	}

	sub := inv.Parent.SubscriptionDetails.Subscription

	return &domain.BillingEvent{
		Type: domain.EventPaymentFailed,
		Subscription: &domain.SubscriptionEvent{
			ExternalSubscriptionID: sub.ID,
			Status:                 constants.SubscriptionStatusPastDue,
			CurrentPeriodStart:     helpers.UnixToTime(inv.PeriodStart),
			CurrentPeriodEnd:       helpers.UnixToTime(inv.PeriodEnd),
		},
	}, nil
}

func mapStripeStatus(status stripe.SubscriptionStatus) string {
	switch status {
	case stripe.SubscriptionStatusActive:
		return constants.SubscriptionStatusActive
	case stripe.SubscriptionStatusPastDue:
		return constants.SubscriptionStatusPastDue
	case stripe.SubscriptionStatusTrialing:
		return constants.SubscriptionTrialing
	case stripe.SubscriptionStatusCanceled:
		return constants.SubscriptionStatusCancelled
	default:
		return constants.SubscriptionStatusActive
	}
}
