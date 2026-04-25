ALTER TABLE billing.subscriptions 
ADD COLUMN expiry_3d_sent boolean default false;

ALTER TABLE billing.usage
ADD COLUMN limit_80_sent boolean default false,
ADD COLUMN limit_100_sent boolean default false;
