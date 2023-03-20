package logger

import (
	"log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	logLevel = "LOG_LEVEL"
)

func InitLogger() *zap.Logger {
	var loggingLevel zapcore.Level

	switch os.Getenv(logLevel) {
	case "DEBUG":
		loggingLevel = zap.DebugLevel
	case "INFO":
		loggingLevel = zap.InfoLevel
	default:
		loggingLevel = zap.ErrorLevel
	}

	conf := zap.Config{
		Level:            zap.NewAtomicLevelAt(loggingLevel),
		Development:      false,
		Encoding:         "console",
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig: zapcore.EncoderConfig{
			LevelKey:       "level",
			TimeKey:        "time",
			MessageKey:     "msg",
			CallerKey:      "caller",
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeLevel:    zapcore.CapitalColorLevelEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
	}

	logger, err := conf.Build()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	return logger
}
