# Billing Service

A production-grade microservice for managing subscriptions, tracking resource usage, and proactively notifying workspaces of billing events within a multi-tenant notification ecosystem.

## Overview

The Billing Service acts as the **Accountant**, **Gatekeeper**, and **Alert System** for the Notification Engine. It validates whether a workspace can send notifications (Email, SMS, Push, Slack, WhatsApp, Webhook, In-App) based on plan limits, tracks usage atomically, manages Stripe subscriptions, and publishes billing events to Kafka so the Notification Engine can alert workspace owners in real time.

## Architecture

The service follows a strict **Domain-Driven layering** with clear dependency boundaries:

```
cmd/main.go
  └── cmd/cmd.go (gRPC + HTTP server setup, graceful shutdown)
        └── internal/app/app.go (dependency injection container)
              ├── config/config.go (Viper + godotenv configuration loader)
              ├── db/postgres.go (pgx connection pool)
              ├── db/sqlc/ (SQLC-generated type-safe query layer)
              ├── pkg/logger/ (zerolog + lumberjack structured logging)
              │
              ├── internal/domain/        Pure Go models & interfaces
              │   ├── plan.go             Plan model, PlanRepository/Service interfaces
              │   ├── subscription.go     Subscription model, Create/Sync inputs, interfaces
              │   ├── usage.go            Usage, ProviderUsage, Upsert inputs, interfaces
              │   └── billing_event.go    BillingEvent types, BillingProvider interface
              │
              ├── internal/repositories/  Data access layer (domain -> SQLC mapping)
              │   ├── plan.go
              │   ├── subscription.go
              │   └── usage.go
              │
              ├── internal/services/      Business logic layer
              │   ├── plan.go            PlanService (GetPlanByID, GetPlanByName)
              │   ├── subscription.go     Subscribe, Cancel, Sync, Checkout flows
              │   └── usage.go           CheckLimit, RecordUsage, limit alerting
              │
              ├── internal/handlers/      Transport layer
              │   ├── grpc.go            BillingServer — all gRPC RPC implementations
              │   └── webhook.go         Stripe webhook HTTP handler
              │
              ├── internal/stripe/        Stripe provider implementation
              │   └── client.go          ParseEvent, CreateCheckoutSession, GetCheckoutSession
              │
              ├── internal/producer/      Kafka async event producer
              │   └── producer.go         Interface + segmentio/kafka-go writer
              │
              └── internal/cron/          Background expiry scheduler
                  └── schdeuler.go        Polls expiring subscriptions every 1 hour
```

### Communication Flow

```
Notification Engine ──gRPC──→ Billing Service   (CheckLimit, RecordUsage, GetUsage)
Billing Service     ──Kafka──→ Notification Eng  (Events: expiry, limit alerts)
Stripe              ──HTTP──→ Billing Service   (Webhooks: subscription updates)
Client              ──gRPC──→ Billing Service   (CreateSubscription, CancelSubscription,
                                                  GetSubscription, CreateCheckoutSession,
                                                  GetCheckoutSession)
```

## Tech Stack

| Category | Technology |
|---|---|
| Language | Go 1.25+ |
| Database | PostgreSQL (pgx/v5 driver), `billing` schema |
| Query Layer | SQLC — type-safe generated Go from SQL |
| Migrations | golang-migrate |
| gRPC | google.golang.org/grpc (port 50051) |
| Protobuf | google.golang.org/protobuf |
| Kafka | segmentio/kafka-go (topic: `system.notifications`) |
| Payments | stripe/stripe-go/v85 |
| Config | spf13/viper + joho/godotenv |
| Logging | rs/zerolog + lumberjack (JSON structured, rotation) |
| JSON | bytedance/sonic |
| UUID | google/uuid |
| Container | Docker (multi-stage Alpine build) |
| Orchestration | Kubernetes + Helm |
| CI/CD | GitHub Actions + AWS OIDC + ECR |
| GitOps | Argo CD |
| Local Dev | Skaffold |

## API Reference

### gRPC Service (`billing.v1.BillingService`) — Port `50051`

#### Check & Record Operations

| RPC | Request | Response | Description |
|---|---|---|---|
| `CheckLimit` | `workspace_id, environment_id, channel` | `allowed, reason, limit, current, reset_at` | Validates if a workspace can send on a channel. Missing usage treated as zero (new workspaces always allowed). Plan limits: `-1` = unlimited. Returns error `PermissionDenied` if no active subscription, `ResourceExhausted` if limit reached. |
| `RecordUsage` | `workspace_id, environment_id, channel_config_id, channel, provider, success` | `acknowledged` | Atomically increments channel usage + provider usage counters in a single transaction. Evaluates 80%/100% thresholds and publishes Kafka alerts. |
| `GetUsage` | `workspace_id, environment_id` | `usage[], period_start, period_end, subscription_status` | Returns per-channel usage summary for the current billing period. |

