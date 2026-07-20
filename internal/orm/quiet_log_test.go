package orm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/logx"
)

// levelRecordingLogger captures the level of every non-formatting log call so
// hook tests can assert routing without a real logging backend.
type levelRecordingLogger struct {
	levels []logx.Level
}

func (l *levelRecordingLogger) record(level logx.Level) { l.levels = append(l.levels, level) }

func (l *levelRecordingLogger) Named(string) logx.Logger       { return l }
func (l *levelRecordingLogger) WithCallerSkip(int) logx.Logger { return l }
func (*levelRecordingLogger) Enabled(logx.Level) bool          { return true }
func (*levelRecordingLogger) Sync()                            {}
func (l *levelRecordingLogger) Debug(string)                   { l.record(logx.LevelDebug) }
func (*levelRecordingLogger) Debugf(string, ...any)            {}
func (l *levelRecordingLogger) Info(string)                    { l.record(logx.LevelInfo) }
func (*levelRecordingLogger) Infof(string, ...any)             {}
func (l *levelRecordingLogger) Warn(string)                    { l.record(logx.LevelWarn) }
func (*levelRecordingLogger) Warnf(string, ...any)             {}
func (l *levelRecordingLogger) Error(string)                   { l.record(logx.LevelError) }
func (*levelRecordingLogger) Errorf(string, ...any)            {}
func (l *levelRecordingLogger) Panic(string)                   { l.record(logx.LevelPanic) }
func (*levelRecordingLogger) Panicf(string, ...any)            {}

func TestWithQuietSQLLog(t *testing.T) {
	ctx := context.Background()
	assert.False(t, IsQuietSQLLog(ctx), "An unmarked context must not read as quiet")

	quiet := WithQuietSQLLog(ctx)
	assert.True(t, IsQuietSQLLog(quiet), "A marked context must read as quiet")
	assert.False(t, IsQuietSQLLog(ctx), "Marking must not mutate the original context")

	type nestedKey struct{}

	nested := context.WithValue(quiet, nestedKey{}, "value")
	assert.True(t, IsQuietSQLLog(nested), "The mark must survive further context derivation")
}

func TestQueryHookLogLevelRouting(t *testing.T) {
	newEvent := func(elapsed time.Duration, err error) *bun.QueryEvent {
		return &bun.QueryEvent{
			Query:     "SELECT 1",
			StartTime: time.Now().Add(-elapsed),
			Err:       err,
		}
	}

	tests := []struct {
		name      string
		ctx       context.Context //nolint:containedctx // table-driven test input
		event     *bun.QueryEvent
		wantLevel logx.Level
	}{
		{
			name:      "RegularQueryLogsAtInfo",
			ctx:       context.Background(),
			event:     newEvent(0, nil),
			wantLevel: logx.LevelInfo,
		},
		{
			name:      "QuietContextDemotesToDebug",
			ctx:       WithQuietSQLLog(context.Background()),
			event:     newEvent(0, nil),
			wantLevel: logx.LevelDebug,
		},
		{
			name:      "SlowQueryOutranksTheQuietMark",
			ctx:       WithQuietSQLLog(context.Background()),
			event:     newEvent(time.Second, nil),
			wantLevel: logx.LevelWarn,
		},
		{
			name:      "FailureOutranksTheQuietMark",
			ctx:       WithQuietSQLLog(context.Background()),
			event:     newEvent(0, errors.New("connection refused")),
			wantLevel: logx.LevelError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := new(levelRecordingLogger)
			hook := &queryHook{logger: logger, output: termenv.DefaultOutput()}

			hook.AfterQuery(tt.ctx, tt.event)

			assert.Equal(t, []logx.Level{tt.wantLevel}, logger.levels,
				"The statement must log exactly once at the routed level")
		})
	}
}
