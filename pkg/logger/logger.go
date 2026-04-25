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

func NewLogger(cfg *config.LogConfig) zerolog.Logger {
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

	level, _ := zerolog.ParseLevel(cfg.Level)
	zerolog.SetGlobalLevel(level)

	return zerolog.New(io.MultiWriter(consoleWriter, fileWriter)).
		With().
		Timestamp().
		Caller().
		Logger()
}
