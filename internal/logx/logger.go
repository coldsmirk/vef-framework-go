package logx

import (
	"os"
	"strings"

	"go.uber.org/zap"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/logx"
)

// rootLevel governs the verbosity of every logger derived from rootLogger.
// It is a zap.AtomicLevel, so SetLevel can adjust the active threshold at
// runtime (and lets tests restore a known level) without rebuilding loggers.
var rootLevel = zap.NewAtomicLevelAt(toZapLevel(parseLevel(os.Getenv(config.EnvLogLevel))))

var rootLogger = &zapLogger{
	logger: newZapLogger(rootLevel).WithOptions(zap.AddCallerSkip(1)),
}

func Named(name string) logx.Logger {
	return rootLogger.Named(name)
}

// SetLevel updates the threshold of the root logger and every logger derived
// from it. It exists so the verbosity captured from the environment at start
// up can be overridden at runtime and reset between tests.
func SetLevel(level logx.Level) {
	rootLevel.SetLevel(toZapLevel(level))
}

func parseLevel(levelString string) logx.Level {
	switch strings.ToLower(levelString) {
	case "debug":
		return logx.LevelDebug
	case "warn":
		return logx.LevelWarn
	case "error":
		return logx.LevelError
	case "panic":
		return logx.LevelPanic
	default:
		return logx.LevelInfo
	}
}
