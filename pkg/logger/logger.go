package logger

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/NIROOZbx/billing-service/config"
	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

func NewLogger(cfg *config.LogConfig, env string) zerolog.Logger {
	if env == "production" {
		zerolog.TimestampFunc = func() time.Time {
			return time.Now().UTC()
		}
	} else {
		zerolog.TimestampFunc = func() time.Time {
			return time.Now().Local()
		}
	}

	level, _ := zerolog.ParseLevel(cfg.Level)
	zerolog.SetGlobalLevel(level)

	var logWriter io.Writer

	if env == "production" {
		logWriter = os.Stdout
	} else {
		if err := os.MkdirAll(filepath.Dir(cfg.File), 0755); err != nil {
			panic("Failed to create log directory: " + err.Error())
		}

		fileWriter := &lumberjack.Logger{
			Filename:   cfg.File,
			MaxSize:    cfg.MaxSizeMB,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAgeDays,
			Compress:   true,
		}

		consoleWriter := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.DateTime,
		}

		logWriter = io.MultiWriter(consoleWriter, fileWriter)

	}

	logger := zerolog.New(logWriter).With().Timestamp()

	if env != "production" {
		logger = logger.Caller()
	}

	return logger.Logger()
}
