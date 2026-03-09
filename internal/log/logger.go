package log //nolint:revive // intentional package name

import (
	"os"
	"strings"

	"go.uber.org/zap"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/log"
)

var logger = newLogger()

func Named(name string) log.Logger {
	return logger.Named(name)
}

func newLogger() *zapLogger {
	level, levelString := zap.InfoLevel, strings.ToLower(os.Getenv(config.EnvLogLevel))

	switch levelString {
	case "debug":
		level = zap.DebugLevel
	case "info":
		level = zap.InfoLevel
	case "warn":
		level = zap.WarnLevel
	case "error":
		level = zap.ErrorLevel
	}

	return &zapLogger{
		logger: newZapLogger(level).WithOptions(zap.AddCallerSkip(1)),
	}
}
