CREATE TABLE billing.subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL references public.workspace(id),
    plan_id UUID NOT NULL references public.plans(id),
    payment_provider varchar(255) not null ,
    external_subscription_id varchar(255),
    external_customer_id varchar(255),
    status VARCHAR(50) NOT NULL DEFAULT 'active' CHECK (
        status IN ('active', 'cancelled', 'past_due', 'trialing')
    ),
     provider_metadata JSONB DEFAULT '{}',
    current_period_start TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    current_period_end TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '30 days',
    cancelled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_subscriptions_external_id 
ON billing.subscriptions(external_subscription_id);

CREATE UNIQUE INDEX idx_subscriptions_active ON billing.subscriptions(workspace_id) 
WHERE status = 'active';

CREATE INDEX idx_subscriptions_workspace_id ON billing.subscriptions(workspace_id);