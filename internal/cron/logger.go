package cron

import (
	"fmt"
	"strings"
)

// cronLogger implements gocron.Logger interface to integrate with the framework's logging system.
type cronLogger struct{}

func (*cronLogger) Debug(msg string, args ...any) {
	logger.Debug(msg + formatKV(args...))
}

func (*cronLogger) Error(msg string, args ...any) {
	logger.Error(msg + formatKV(args...))
}

func (*cronLogger) Info(msg string, args ...any) {
	logger.Info(msg + formatKV(args...))
}

func (*cronLogger) Warn(msg string, args ...any) {
	logger.Warn(msg + formatKV(args...))
}

func newCronLogger() *cronLogger {
	return &cronLogger{}
}

// formatKV renders gocron's structured key/value arguments into the message
// suffix. gocron always calls Logger.<Level>(msg, key1, val1, ...) with msg as a
// plain string, so the framework's non-formatting Debug/Info/Warn/Error methods
// must be used: forwarding msg to the printf-style Debugf/Infof would treat it as
// a format template and corrupt any '%' it contains. The pairing/odd-count
// behavior mirrors gocron's own logFormatArgs so the output stays consistent
// with gocron's default logger.
func formatKV(args ...any) string {
	if len(args) == 0 {
		return ""
	}

	if len(args)%2 != 0 {
		return ", " + fmt.Sprint(args...)
	}

	pairs := make([]string, 0, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		pairs = append(pairs, fmt.Sprintf("%s=%v", args[i], args[i+1]))
	}

	return ", " + strings.Join(pairs, ", ")
}
