ALTER TABLE billing.subscriptions DROP COLUMN IF EXISTS expiry_3d_sent;
ALTER TABLE billing.usage DROP COLUMN IF EXISTS limit_80_sent;
ALTER TABLE billing.usage DROP COLUMN IF EXISTS limit_100_sent;