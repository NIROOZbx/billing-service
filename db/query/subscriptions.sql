-- name: GetActiveSubscription :one
SELECT *
FROM billing.subscriptions
WHERE workspace_id = $1
  AND current_period_end > NOW()
  AND status IN ('active', 'cancelled', 'past_due')
ORDER BY current_period_end DESC
LIMIT 1;
-- name: CreateSubscription :one
INSERT INTO billing.subscriptions (
    workspace_id,
    plan_id,
    payment_provider,
    external_subscription_id,
    external_customer_id
  )
VALUES ($1, $2, $3, $4, $5)
RETURNING *;
-- name: CancelActiveSubscription :exec
UPDATE billing.subscriptions
SET status = 'cancelled',
  cancelled_at = NOW()
WHERE workspace_id = $1
  AND status = 'active'
  AND current_period_end > NOW();
-- name: CancelSubscription :execresult
UPDATE billing.subscriptions
SET status = 'cancelled',
  cancelled_at = NOW()
WHERE workspace_id = $1
  AND id = $2;
-- name: RenewExpiredFreeSubscription :one
UPDATE billing.subscriptions
SET current_period_start = NOW(),
  current_period_end = NOW() + INTERVAL '30 days'
WHERE workspace_id = $1
  AND current_period_end < NOW()
  AND payment_provider = 'system'
RETURNING *;
-- name: SyncSubscription :exec
INSERT INTO billing.subscriptions (
    external_subscription_id,
    workspace_id,
    plan_id,
    status,
    current_period_start,
    current_period_end,
    cancelled_at,
    payment_provider,
    external_customer_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (external_subscription_id) DO UPDATE SET
    status = EXCLUDED.status,
    current_period_start = EXCLUDED.current_period_start,
    current_period_end = EXCLUDED.current_period_end,
    cancelled_at = EXCLUDED.cancelled_at,
    updated_at = NOW();

-- name: GetExpiringSubscriptions :many
SELECT *
from billing.subscriptions
WHERE current_period_end BETWEEN NOW()
  AND NOW() + INTERVAL '3 days'
  AND status = 'active'
  AND expiry_3d_sent = false
limit $1;
-- name: MarkExpiryEmailSent :exec
UPDATE billing.subscriptions
SET expiry_3d_sent = true
WHERE id = $1;

-- name: GetSubscriptionByExternalID :one
SELECT *
FROM billing.subscriptions
WHERE external_subscription_id = $1
LIMIT 1;

-- name: GetSubscriptionByID :one

SELECT *
FROM billing.subscriptions
WHERE id = $1
LIMIT 1;