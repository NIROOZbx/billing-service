# Architecture & Design Decision Records (ADR)

This document outlines the architectural decisions and design patterns implemented in the Billing Service.

---

## 1. Multi-Tenant Isolation via (Workspace, Environment, Channel)

**Decision**: Usage is tracked at the `(workspace_id, environment_id, channel_name)` granularity, enforced by a composite `UNIQUE` constraint on `billing.usage`.

**Rationale**: A single workspace operates multiple environments (Dev, Stage, Production). Load-testing in Development must not consume the Production email or SMS quota and cause service outages for real end-users. Further decomposition by `channel_name` allows independent tracking of email, SMS, push, and other notification types within the same environment.

**Code reference**: `db/migrations/000003_create_usage.up.sql:10`, `internal/domain/usage.go:20`

---

## 2. Decoupled Usage and Provider Usage Tracking

**Decision**: Separate `billing.usage` and `billing.provider_usage` tables updated simultaneously during `RecordUsage`.

**Rationale**:
- `billing.usage` handles **Customer Billing** — how many emails did the plan allow and how many were sent?
- `billing.provider_usage` handles **Infrastructure Monitoring** — how many emails did SendGrid (or Twilio) actually deliver vs. fail?

This separation prevents provider-level failure metrics from polluting the customer's billable count, while still enabling margin analysis and provider health monitoring.

**Code reference**: `internal/services/usage.go:94-106`, `internal/repositories/usage.go:66-107`

---

## 3. Atomic Upsert Pattern for Usage Increments

**Decision**: Use `INSERT ... ON CONFLICT DO UPDATE` with `RETURNING *` for all usage increments.

**Rationale**: In a high-volume system, hundreds of `RecordUsage` calls arrive concurrently for the same workspace, environment, and channel. Atomic database upserts prevent lost counts from race conditions without requiring distributed locks or application-level serialization. Using `RETURNING *` eliminates a separate SELECT round-trip by returning the updated record in a single query.

**SQL reference**:
```sql
INSERT INTO billing.usage (...) VALUES (...)
ON CONFLICT (workspace_id, environment_id, channel_name, reset_at)
DO UPDATE SET current_usage = billing.usage.current_usage + EXCLUDED.current_usage
RETURNING *;
```

**Code reference**: `db/query/usage.sql:5-14`, `db/query/provider_usage.sql:1-11`

---

## 4. Zero-Usage as Default in CheckLimit

**Decision**: A missing row in `billing.usage` is treated as zero usage, not an error.

**Rationale**: New workspaces have no usage rows until their first notification is sent. Returning `ErrNotFound` caused spurious "billing check failed" warnings in the Notification Engine. Since zero usage is always within any plan's limit, treating missing rows as zero is the correct and safe default.

**Code reference**: `internal/services/usage.go:53-56`

---

## 5. Fail-Safe (Best-Effort) Alerting

**Decision**: Kafka event publishing and database dedup flag updates in the alert pipeline are "best-effort" — failures are logged via `zerolog.Error()` but never block or fail the primary `RecordUsage` operation.

**Rationale**: The core value of `RecordUsage` is accurately counting consumption. A transient Kafka broker outage or database blip must not cause usage data loss. Structured error logs are written for all secondary failures so they can be monitored, alerted on, and retried externally.

**Code reference**: `internal/services/usage.go:120-130`, `internal/services/usage.go:135-149`

---

## 6. Provider-Agnostic Subscription Design

**Decision**: Use generic column names (`external_subscription_id`, `payment_provider`) with a `JSONB` metadata column for provider-specific data.

**Rationale**: Allows switching from Stripe to Paddle, LemonSqueezy, or an in-house system without a schema migration. `provider_metadata` stores data that doesn't fit the core normalized schema. The `payment_provider` discriminator column enables provider-specific routing in the service layer.

**Code reference**: `db/migrations/000002_create_subscriptions.up.sql:4-11`

---

## 7. Partial Unique Index for One Active Subscription Per Workspace

**Decision**: `CREATE UNIQUE INDEX idx_subscriptions_active ON billing.subscriptions(workspace_id) WHERE status = 'active'`.

**Rationale**: Enforces the business rule "exactly one active subscription per workspace" at the database level while allowing an unlimited history of cancelled, past_due, or expired subscriptions in the same table. This is more expressive and performant than application-level checks.

**Code reference**: `db/migrations/000002_create_subscriptions.up.sql:19-20`

