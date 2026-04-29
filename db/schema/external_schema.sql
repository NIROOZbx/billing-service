
CREATE TABLE public.workspaces (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    plan_id UUID NOT NULL
);

CREATE TABLE public.plans (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email_limit_month INTEGER NOT NULL DEFAULT 1000,
    sms_limit_month   INTEGER NOT NULL DEFAULT 100,
    push_limit_month  INTEGER NOT NULL DEFAULT 5000,
    slack_limit_month    INTEGER NOT NULL DEFAULT 500,
    whatsapp_limit_month INTEGER NOT NULL DEFAULT 100,
    webhook_limit_month  INTEGER NOT NULL DEFAULT 1000,
    in_app_limit_month   INTEGER NOT NULL DEFAULT 5000,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    external_price_id VARCHAR(255)
);

CREATE TABLE public.environments (
    id UUID PRIMARY KEY
);

CREATE TABLE public.channel_configs (
    id UUID PRIMARY KEY
);
