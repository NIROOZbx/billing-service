CREATE TABLE billing.usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES public.workspaces(id),
    environment_id UUID NOT NULL REFERENCES public.environments(id),
    channel_name VARCHAR(50) NOT NULL, 
    current_usage BIGINT NOT NULL DEFAULT 0,
    reset_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (workspace_id, environment_id, channel_name, reset_at)
);

CREATE INDEX idx_usage_workspace_channel ON billing.usage(workspace_id, environment_id, channel_name);
