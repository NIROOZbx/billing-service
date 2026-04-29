package app

import (
	"fmt"
	"time"

	"github.com/NIROOZbx/billing-service/config"
	"github.com/NIROOZbx/billing-service/db"
	"github.com/NIROOZbx/billing-service/db/sqlc"
	"github.com/NIROOZbx/billing-service/internal/cron"
	"github.com/NIROOZbx/billing-service/internal/handlers"
	"github.com/NIROOZbx/billing-service/internal/producer"
	"github.com/NIROOZbx/billing-service/internal/repositories"
	"github.com/NIROOZbx/billing-service/internal/services"
	"github.com/NIROOZbx/billing-service/internal/stripe"
	"github.com/NIROOZbx/billing-service/pkg/logger"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	stripego "github.com/stripe/stripe-go/v85"
)

type App struct {
	Config         *config.Config
	DB             *pgxpool.Pool
	Logger         zerolog.Logger
	GRPCHandler    *handlers.BillingServer
	WebHookHandler *handlers.WebhookHandler
	Scheduler      *cron.Scheduler
	Producer       producer.Producer
}

func StartApp(cfg *config.Config) (*App, error) {
	syslog := logger.NewLogger(&cfg.Log)
	syslog.Info().Msg("Starting billing service...")

	pool, err := db.ConnectDB(cfg.Database)
	if err != nil {
		syslog.Fatal().Err(err).Msg("failed to connect to DB")
		return nil, fmt.Errorf("failed to connect to DB: %w", err)
	}

	queries := sqlc.New(pool)
	kafkaProducer := producer.NewKafkaProducer(cfg.Kafka)
	// ==========================================
	// PROVIDER SETUP
	// Initializes the Stripe client with the secret
	// ==========================================
	stripego.Key = cfg.Stripe.ApiKey
	stripeProvider := stripe.NewStripeProvider(cfg.Stripe.WebhookSecret, syslog)

	// ==========================================
	// HANDLER WIRING
	// Injects the provider and services into the handler
	// ==========================================

	usageRepo := repositories.NewUsageRepository(queries)
	subscriptionRepo := repositories.NewSubscriptionRepository(queries)
	planRepo := repositories.NewPlanRepository(queries)

	usageSvc := services.NewUsageService(usageRepo, subscriptionRepo, planRepo, kafkaProducer, syslog)
	subscriptionSvc := services.NewSubscriptionService(subscriptionRepo, planRepo, stripeProvider, cfg)
	planSvc := services.NewPlanService(planRepo)

	grpcHandler := handlers.NewBillingServer(usageSvc, subscriptionSvc, planSvc, syslog)
	webhookHandler := handlers.NewWebHookHandler(stripeProvider, subscriptionSvc, syslog)

	scheduler := cron.NewScheduler(subscriptionRepo, syslog, 1*time.Hour, kafkaProducer)

	return &App{
		Config:         cfg,
		DB:             pool,
		Logger:         syslog,
		GRPCHandler:    grpcHandler,
		WebHookHandler: webhookHandler,
		Scheduler:      scheduler,
		Producer:       kafkaProducer,
	}, nil
}

func (a *App) Close() {
	if a.DB != nil {
		a.DB.Close()
	}
}