---

## 8. Event-Driven Alert Architecture

**Decision**: Billing alerts (expiry reminders, usage limit thresholds) are published as events to the `system.notifications` Kafka topic rather than sending emails or push notifications directly.

**Rationale**: The Billing Service has no knowledge of email templates, provider credentials, user preferences, or workspace owner resolution. Publishing to Kafka decouples billing concerns from notification delivery. The Notification Engine consumes these events and handles template rendering, environment resolution, workspace owner lookup, and multi-channel delivery (email, in-app, etc.).

**Code reference**: `pkg/constants/kafka.go:4-11`, `internal/services/usage.go:135-149`, `internal/cron/schdeuler.go:55-66`

---

## 9. FallBackUUID for Environment Resolution

**Decision**: System-triggered events (e.g., the expiry scheduler) send `environment_id = "00000000-0000-0000-0000-000000000000"` (`FallBackUUID`) as a signal.

**Rationale**: The background cron scheduler has no request context and cannot know which environment a workspace considers "primary" for admin notifications. Sending `FallBackUUID` signals the Notification Engine to resolve the workspace's Production environment automatically. Events triggered by real API calls (e.g., usage limit alerts) always carry the actual `environment_id` from the request.

**Code reference**: `pkg/constants/kafka.go:5`, `internal/cron/schdeuler.go:58`

---

## 10. Communication Pattern — Strict Decoupling

The Billing Service follows a strict communication model:

```
Notification Engine ──gRPC──→ Billing Service   (CheckLimit, RecordUsage, GetUsage)
Billing Service     ──Kafka──→ Notification Eng  (Events: expiry, limit alerts)
Stripe              ──HTTP──→ Billing Service   (Webhooks: subscription updates)
```

The Engine is never pushed to directly by the Billing Service. All proactive communication goes through Kafka, keeping both services fully decoupled and independently deployable.

**Code reference**: `internal/handlers/grpc.go`, `internal/handlers/webhook.go`, `internal/producer/producer.go`

---

## 11. Domain-Driven Layered Architecture

**Decision**: Strict four-layer separation with inward dependency rules:

| Layer | Responsibility | Depends On |
|---|---|---|
| `domain/` | Pure Go models & interfaces | Nothing (no infra imports) |
| `repositories/` | Domain → SQLC mapping, DB specifics | `domain/`, `db/sqlc/`, `pkg/` |
| `services/` | Business logic orchestration | `domain/`, `producer/` |
| `handlers/` | gRPC + HTTP transport | `domain/`, `services/` |

**Rationale**: Enables testability (domain models have no infrastructure imports), swapability (databases, message queues), and clear separation of concerns. The `pkg/apperrors` package centralizes DB error → domain error translation.

**Code reference**: `internal/domain/`, `internal/repositories/`, `internal/services/`, `internal/handlers/`

---

## 12. Bucket Usage Pattern (Lazy Rollover, No Cron Jobs)

