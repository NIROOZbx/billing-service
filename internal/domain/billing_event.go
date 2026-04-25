package domain

import (
	"net/http"
	"time"
)

type BillingEventType string

const (
	EventSubscriptionCreated   BillingEventType = "subscription.created"
	EventSubscriptionUpdated   BillingEventType = "subscription.updated"
	EventSubscriptionCancelled BillingEventType = "subscription.cancelled"
	EventPaymentSucceeded      BillingEventType = "payment.succeeded"
	EventPaymentFailed         BillingEventType = "payment.failed"
)

type SubscriptionEvent struct {
	ExternalSubscriptionID string
	ExternalCustomerID     string
	WorkspaceID            string
	PlanID                 string
	Status                 string
	PaymentProvider        string
	CurrentPeriodStart     time.Time
	CurrentPeriodEnd       time.Time
	CancelledAt            *time.Time
}

type BillingEvent struct {
	Type         BillingEventType
	Subscription *SubscriptionEvent
}

type CheckoutSessionParams struct {
	WorkspaceID string
	PlanID      string
	PriceID     string
	SuccessURL  string
	CancelURL   string
	CustomerEmail string
}

type BillingProvider interface {
	ParseEvent(body []byte, header http.Header) (*BillingEvent, error)
	CreateCheckoutSession(params CheckoutSessionParams) (string, error)
}