#### Subscription Lifecycle

| RPC | Request | Response | Description |
|---|---|---|---|
| `CreateSubscription` | `workspace_id, plan_id, payment_provider, external_subscription_id, external_customer_id` | `subscription_id, success` | Cancels any existing active subscription, then creates a new one. `plan_id` accepts both UUID and plan name (resolved to ID). Defaults `payment_provider` to `"system"` if empty. |
| `CancelSubscription` | `workspace_id, subscription_id` | `success` | Marks a subscription as cancelled. Verifies workspace ownership and checks not already cancelled. |
| `GetSubscription` | `workspace_id` | `subscription_id, plan_name, status, current_period_end, payment_provider` | Returns the current active subscription with plan name for a workspace. |

#### Stripe Checkout

| RPC | Request | Response | Description |
|---|---|---|---|
| `CreateCheckoutSession` | `workspace_id, plan_id, customer_email` | `checkout_url` | Creates a Stripe Checkout Session using the plan's `external_price_id`. Returns the redirect URL. |
| `GetCheckoutSession` | `session_id` | `id, customer_email, amount_total, currency, payment_status, plan_name, subscription_id` | Retrieves checkout session details from Stripe. |

### HTTP Endpoints — Port `8081`

| Route | Method | Description |
|---|---|---|
| `/webhooks/stripe` | POST | Stripe webhook handler — processes `checkout.session.completed`, `customer.subscription.updated`, `customer.subscription.deleted`, `invoice.payment_succeeded`, `invoice.payment_failed` |
| `/health` | GET | Health check — returns `200 OK` |

### gRPC Error Mapping

| Domain Error | gRPC Code | Condition |
|---|---|---|
| `ErrNoActiveSubscription` | `PermissionDenied` | No active or renewable subscription |
| `ErrLimitReached` | `ResourceExhausted` | Channel usage at or above plan limit |
| `ErrNotFound` / `ErrPlanNotFound` | `NotFound` | Resource doesn't exist |
| `ErrAlreadyCancelled` | `FailedPrecondition` | Subscription already cancelled |
| Others | `Internal` | Unexpected server error |

## Kafka Events

All events are published to topic `system.notifications`.

| Event Type | Trigger | `environment_id` |
|---|---|---|
| `subscription_expiry_reminder` | Subscription expiring within 3 days (cron) | `FallBackUUID` (`00000000-0000-0000-0000-000000000000`) — Notification Engine resolves Production env |
| `subscription_limit_reached_80` | Channel usage crosses 80% of plan limit | Real environment ID |
| `subscription_limit_reached_100` | Channel usage crosses 100% of plan limit | Real environment ID |

Kafka publishing is **best-effort** — failures are logged but never block the primary operation.

## Database Schema

### Schema: `billing`

#### `billing.subscriptions`
| Column | Type | Description |
|---|---|---|
| `id` | UUID PK | `gen_random_uuid()` |
| `workspace_id` | UUID FK → `public.workspaces(id)` | |
| `plan_id` | UUID FK → `public.plans(id)` | |
| `payment_provider` | varchar(255) | `stripe`, `system`, etc. |
| `external_subscription_id` | varchar(255) | Stripe subscription ID (unique) |
| `external_customer_id` | varchar(255) | Stripe customer ID |
| `status` | varchar(50) | `active`, `cancelled`, `past_due`, `trialing` |
| `provider_metadata` | JSONB | Flexible provider-specific data |
| `current_period_start` | TIMESTAMPTZ | Billing period start |
| `current_period_end` | TIMESTAMPTZ | Billing period end |
| `cancelled_at` | TIMESTAMPTZ | Nullable |
| `expiry_3d_sent` | boolean | Dedup flag for expiry alerts |
| `created_at` / `updated_at` | TIMESTAMPTZ | Auto-managed via trigger |
- **Indexes**: Unique on `external_subscription_id`, partial unique `WHERE status = 'active'` on `workspace_id` (enforces one active per workspace), index on `workspace_id`.

