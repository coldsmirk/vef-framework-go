package logx

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/logx"
)

type loggedEntry struct {
	level   logx.Level
	name    string
	message string
}

// recordingLogger is a logx.Logger that captures every emitted message so the
// slog handler's output can be asserted. Named children share the parent's
// state slice so messages routed through WithGroup are still observed.
type recordingLogger struct {
	state   *[]loggedEntry
	name    string
	enabled logx.Level
}

func newRecordingLogger(enabled logx.Level) *recordingLogger {
	return &recordingLogger{state: new([]loggedEntry), enabled: enabled}
}

func (l *recordingLogger) record(level logx.Level, message string) {
	*l.state = append(*l.state, loggedEntry{level: level, name: l.name, message: message})
}

func (l *recordingLogger) Named(name string) logx.Logger {
	child := name
	if l.name != "" {
		child = l.name + "." + name
	}

	return &recordingLogger{state: l.state, name: child, enabled: l.enabled}
}

func (l *recordingLogger) WithCallerSkip(int) logx.Logger { return l }
func (l *recordingLogger) Enabled(level logx.Level) bool  { return level >= l.enabled }
func (*recordingLogger) Sync()                            {}
func (l *recordingLogger) Debug(message string)           { l.record(logx.LevelDebug, message) }
func (*recordingLogger) Debugf(string, ...any)            {}
func (l *recordingLogger) Info(message string)            { l.record(logx.LevelInfo, message) }
func (*recordingLogger) Infof(string, ...any)             {}
func (l *recordingLogger) Warn(message string)            { l.record(logx.LevelWarn, message) }
func (*recordingLogger) Warnf(string, ...any)             {}
func (l *recordingLogger) Error(message string)           { l.record(logx.LevelError, message) }
func (*recordingLogger) Errorf(string, ...any)            {}
func (l *recordingLogger) Panic(message string)           { l.record(logx.LevelPanic, message) }
func (*recordingLogger) Panicf(string, ...any)            {}

type stringValuer string

func (s stringValuer) LogValue() slog.Value { return slog.StringValue("resolved:" + string(s)) }

func TestSlogLevelToLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		level slog.Level
		want  logx.Level
	}{
		{"Debug", slog.LevelDebug, logx.LevelDebug},
		{"BelowDebug", slog.LevelDebug - 4, logx.LevelDebug},
		{"Info", slog.LevelInfo, logx.LevelInfo},
		{"BetweenInfoAndWarn", slog.LevelInfo + 1, logx.LevelInfo},
		{"Warn", slog.LevelWarn, logx.LevelWarn},
		{"Error", slog.LevelError, logx.LevelError},
		{"AboveError", slog.LevelError + 4, logx.LevelError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, slogLevelToLogLevel(tt.level), "slog level %d should map to the matching logx level", tt.level)
		})
	}
}

