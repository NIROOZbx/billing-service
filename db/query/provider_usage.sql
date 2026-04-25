-- name: UpsertProviderUsage :exec
INSERT INTO billing.provider_usage (
    workspace_id, environment_id, channel_config_id, 
    provider_name, channel_name, success_count, failure_count, reset_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
ON CONFLICT (workspace_id, environment_id, provider_name, channel_name, reset_at)
DO UPDATE SET 
    success_count = billing.provider_usage.success_count + EXCLUDED.success_count,
    failure_count = billing.provider_usage.failure_count + EXCLUDED.failure_count;

-- name: GetProviderUsageByWorkspace :many
SELECT * FROM billing.provider_usage
WHERE workspace_id = $1 AND environment_id = $2 AND reset_at > NOW();