#### `billing.usage`
| Column | Type | Description |
|---|---|---|
| `id` | UUID PK | |
| `workspace_id` | UUID FK → `public.workspaces(id)` | |
| `environment_id` | UUID FK → `public.environments(id)` | |
| `channel_name` | varchar(50) | `email`, `sms`, `push`, `slack`, `whatsapp`, `webhook`, `in_app` |
| `current_usage` | BIGINT | Incremented atomically via upsert |
| `reset_at` | TIMESTAMPTZ | Billing bucket boundary |
| `limit_80_sent` / `limit_100_sent` | boolean | Dedup flags for threshold alerts |
| `updated_at` | TIMESTAMPTZ | Auto-managed via trigger |
- **Unique**: `(workspace_id, environment_id, channel_name, reset_at)` — enables the bucket pattern.

#### `billing.provider_usage`
| Column | Type | Description |
|---|---|---|
| `id` | UUID PK | |
| `workspace_id` | UUID FK → `public.workspaces(id)` | |
| `environment_id` | UUID FK → `public.environments(id)` | |
| `channel_config_id` | UUID FK → `public.channel_configs(id)` | |
| `provider_name` | varchar(100) | e.g. `sendgrid`, `twilio` |
| `channel_name` | varchar(50) | |
| `success_count` | BIGINT | Incremented via upsert |
| `failure_count` | BIGINT | Incremented via upsert |
| `reset_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | Auto-managed via trigger |
- **Unique**: `(workspace_id, environment_id, provider_name, channel_name, reset_at)`.

### External Tables (referenced but not owned)
- `public.workspaces(id, name, plan_id)` — workspace registry
- `public.plans(id, name, [channel]_limit_month, is_active, external_price_id)` — plan definitions with per-channel limits
- `public.environments(id)` — environment registry (Dev, Stage, Prod)
- `public.channel_configs(id)` — provider channel configuration

## Key Design Patterns

### Bucket Usage Pattern (No Cron Jobs)
Usage records are keyed by `reset_at` (the subscription's `current_period_end`). `INSERT ... ON CONFLICT DO UPDATE` handles rollover: a new period with a new `reset_at` automatically creates a fresh usage row (new bucket) while preserving historical data. No nightly reset jobs needed.

### Date is the Boss, Not Status
Access is enforced by `current_period_end < NOW()` at the query level. A cancelled user can still send until their paid time expires. Status indicates billing intent, but the clock is the real gatekeeper.

### Lazy Free Subscription Renewal
When a `system`-provider subscription has expired (`current_period_end < NOW()`), the `getOrRenewSubscription` function automatically extends it by 30 days on the next `CheckLimit` or `RecordUsage` call.

### Dual Usage Tracking
- `billing.usage`: Customer billing — how many emails were sold vs consumed.
- `billing.provider_usage`: Infrastructure monitoring — how many emails a specific provider (e.g. SendGrid) actually delivered vs failed.

### Fail-Safe Alerting
Threshold evaluations (80%/100%) happen after every `RecordUsage` call. Kafka publishing and dedup flag updates are best-effort — logged on failure, never blocking the increment.

## Project Structure

```
├── .github/workflows/deploy.yml   CI/CD pipeline (ECR push, Helm tag update)
├── cmd/                           Entry points (main.go, cmd.go)
│   ├── main.go                    Bootstrap
│   └── cmd.go                     Server setup, graceful shutdown
├── config/
│   ├── config.go                  Viper/godotenv config struct + loader
│   └── config.yaml                Default configuration
├── db/
│   ├── migrations/                6 sequential SQL migrations (up + down)
│   ├── query/                     SQLC query definitions (.sql)
│   ├── schema/external_schema.sql External table DDL (workspaces, plans, envs)
│   └── sqlc/                      SQLC generated Go code
├── deployments/helm/billing-service/ Helm chart (templates, values, values-prod)
├── internal/
│   ├── app/app.go                 Dependency injection container
│   ├── cron/schdeuler.go          Background expiry scheduler
│   ├── domain/                    Pure Go models & interfaces
│   ├── handlers/                  gRPC + HTTP webhook handlers
│   ├── producer/                  Kafka producer
│   ├── repositories/              SQLC → domain translation layer
│   ├── services/                  Business logic layer
│   └── stripe/                    Stripe provider implementation
├── pkg/
│   ├── apperrors/                 Sentinel errors + DB error mapper
│   ├── constants/                 Kafka topics, event types, statuses, reasons
│   ├── helpers/                   UUID/pgtype/time conversion utilities
│   └── logger/                    zerolog + lumberjack setup
├── proto/                         Protobuf definitions + generated code
├── Taskfile.yml                   Automation (build, run, migrate, gen-proto, etc.)
├── Dockerfile                     Multi-stage Alpine build
└── skaffold.yaml                  Local K8s development
```

## Getting Started

### Prerequisites
- Docker & Docker Compose
- Go 1.25+
- `migrate` CLI (golang-migrate)
- `sqlc` CLI
- `grpcurl` (for manual testing)
- `protoc` + `protoc-gen-go` (for proto generation)

### 1. Start Infrastructure
```bash
docker compose up -d
```

### 2. Run Migrations
```bash
task migrate-up
```

### 3. Generate SQLC
```bash
task gen-sql
```

### 4. Run the Service
```bash
task run
```

### 5. Manual Testing via grpcurl (Windows PowerShell)
```powershell
# Create a system subscription
$body = '{"workspace_id":"YOUR_WORKSPACE_ID","plan_id":"free","payment_provider":"system"}'
Set-Content -Path req.json -Value $body
grpcurl -plaintext -d @req.json localhost:50051 billing.v1.BillingService/CreateSubscription

