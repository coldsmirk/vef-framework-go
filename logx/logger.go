package logx

// Level represents a logging priority. Higher levels are more important.
type Level int8

const (
	// LevelDebug logs are typically voluminous and usually disabled in production.
	LevelDebug Level = iota + 1
	// LevelInfo is the default logging priority.
	LevelInfo
	// LevelWarn logs are more important than Info but don't need individual human review.
	LevelWarn
	// LevelError logs are high-priority. A smoothly running application shouldn't generate any.
	LevelError
	// LevelPanic logs a message, then panics.
	LevelPanic
)

// String returns the string representation of the log level.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	case LevelPanic:
		return "panic"
	default:
		return "unknown"
	}
}

// Logger defines the core logging interface for structured logging across the framework.
type Logger interface {
	// Named creates a child logger with the given namespace.
	Named(name string) Logger
	// WithCallerSkip adjusts the number of stack frames to skip when reporting caller location.
	WithCallerSkip(skip int) Logger
	// Enabled checks whether the given log level is enabled.
	Enabled(level Level) bool
	// Sync flushes any buffered log entries.
	Sync()
	// Debug logs a message at Debug level.
	Debug(message string)
	// Debugf logs a formatted message at Debug level.
	Debugf(template string, args ...any)
	// Info logs a message at Info level.
	Info(message string)
	// Infof logs a formatted message at Info level.
	Infof(template string, args ...any)
	// Warn logs a message at Warn level.
	Warn(message string)
	// Warnf logs a formatted message at Warn level.
	Warnf(template string, args ...any)
	// Error logs a message at Error level.
	Error(message string)
	// Errorf logs a formatted message at Error level.
	Errorf(template string, args ...any)
	// Panic logs a message at Panic level and then panics.
	Panic(message string)
	// Panicf logs a formatted message at Panic level and then panics.
	Panicf(template string, args ...any)
}

// LoggerConfigurable defines an interface for components that can be configured with a logger.
type LoggerConfigurable[T any] interface {
	// WithLogger returns a copy of the component configured with the given logger.
	WithLogger(logger Logger) T
}
