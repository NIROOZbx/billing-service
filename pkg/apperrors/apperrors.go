package apperrors

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrAlreadyExists     = errors.New("already exists")
	ErrInvalidInput      = errors.New("invalid input")
	ErrInternal          = errors.New("internal server error")

	// Billing Specific Errors
	ErrActiveSubscriptionExists   = errors.New("an active subscription already exists for this workspace")
	ErrExternalIDAlreadyExists    = errors.New("this payment provider ID is already linked to another subscription")
	ErrPlanNotFound               = errors.New("the specified plan does not exist")
	ErrLimitReached               = errors.New("monthly limit reached")
	ErrNoActiveSubscription       = errors.New("no active subscription found")
)

// MapDBError is a reusable component that translates raw database errors
// into clean, domain-specific application errors.
func MapDBError(err error) error {
	if err == nil {
		return nil
	}

	// Handle "No Rows" (404 equivalent)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			switch pgErr.ConstraintName {
			case "idx_subscriptions_active":
				return ErrActiveSubscriptionExists
			case "idx_subscriptions_external_id":
				return ErrExternalIDAlreadyExists
			default:
				return ErrAlreadyExists
			}
		case "23503": // foreign_key_violation
			if pgErr.TableName == "subscriptions" && pgErr.ConstraintName == "subscriptions_plan_id_fkey" {
				return ErrPlanNotFound
			}
			return fmt.Errorf("%w: %s", ErrInvalidInput, pgErr.Detail)
		case "23502": // not_null_violation
			return fmt.Errorf("%w: missing required field %s", ErrInvalidInput, pgErr.ColumnName)
		}
	}

	// If it's an error we don't recognize, return it as is or wrap it as internal
	return err
}
