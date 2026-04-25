DROP TRIGGER IF EXISTS update_provider_usage_updated_at ON billing.provider_usage;
DROP TRIGGER IF EXISTS update_usage_updated_at ON billing.usage;
DROP TRIGGER IF EXISTS update_subscriptions_updated_at ON billing.subscriptions;

DROP FUNCTION IF EXISTS update_updated_at_column();
