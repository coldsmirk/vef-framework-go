package jsconsole_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/js"
	"github.com/coldsmirk/vef-framework-go/js/jsconsole"
	"github.com/coldsmirk/vef-framework-go/logx"
)

// LogEntry is one captured log call.
type LogEntry struct {
	level   string
	message string
}

// RecordingLogger captures Info/Warn/Error calls for console tests.
type RecordingLogger struct {
	entries []LogEntry
}

func (l *RecordingLogger) Named(string) logx.Logger {
	return l
}

func (l *RecordingLogger) WithCallerSkip(int) logx.Logger {
	return l
}

func (*RecordingLogger) Enabled(logx.Level) bool {
	return true
}

func (*RecordingLogger) Sync() {}

func (*RecordingLogger) Debug(string) {}

func (*RecordingLogger) Debugf(string, ...any) {}

func (l *RecordingLogger) Info(message string) {
	l.entries = append(l.entries, LogEntry{level: "info", message: message})
}

func (*RecordingLogger) Infof(string, ...any) {}

func (l *RecordingLogger) Warn(message string) {
	l.entries = append(l.entries, LogEntry{level: "warn", message: message})
}

func (*RecordingLogger) Warnf(string, ...any) {}

func (l *RecordingLogger) Error(message string) {
	l.entries = append(l.entries, LogEntry{level: "error", message: message})
}

func (*RecordingLogger) Errorf(string, ...any) {}

func (*RecordingLogger) Panic(string) {}

func (*RecordingLogger) Panicf(string, ...any) {}

// newConsoleRuntime builds a bare runtime with the console library enabled.
func newConsoleRuntime(t *testing.T) (*js.Runtime, *RecordingLogger) {
	t.Helper()

	logger := new(RecordingLogger)

	engine, err := js.NewEngine(js.WithoutStdLibs(), js.WithLibs(jsconsole.New(logger)))
	require.NoError(t, err, "NewEngine should succeed")

	rt, err := engine.NewRuntime(js.EnableLibs(jsconsole.Name))
	require.NoError(t, err, "NewRuntime should succeed")

	return rt, logger
}

// TestConsole tests level mapping and argument formatting.
func TestConsole(t *testing.T) {
	t.Run("InfoFormatsArguments", func(t *testing.T) {
		rt, logger := newConsoleRuntime(t)

		_, err := rt.RunString(t.Context(), `console.info('processed', 42, { a: 1, b: ['x'] })`)
		require.NoError(t, err, "Script should execute successfully")

		require.Len(t, logger.entries, 1, "One log entry should be recorded")
		assert.Equal(t, "info", logger.entries[0].level, "console.info should log at info level")
		assert.Equal(t, `processed 42 {"a":1,"b":["x"]}`, logger.entries[0].message, "Arguments should be space-joined with objects as JSON")
	})

	t.Run("WarnLevel", func(t *testing.T) {
		rt, logger := newConsoleRuntime(t)

		_, err := rt.RunString(t.Context(), `console.warn('careful')`)
		require.NoError(t, err, "Script should execute successfully")

		require.Len(t, logger.entries, 1, "One log entry should be recorded")
		assert.Equal(t, "warn", logger.entries[0].level, "console.warn should log at warn level")
		assert.Equal(t, "careful", logger.entries[0].message, "String arguments should pass through verbatim")
	})

	t.Run("ErrorLevel", func(t *testing.T) {
		rt, logger := newConsoleRuntime(t)

		_, err := rt.RunString(t.Context(), `console.error('failed', { code: 500 })`)
		require.NoError(t, err, "Script should execute successfully")

		require.Len(t, logger.entries, 1, "One log entry should be recorded")
		assert.Equal(t, "error", logger.entries[0].level, "console.error should log at error level")
		assert.Equal(t, `failed {"code":500}`, logger.entries[0].message, "Arguments should be space-joined with objects as JSON")
	})

	t.Run("NoArguments", func(t *testing.T) {
		rt, logger := newConsoleRuntime(t)

		_, err := rt.RunString(t.Context(), `console.info()`)
		require.NoError(t, err, "Script should execute successfully")

		require.Len(t, logger.entries, 1, "One log entry should be recorded")
		assert.Empty(t, logger.entries[0].message, "No arguments should log an empty message")
	})
}
