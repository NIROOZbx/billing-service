-- name: GetUsageByWorkspace :many
SELECT * FROM billing.usage
WHERE workspace_id = $1 AND environment_id = $2 AND reset_at > NOW();

-- name: UpsertUsage :one
INSERT INTO billing.usage (
    workspace_id, environment_id, channel_name, current_usage, reset_at
) VALUES (
    $1, $2, $3, $4, $5
)
ON CONFLICT (workspace_id, environment_id, channel_name, reset_at)
DO UPDATE SET 
    current_usage = billing.usage.current_usage + EXCLUDED.current_usage
RETURNING *;

-- name: GetUsageByChannel :one
SELECT * FROM billing.usage
WHERE workspace_id = $1 AND environment_id = $2 AND channel_name = $3 AND reset_at > NOW();

-- name: SetLimit80Sent :exec
UPDATE billing.usage SET limit_80_sent = true WHERE id = $1;

-- name: SetLimit100Sent :exec
UPDATE billing.usage SET limit_100_sent = true WHERE id = $1;