# Check sending limit
$body = '{"workspace_id":"YOUR_WORKSPACE_ID","environment_id":"YOUR_ENV_ID","channel":"email"}'
Set-Content -Path req.json -Value $body
grpcurl -plaintext -d @req.json localhost:50051 billing.v1.BillingService/CheckLimit

# Record usage
$body = '{"workspace_id":"YOUR_WORKSPACE_ID","environment_id":"YOUR_ENV_ID","channel_config_id":"YOUR_CONFIG_ID","channel":"email","provider":"sendgrid","success":true}'
Set-Content -Path req.json -Value $body
grpcurl -plaintext -d @req.json localhost:50051 billing.v1.BillingService/RecordUsage

# Get usage summary
$body = '{"workspace_id":"YOUR_WORKSPACE_ID","environment_id":"YOUR_ENV_ID"}'
Set-Content -Path req.json -Value $body
grpcurl -plaintext -d @req.json localhost:50051 billing.v1.BillingService/GetUsage
```

## Configuration

The service uses `config.yaml` with `.env` file overrides. Reference:

```yaml
app:
  name: "billing-service"
  port: "50051"          # gRPC port
  http_port: "8081"       # HTTP (webhook + health) port
  environment: "production"

database:
  max_open_conns: 5
  min_open_conns: 1
  max_idle_conns: 2
  max_conn_lifetime: "5m"
  max_idle_time: "1m"

log:
  level: "info"
  file: "logs/billing-service.log"
  max_size_mb: 50
  max_backups: 3
  max_age_days: 28

kafka:
  broker_address: "deployments-kafka-1:9092"
  batch_size: 100
  batch_timeout_ms: 10
```

Environment variables: `DB_USER`, `DB_PASSWORD`, `DB_HOST`, `DB_PORT`, `DB_NAME`, `STRIPE_API_KEY`, `STRIPE_WEBHOOK_SECRET`, `STRIPE_SUCCESS_URL`, `STRIPE_CANCEL_URL`, `KAFKA_BROKER_ADDRESS`, `APP_ENV`.

## Deployment & GitOps

### CI/CD Pipeline (`.github/workflows/deploy.yml`)
Triggered on pushes to `main` (excluding `values-prod.yaml` changes):

1. **Test**: `go test -v ./...` on Go 1.26
2. **Build & Deploy** (keyless OIDC to AWS):
   - Assume IAM role via GitHub OIDC provider
   - Build & push Docker image to ECR (`711396988882.dkr.ecr.ap-south-1.amazonaws.com/billing-service`)
   - Update image tag in `deployments/helm/billing-service/values-prod.yaml`
   - Commit & push with `[skip ci]`

### Argo CD GitOps
- Argo CD watches the Git repository
- Helm chart at `deployments/helm/billing-service/`
- Auto-syncs, prunes, and self-heals cluster state
- Two value sets: `values.yaml` (dev) and `values-prod.yaml` (prod)

### Config & Secret Management
- **External Secrets Operator**: Syncs secrets from AWS Secrets Manager into K8s
- **Stakater Reloader**: Annotated with `reloader.stakater.com/auto: "true"` — automatically restarts pods when secrets/configmaps change
- **Supavisor (Session Mode)**: Database connection proxying with 60s queuing

## Available Task Commands

| Command | Description |
|---|---|
| `task build` | Build Go binary to `bin/billing.exe` |
| `task run` | Migrate up + build + run |
| `task migrate-up` | Apply pending migrations |
| `task migrate-down` | Roll back migrations |
| `task migrate-create [name]` | Create new migration pair |
| `task gen-sql` | Regenerate SQLC code |
| `task gen-proto` | Regenerate protobuf Go code |
| `task generate` | Run all code generation |
| `task tidy` | `go mod tidy` |
| `task docker-build` | Build Docker image |
| `task docker-run` | Build & run Docker container |
| `task k8s-dev` | Skaffold dev for local K8s |
