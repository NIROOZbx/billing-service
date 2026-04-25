# Architecture & Design Decision Records (ADR)

This document outlines the architectural decisions made for the Billing Service.

## 1. Multi-Tenant Isolation
**Decision**: Usage is tracked at the `(workspace_id, environment_id, channel)` level.  
**Rationale**: A single workspace has multiple environments (Dev, Stage, Prod). Load-testing in Development must not consume the Production quota and cause service outages for end-users.

## 2. Decoupled Usage Tracking
**Decision**: Separate `billing.usage` and `billing.provider_usage` tables.  
**Rationale**:
- `billing.usage` handles **Customer Billing** — how many emails did we sell them?
- `billing.provider_usage` handles **Infrastructure Monitoring** — how many emails did SendGrid actually deliver vs. fail?

This allows tracking provider margins and health without polluting the customer's bill with technical failure metrics.

## 3. Atomic Upsert Pattern
**Decision**: Use `INSERT ... ON CONFLICT DO UPDATE` with `RETURNING *` for usage increments.  
**Rationale**: In a high-volume system, hundreds of `RecordUsage` calls arrive simultaneously. Atomic database increments prevent lost counts from race conditions without needing distributed locks. Using `RETURNING *` reduces round-trips by returning the updated record in a single query.

## 4. Zero-Usage Handling in CheckLimit
**Decision**: A missing usage row is treated as zero usage, not an error.  
**Rationale**: New workspaces have no rows in `billing.usage` until their first notification is sent. Returning `ErrNotFound` was causing spurious "billing check failed" warnings in the Notification Engine. Since zero usage is always within any plan's limit, it is the correct default.

## 5. Fail-Safe Alerting
**Decision**: Kafka event publishing and DB flag updates in the alert pipeline are "best-effort" — failures are logged but never block the primary operation.  
**Rationale**: The core value of `RecordUsage` is accurately counting consumption. A transient Kafka outage must not cause usage data loss. Structured `zerolog` error logs are written for all secondary failures so they can be monitored and retried externally.

## 6. Provider-Agnostic Design
**Decision**: Generic column names (`external_subscription_id`, `payment_provider`) and a `JSONB` metadata column.  
**Rationale**: Allows switching from Stripe to Paddle or LemonSqueezy without a schema migration. Provider-specific data that doesn't fit the core schema is stored in `provider_metadata`.

## 7. Partial Unique Index for Active Subscriptions
**Decision**: `CREATE UNIQUE INDEX ... WHERE status = 'active'`.  
**Rationale**: Enforces the business rule "one active subscription per workspace" while allowing an unlimited history of cancelled or expired subscriptions in the same table.

## 8. Event-Driven Alert Architecture
**Decision**: Billing alerts (expiry, usage limits) are published as events to a `system.notifications` Kafka topic rather than sending emails directly.  
**Rationale**: The Billing Service has no knowledge of email templates, provider credentials, or user preferences. Publishing to Kafka decouples billing concerns from notification delivery. The Notification Engine consumes these events and handles workspace owner resolution, template rendering, and delivery — each service does only what it's responsible for.

## 9. FallBackUUID for Environment Resolution
**Decision**: System-triggered events (e.g., the expiry scheduler) send `environment_id = "00000000-0000-0000-0000-000000000000"` as a signal.  
**Rationale**: The background scheduler has no request context to know which environment a workspace considers "primary". Sending the `FallBackUUID` signals the Notification Engine to resolve the workspace's Production environment automatically. Events triggered by real API calls (e.g., usage limit alerts) always send the actual `environment_id`.

## 10. Communication Pattern
The Billing Service follows a strict communication model:

```
Notification Engine ──→ Billing Service   (gRPC: CheckLimit, RecordUsage)
Billing Service     ──→ Kafka             (Events: expiry, limit alerts)
Stripe              ──→ Billing Service   (Webhooks: subscription updates)
```

The Engine is never pushed to directly by the Billing Service. All proactive communication goes through Kafka, keeping both services fully decoupled.
