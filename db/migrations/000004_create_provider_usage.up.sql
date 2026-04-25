CREATE TABLE billing.provider_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES public.workspaces(id),
    environment_id UUID NOT NULL REFERENCES public.environments(id),
    channel_config_id UUID NOT NULL references public.channel_configs(id),
    provider_name varchar(100) not null,
    channel_name VARCHAR(50) NOT NULL,
    success_count BIGINT NOT NULL DEFAULT 0,
    failure_count BIGINT NOT NULL DEFAULT 0,
    reset_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (workspace_id, environment_id, provider_name, channel_name, reset_at)
);

CREATE INDEX idx_provider_usage_lookup ON billing.provider_usage(workspace_id, environment_id, provider_name, channel_name);
