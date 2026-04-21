# Architecture & Design Decision Records (ADR)

This document outlines the architectural decisions made for the Billing Service to ensure a scalable, multi-tenant system.

## 1. Multi-Tenant Isolation
**Decision**: Usage is tracked at the `(workspace_id, environment_id)` level.
**Rationale**: In a professional SaaS, a single customer (Workspace) often has multiple environments (Dev, Stage, Prod). We must ensure that load-testing in a `Development` environment does not accidentally consume the `Production` quota, leading to service outages for the customer's end-users.

## 2. Decoupled Usage Tracking
**Decision**: Separation of `billing.usage` and `billing.provider_usage`.
**Rationale**:
- `billing.usage` handles **Customer Billing** (How many emails did I sell them?).
- `billing.provider_usage` handles **Infrastructure Monitoring** (How many emails did SendGrid actually deliver vs fail?).
This allows us to track margins and provider health without polluting the customer's bill with technical failure metrics.

## 3. Atomic "Upsert" Pattern
**Decision**: Use SQL `ON CONFLICT` for usage increments.
**Rationale**: In a high-volume notification system, hundreds of `RecordUsage` calls can arrive simultaneously. Using atomic database increments ensures we never lose a "count" due to race conditions, without needing expensive distributed locks in Redis.

## 4. Provider Agnostic Design
**Decision**: Generic column names like `external_subscription_id` and `payment_provider`.
**Rationale**: To allow switching from Stripe to Paddle or LemonSqueezy without a database migration. We use `JSONB` for provider-specific metadata that doesn't fit the core schema.

## 5. Partial Unique Indexing
**Decision**: `CREATE UNIQUE INDEX ... WHERE status = 'active'`.
**Rationale**: To enforce the business rule "One active subscription per workspace" while still allowing an unlimited history of cancelled or expired subscriptions.

## 6. Communication Flow
The Billing Service follows a unidirectional "Report & Ask" pattern:
- **Engine asks Billing**: "Am I allowed to send?" (`CheckLimit`)
- **Engine reports to Billing**: "I just sent something." (`RecordUsage`)

This keeps the **Notification Engine** stateless and focused on delivery performance.