func TestSlogHandlerHandle(t *testing.T) {
	t.Run("EmitsRecordAttrs", func(t *testing.T) {
		logger := newRecordingLogger(logx.LevelDebug)
		slog.New(&slogHandler{logger: logger}).Info("hello", slog.String("key", "value"))

		require.Len(t, *logger.state, 1, "handler should emit exactly one entry")
		assert.Equal(t, logx.LevelInfo, (*logger.state)[0].level, "Info record should dispatch to the Info level")
		assert.Equal(t, "hello | key: value", (*logger.state)[0].message, "record attrs should be appended to the message")
	})

	t.Run("EmitsWithAttrs", func(t *testing.T) {
		logger := newRecordingLogger(logx.LevelDebug)
		log := slog.New(&slogHandler{logger: logger}).With(slog.String("request_id", "abc"))
		log.Info("hello", slog.Int("count", 2))

		require.Len(t, *logger.state, 1, "handler should emit exactly one entry")
		assert.Equal(t, "hello | request_id: abc | count: 2", (*logger.state)[0].message, "attrs from With should precede per-record attrs")
	})

	t.Run("WithAttrsDoesNotAlias", func(t *testing.T) {
		logger := newRecordingLogger(logx.LevelDebug)
		// A backing array with spare capacity is what makes the slog clone
		// pitfall observable: two children appended into shared slack would
		// overwrite each other without a defensive copy.
		attrs := make([]slog.Attr, 1, 4)
		attrs[0] = slog.String("a", "1")
		base := (&slogHandler{logger: logger, attrs: attrs})

		left := base.WithAttrs([]slog.Attr{slog.String("b", "2")})
		right := base.WithAttrs([]slog.Attr{slog.String("c", "3")})

		require.NoError(t, left.Handle(context.Background(), newInfoRecord("L")), "left handler should accept the record")
		require.NoError(t, right.Handle(context.Background(), newInfoRecord("R")), "right handler should accept the record")

		require.Len(t, *logger.state, 2, "both derived handlers should emit one entry each")
		assert.Equal(t, "L | a: 1 | b: 2", (*logger.state)[0].message, "left handler must not see the right handler's attr")
		assert.Equal(t, "R | a: 1 | c: 3", (*logger.state)[1].message, "right handler must not see the left handler's attr")
	})

	t.Run("WithGroupNamesLogger", func(t *testing.T) {
		logger := newRecordingLogger(logx.LevelDebug)
		log := slog.New((&slogHandler{logger: logger}).WithGroup("sub"))
		log.Info("hello", slog.String("k", "v"))

		require.Len(t, *logger.state, 1, "handler should emit exactly one entry")
		assert.Equal(t, "sub", (*logger.state)[0].name, "WithGroup should derive a named child logger from the group name")
		assert.Equal(t, "hello | k: v", (*logger.state)[0].message, "WithGroup keeps attrs intact while namespacing via the logger")
	})

	t.Run("IntermediateLevelStillEmits", func(t *testing.T) {
		logger := newRecordingLogger(logx.LevelDebug)
		handler := &slogHandler{logger: logger}
		record := slog.NewRecord(time.Now(), slog.LevelWarn+1, "between", 0)

		require.True(t, handler.Enabled(context.Background(), slog.LevelWarn+1), "Enabled should admit an intermediate level")
		require.NoError(t, handler.Handle(context.Background(), record), "Handle should accept the intermediate record")
		require.Len(t, *logger.state, 1, "an intermediate level admitted by Enabled must still be emitted")
		assert.Equal(t, logx.LevelWarn, (*logger.state)[0].level, "an intermediate level above Warn should route to Warn")
	})

	t.Run("FilterBelowLevelFilter", func(t *testing.T) {
		handler := &slogHandler{logger: newRecordingLogger(logx.LevelDebug), levelFilter: logx.LevelError}

		assert.False(t, handler.Enabled(context.Background(), slog.LevelInfo), "levelFilter should suppress records below the threshold")
		assert.True(t, handler.Enabled(context.Background(), slog.LevelError), "levelFilter should admit records at or above the threshold")
	})
}

func newInfoRecord(message string) slog.Record {
	return slog.NewRecord(time.Now(), slog.LevelInfo, message, 0)
}

func TestAppendAttrValues(t *testing.T) {
	tests := []struct {
		name  string
		value slog.Value
		want  string
	}{
		{"String", slog.StringValue("text"), "k: text"},
		{"Int64", slog.Int64Value(-7), "k: -7"},
		{"Uint64", slog.Uint64Value(7), "k: 7"},
		{"Float64", slog.Float64Value(1.5), "k: 1.5"},
		{"Bool", slog.BoolValue(true), "k: true"},
		{"Duration", slog.DurationValue(2 * time.Second), "k: 2s"},
		{"AnySlice", slog.AnyValue([]int{1, 2}), "k: [1 2]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := appendAttr(nil, slog.Attr{Key: "k", Value: tt.value})
			assert.Equal(t, []string{tt.want}, fields, "appendAttr should render the %s kind without dropping it", tt.name)
		})
	}
}

func TestAppendAttr(t *testing.T) {
	t.Run("ResolvesLogValuer", func(t *testing.T) {
		fields := appendAttr(nil, slog.Any("token", stringValuer("secret")))
		assert.Equal(t, []string{"token: resolved:secret"}, fields, "LogValuer attrs should be resolved before formatting")
	})

	t.Run("FlattensGroup", func(t *testing.T) {
		group := slog.Group("addr", slog.String("city", "NYC"), slog.Int("zip", 10001))
		fields := appendAttr(nil, group)
		assert.Equal(t, []string{"addr.city: NYC", "addr.zip: 10001"}, fields, "group attrs should be flattened with a dotted prefix")
	})

	t.Run("InlinesEmptyGroupKey", func(t *testing.T) {
		group := slog.Group("", slog.String("k", "v"))
		fields := appendAttr(nil, group)
		assert.Equal(t, []string{"k: v"}, fields, "a group with an empty key should inline its attrs without a prefix")
	})

	t.Run("DropsEmptyAttr", func(t *testing.T) {
		fields := appendAttr(nil, slog.Attr{})
		assert.Empty(t, fields, "an empty attr should be dropped")
	})

	t.Run("DropsEmptyGroup", func(t *testing.T) {
		fields := appendAttr(nil, slog.Group("empty"))
		assert.Empty(t, fields, "a group with no attrs should be dropped")
	})
}