**Decision**: Usage records are keyed by `reset_at` (mirroring the subscription's `current_period_end`). The upsert's `ON CONFLICT` clause handles rollover automatically.

**Rationale**: Instead of running expensive midnight cron jobs to reset usage counters, each usage row belongs to a specific billing period bucket. When a subscription renews or upgrades, the new `reset_at` value causes the `ON CONFLICT` to miss (no matching row), and PostgreSQL creates a fresh usage row — a new bucket — while the old one remains for historical records. This is zero-maintenance and handles plan changes, upgrades, and downgrades seamlessly.

**SQL reference**: `db/query/usage.sql:5-14`
**Code reference**: `internal/services/usage.go:91-103`

---

## 13. Date is the Boss — Time-Based Access Enforcement

**Decision**: The `GetActiveSubscription` query filters by `current_period_end > NOW()`. A subscription's `status` column indicates billing intent, but real-time access is gated by the clock.

**Rationale**: A user who cancels their subscription still has access until the end of their paid billing period. A user whose payment is past_due may retain access temporarily. By enforcing access via `current_period_end`, the system respects the "paid until" semantic naturally and consistently.

**SQL reference**: `db/query/subscriptions.sql:1-8`
**Code reference**: `internal/repositories/subscription.go:23-30`

---

## 14. Lazy Free Subscription Renewal

**Decision**: When an expired `system`-provider subscription is encountered during `CheckLimit` or `RecordUsage`, it is automatically renewed for another 30 days via `RenewExpiredFreeSubscription`.

**Rationale**: New workspaces start with a free system-managed subscription. Rather than requiring a separate cron job or manual activation, the first usage call transparently renews it. The query targets `payment_provider = 'system'` so paid Stripe subscriptions are never auto-renewed without payment.

**SQL reference**: `db/query/subscriptions.sql:32-39`
**Code reference**: `internal/services/usage.go:160-173`

---

## 15. Stripe Webhook Idempotency via Upsert

**Decision**: The `SyncSubscription` query uses `INSERT ... ON CONFLICT (external_subscription_id) DO UPDATE` to handle Stripe webhook events.

**Rationale**: Stripe delivers webhooks with at-least-once semantics — the same event may arrive multiple times. The upsert pattern makes processing idempotent: repeated `customer.subscription.updated` or `invoice.payment_succeeded` events simply re-apply the same state without creating duplicate rows or raising errors.

**SQL reference**: `db/query/subscriptions.sql:40-57`
**Code reference**: `internal/repositories/subscription.go:80-105`

---

## 16. Database Connection Pool Tuning for Supabase Compatibility

**Decision**: Tight connection pool limits (`max_open_conns: 5`, `min_open_conns: 1`, `max_idle_conns: 2`) combined with Supavisor Session Mode proxying and 60-second connection queuing.

**Rationale**: Supabase projects limit concurrent connections to a relatively low number (e.g., 60 on paid tiers, 15 on free). A service with many replicas or high connection churn would quickly saturate database capacity. Tight pool limits combined with Supavisor's queuing absorb usage spikes without failing requests immediately. Ubuntu-style: prefer queueing to crashing.

**Code reference**: `config.yaml:7-12`, `db/postgres.go:21-24`

---

## 17. Graceful Shutdown with Ordered Dependencies

**Decision**: On shutdown signal (`SIGTERM`, `SIGINT`, `SIGQUIT`, `SIGHUP`), shut down HTTP first, then gRPC, with a 10-second timeout.

**Rationale**: HTTP webhooks have shorter timeouts and should be stopped first to prevent processing partial requests. gRPC has active long-running streams that drain via `GracefulStop`. The 10-second deadline prevents indefinite hangs.

**Code reference**: `cmd/cmd.go:19-123`

---

## 18. Structured Logging with Log Rotation

**Decision**: Use `zerolog` for structured JSON logging with `lumberjack` for file rotation in non-production environments, and stdout-only in production.

**Rationale**: Production containers log to stdout for container runtime collection (e.g., CloudWatch, Datadog). Development environments write to both console (colorized, human-readable) and rotating files for post-mortem analysis. UTC timestamps in production, local time in development.

**Code reference**: `pkg/logger/logger.go:14-61`, `config.yaml:14-19`

---

## 19. Stripe Event Routing Strategy

**Decision**: The Stripe provider parses webhook events and maps them to domain events. Unhandled event types return `nil` (no-op). Handled events:

| Stripe Event | Domain Event | Action |
|---|---|---|
| `checkout.session.completed` | `EventSubscriptionCreated` | Fetches subscription from Stripe, creates local subscription |
| `customer.subscription.updated` | `EventSubscriptionUpdated` | Syncs status, period dates, cancellation |
| `customer.subscription.deleted` | `EventSubscriptionCancelled` | Marks as cancelled with period dates |
| `invoice.payment_succeeded` | `EventPaymentSucceeded` | Syncs status to active, updates period |
| `invoice.payment_failed` | `EventPaymentFailed` | Syncs status to past_due |

**Code reference**: `internal/stripe/client.go:32-57`

---

## 20. Plan Resolution by Name or ID

**Decision**: `CreateSubscription` accepts both a UUID `plan_id` and a plan name string. Names are resolved to IDs via `GetPlanByName`.

**Rationale**: Simplifies client integration. The Notification Engine and testing tools may know a plan by its human-readable name (`"free"`, `"pro"`) without needing to look up the UUID separately. UUIDs remain the canonical identifier for internal consistency.

**Code reference**: `internal/handlers/grpc.go:130-139`

---

## 21. Config Layering and Validation

**Decision**: Configuration is loaded from `config.yaml` with `.env` file and environment variable overrides via Viper. Required fields are validated at startup with `log.Fatalf`.

**Rationale**: Static defaults in YAML, secrets in `.env` (gitignored), and environment variable overrides for containerized deployments. Startup validation catches misconfiguration immediately rather than failing at runtime under load.

**Code reference**: `config/config.go:61-128`

---

## 22. gRPC Error Classification

**Decision**: Domain errors are mapped to standard gRPC status codes in a centralized `mapGRPCError` function.

| Domain Error | gRPC Code | Usage |
|---|---|---|
| `ErrNoActiveSubscription` | `PermissionDenied` | Workspace has no active subscription |
| `ErrLimitReached` | `ResourceExhausted` | Channel usage at or above plan limit |
| `ErrNotFound` / `ErrPlanNotFound` | `NotFound` | Resource doesn't exist |
| `ErrAlreadyCancelled` | `FailedPrecondition` | Subscription already in cancelled state |
| Invalid UUID input | `InvalidArgument` | Malformed request parameters |
| All other errors | `Internal` | Unexpected server failures |

This classification allows gRPC clients (including the Notification Engine) to implement proper retry, circuit-breaking, and fallback logic based on error codes rather than parsing error messages.

**Code reference**: `internal/handlers/grpc.go:331-348`

---

## 23. GitOps with Argo CD

**Decision**: Deploy the application using Argo CD tracking Helm chart configurations committed to the Git repository.

**Rationale**: Eliminates manual deployment steps and sync errors. Git becomes the single source of truth for cluster state, enabling:
- **Auditability**: Every change is a Git commit with a full history
- **Rollbacks**: `argocd app rollback` or `git revert` to restore previous state
- **Self-healing**: Argo CD automatically corrects cluster drift
- **Multi-environment**: `values.yaml` (dev) and `values-prod.yaml` (prod) with different image tags and replica counts

**Code reference**: `deployments/helm/billing-service/`, `.github/workflows/deploy.yml`

---

## 24. Keyless OIDC for CI/CD Authentication

**Decision**: Use GitHub Actions OIDC (`sts:AssumeRoleWithWebIdentity`) for AWS authentication instead of static Access/Secret keys in GitHub Secrets.

**Rationale**: Mitigates risk by eliminating long-lived credentials. Each CI/CD run receives a short-lived, scoped token dynamically. There are no secrets to rotate, leak, or manage. The IAM role trust policy restricts which GitHub organizations, repositories, and branches can assume the role.

**Code reference**: `.github/workflows/deploy.yml`

---

## 25. Kafka Producer Configuration

**Decision**: Kafka producer uses `LeastBytes` balancer, `RequireAll` ACKs, and configurable batch size/timeout.

**Rationale**: `LeastBytes` distributes messages to the partition with the smallest data volume, improving broker load distribution. `RequireAll` ensures all in-sync replicas acknowledge before the write is considered successful, preventing data loss during broker failures. Batching improves throughput.

**Code reference**: `internal/producer/producer.go:21-31`, `config.yaml:21-24`

---

## 26. Auto-UpdatedAt Triggers

**Decision**: All three billing tables share a common `update_updated_at_column()` trigger function that sets `updated_at = NOW()` on every row update.

**Rationale**: Ensures the `updated_at` column is always accurate without relying on application code to set it. The shared function prevents drift between tables and is a single point of maintenance.

**SQL reference**: `db/migrations/000005_add_updated_at_trigger.up.sql:1-19`

---

## 27. Dedup Flags for Alert Events

**Decision**: `expiry_3d_sent` on `billing.subscriptions` and `limit_80_sent`/`limit_100_sent` on `billing.usage` prevent duplicate alert publishing.

**Rationale**: Without dedup flags, the expiry scheduler would publish a `subscription_expiry_reminder` every polling cycle (every hour) for the same subscription. Similarly, `RecordUsage` could be called many times per second, triggering repeated limit alerts. These boolean flags, set after successful Kafka publishing (best-effort), ensure each threshold event is published at most once per billing period.

**SQL reference**: `db/migrations/000006_add_reminder_fields.up.sql:1-6`
**Code reference**: `internal/cron/schdeuler.go:47-53`, `internal/services/usage.go:120-130`

---

## 28. Skaffold for Local Kubernetes Development

**Decision**: Use Skaffold for local Kubernetes development with live file sync and automatic rebuild.

**Rationale**: Skaffold watches source files, rebuilds the Docker image, and updates the K8s deployment automatically on code changes. This eliminates the manual build-push-deploy cycle during development while running in a production-like K8s environment with all dependencies (Postgres, Kafka) available.

**Code reference**: `skaffold.yaml`, `Taskfile.yml:88-91`
