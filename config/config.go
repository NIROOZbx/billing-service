package config

import (
	"fmt"
	"log"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Database DatabaseConfig `mapstructure:"database"`
	Stripe   StripeConfig   `mapstructure:"stripe"`
	Log      LogConfig      `mapstructure:"log"`
	Kafka KafkaConfig `mapstructure:"kafka"`
}

type AppConfig struct {
	Name        string `mapstructure:"name"`
	Port        string `mapstructure:"port"`
	HttpPort    string `mapstructure:"http_port"`
	Environment string `mapstructure:"environment"`
}

type DatabaseConfig struct {
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Host            string        `mapstructure:"host"`
	Port            string        `mapstructure:"port"`
	Name            string        `mapstructure:"name"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MinOpenConns    int           `mapstructure:"min_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	MaxConnLifetime time.Duration `mapstructure:"max_conn_lifetime"`
	MaxIdleTime     time.Duration `mapstructure:"max_idle_time"`
	URL             string
}

type StripeConfig struct {
	ApiKey        string `mapstructure:"api_key"`
	WebhookSecret string `mapstructure:"webhook_secret"`
	SuccessURL string `mapstructure:"success_url"`
	CancelURL string `mapstructure:"cancel_url"`
}

type LogConfig struct {
	Level      string `mapstructure:"level"`
	File       string `mapstructure:"file"`
	MaxSizeMB  int    `mapstructure:"max_size_mb"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAgeDays int    `mapstructure:"max_age_days"`
}
type KafkaConfig struct {
	BrokerAddress  string `mapstructure:"broker_address"`
	BatchSize      int    `mapstructure:"batch_size"`
	BatchTimeoutMS int    `mapstructure:"batch_timeout_ms"`
}

func LoadConfig() (*Config, error) {
	if err := godotenv.Load("./.env"); err != nil {
		log.Println("no .env file found — reading from environment directly")
	}

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")

	v.AutomaticEnv()

	v.BindEnv("database.user", "DB_USER")
	v.BindEnv("database.password", "DB_PASSWORD")
	v.BindEnv("database.host", "DB_HOST")
	v.BindEnv("database.port", "DB_PORT")
	v.BindEnv("database.name", "DB_NAME")
	v.BindEnv("stripe.api_key", "STRIPE_API_KEY")
	v.BindEnv("stripe.webhook_secret", "STRIPE_WEBHOOK_SECRET")
	v.BindEnv("stripe.success_url", "STRIPE_SUCCESS_URL")
	v.BindEnv("stripe.cancel_url", "STRIPE_CANCEL_URL")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	config.Database.URL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		config.Database.User,
		config.Database.Password,
		config.Database.Host,
		config.Database.Port,
		config.Database.Name,
	)

	validate(&config)

	return &config, nil
}

func validate(cfg *Config) {
	rules := []struct {
		value  string
		envVar string
	}{
		{cfg.App.Port, "APP_PORT"},
		{cfg.App.HttpPort, "APP_HTTP_PORT"},
		{cfg.Database.User, "DB_USER"},
		{cfg.Database.Password, "DB_PASSWORD"},
		{cfg.Database.Host, "DB_HOST"},
		{cfg.Database.Name, "DB_NAME"},
		{cfg.Stripe.ApiKey, "STRIPE_API_KEY"},
		{cfg.Stripe.WebhookSecret, "STRIPE_WEBHOOK_SECRET"},
	}

	for _, rule := range rules {
		if rule.value == "" {
			log.Fatalf("%s is required", rule.envVar)
		}
	}
}
