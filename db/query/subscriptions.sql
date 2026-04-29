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
-- name: SyncSubscription :execresult
UPDATE billing.subscriptions
SET status = $2,
  current_period_start = $3,
  current_period_end = $4,
  cancelled_at = $5
WHERE external_subscription_id = $1;
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