# Billing Service

A production-grade microservice for managing subscriptions, tracking resource usage, and monitoring provider performance within a multi-tenant notification ecosystem.

## 🚀 Overview

The Billing Service acts as the "Accountant" and "Gatekeeper" for the Notification Engine. It ensures that workspaces stay within their plan limits and provides detailed analytics on how third-party providers (SendGrid, Twilio, etc.) are performing.

## 🏗️ Core Architecture

- **Language**: Go 1.25+
- **Communication**: gRPC
- **Database**: PostgreSQL (with Schema-based multi-tenancy)
- **Migrations**: Golang Migrate
- **Configuration**: Viper / godotenv

### Key Services
- **`CheckLimit`**: High-performance gatekeeper that validates if a workspace has the credits/subscription to send a notification.
- **`RecordUsage`**: Atomic counter management for customer billing and infrastructure monitoring.
- **`Subscription Lifecycle`**: Management of Stripe/LemonSqueezy subscriptions.

## 📁 Project Structure

```text
├── cmd/                # Entry points (main.go, cmd.go)
├── db/                 # Database migrations and SQLC queries
├── internal/
│   ├── app/           # Application container & dependency injection
│   ├── domain/        # Core business models
│   ├── handlers/      # gRPC server implementations
│   ├── repositories/  # Database access layer (SQLC generated)
├── proto/              # Protobuf definitions
├── Taskfile.yml       # Automation tasks (build, run, migrate)
└── config.yaml        # Local configuration
```

## 🛠️ Getting Started

### 1. Prerequisites
- Docker & Docker Compose
- Go 1.25+
- `migrate` CLI (`brew install golang-migrate`)

### 2. Setup Environment
Copy the example environment file and update your credentials:
```bash
cp .env.example .env
```

### 3. Run Migrations
```bash
task migrate-up
```

### 4. Run the Service
```bash
task run
```

## 📊 Database Schema

The service uses a sophisticated schema designed for scalability:
1.  **`billing.subscriptions`**: Tracks workspace-level plan assignments and provider IDs.
2.  **`billing.usage`**: Tracks per-environment, per-channel consumption.
3.  **`billing.provider_usage`**: Tracks success/failure rates of individual channel configurations.

---
Built with ❤️ by NIROOZbx Labs.
