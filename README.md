# Billing Service

A production-grade microservice for managing subscriptions, tracking resource usage, and proactively notifying workspaces of billing events within a multi-tenant notification ecosystem.

## 🚀 Overview

The Billing Service acts as the **"Accountant"**, **"Gatekeeper"**, and **"Alert System"** for the Notification Engine. It ensures workspaces stay within their plan limits, manages Stripe subscriptions, and autonomously publishes billing events to Kafka so the Notification Engine can alert workspace owners in real time.

## 🏗️ Core Architecture

- **Language**: Go 1.25+
- **Communication**: gRPC (internal), Kafka (event publishing)
- **Database**: PostgreSQL with `billing` schema
- **Migrations**: Golang Migrate
- **Query Layer**: SQLC (type-safe generated queries)
- **Configuration**: Viper / godotenv

## ✨ Key Features

### gRPC API
| Method | Description |
|---|---|
| `CheckLimit` | Validates if a workspace can send for a given channel. Treats missing usage as zero (new workspaces always allowed). |
| `RecordUsage` | Atomically increments channel and provider usage counters. |
| `CreateSubscription` | Creates a new subscription for a workspace. Cancels any existing active subscription first. |
| `CancelSubscription` | Marks a subscription as cancelled. |
| `GetSubscription` | Returns the current active subscription and plan for a workspace. |
| `GetUsage` | Returns usage summary per channel for a workspace/environment. |
| `CreateCheckoutSession` | Creates a Stripe Checkout Session and returns the redirect URL. |

### Background Scheduler
A background cron job polls for subscriptions expiring within 3 days and publishes a `subscription_expiry_reminder` event to Kafka. It uses a `expiry_3d_sent` flag to prevent duplicate alerts.

### Usage Limit Alerting
Integrated directly into the `RecordUsage` flow. When a channel crosses **80%** or **100%** of its plan limit, the service automatically publishes a `subscription_limit_reached_80` or `subscription_limit_reached_100` event to Kafka.

### Stripe Webhook Handler
Handles `customer.subscription.updated` to keep subscription status, period dates, and billing state synchronized with Stripe in real time.

## 📁 Project Structure

```text
├── cmd/                    # Entry points (main.go, cmd.go)
├── config/                 # Config struct and Viper loader
├── db/
│   ├── migrations/         # SQL migration files (up & down)
│   ├── query/              # SQLC query definitions
│   └── sqlc/               # SQLC generated Go code
├── internal/
│   ├── app/                # Application container & dependency wiring
│   ├── cron/               # Background expiry scheduler
│   ├── domain/             # Core business models and interfaces
│   ├── handlers/           # gRPC and Webhook HTTP handlers
│   ├── producer/           # Kafka producer (interface + implementation)
│   ├── repositories/       # Database access layer
│   ├── services/           # Business logic layer
│   └── stripe/             # Stripe billing provider implementation
├── pkg/
│   ├── apperrors/          # Sentinel error types
│   ├── constants/          # Kafka topics, event types, subscription statuses
│   └── helpers/            # UUID/pgtype conversion utilities
├── proto/                  # Protobuf definitions and generated Go code
├── Taskfile.yml            # Task automation (build, run, migrate, gen-sql)
├── config.yaml             # Local configuration
└── docker-compose.yml      # Local dev stack (Postgres, Kafka)
```

## 🛠️ Getting Started

### Prerequisites
- Docker & Docker Compose
- Go 1.25+
- `migrate` CLI
- `sqlc` CLI
- `grpcurl` (for manual testing)

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

### 5. Test via grpcurl (Windows PowerShell)
```powershell
# Create a subscription for a workspace
[System.IO.File]::WriteAllText("$PWD\req.json", '{"workspace_id":"YOUR_WORKSPACE_ID","plan_id":"YOUR_PLAN_ID","payment_provider":"system"}')
grpcurl -plaintext -d "@req.json" localhost:8081 billing.v1.BillingService/CreateSubscription

# Check sending limit
[System.IO.File]::WriteAllText("$PWD\req.json", '{"workspace_id":"YOUR_WORKSPACE_ID","environment_id":"YOUR_ENV_ID","channel":"email"}')
grpcurl -plaintext -d "@req.json" localhost:8081 billing.v1.BillingService/CheckLimit
```

## 📊 Database Schema

| Table | Purpose |
|---|---|
| `billing.subscriptions` | Workspace-level plan assignments, Stripe IDs, period dates, expiry alert flags |
| `billing.usage` | Per-environment, per-channel consumption with 80%/100% alert flags |
| `billing.provider_usage` | Success/failure rates of individual provider configurations |

## 📡 Kafka Events

All events are published to the `system.notifications` topic.

| Event Type | Trigger | `environment_id` |
|---|---|---|
| `subscription_expiry_reminder` | Subscription expiring within 3 days | `FallBackUUID` (resolved to Production) |
| `subscription_limit_reached_80` | Channel usage crosses 80% of plan limit | Real environment ID |
| `subscription_limit_reached_100` | Channel usage crosses 100% of plan limit | Real environment ID |

---
Built with ❤️ by NIROOZbx Labs.
