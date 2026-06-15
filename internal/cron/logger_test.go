package cron

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/logx"
)

// recordingLogger captures the non-formatting Debug/Info/Warn/Error output so a
// test can assert exactly what cronLogger emits. cronLogger pre-renders the
// gocron message + kv pairs and calls these methods, so the formatting *f
// variants are intentionally inert here.
type recordingLogger struct {
	messages []string
}

func (l *recordingLogger) record(message string) { l.messages = append(l.messages, message) }

func (l *recordingLogger) Named(string) logx.Logger       { return l }
func (l *recordingLogger) WithCallerSkip(int) logx.Logger { return l }
func (*recordingLogger) Enabled(logx.Level) bool          { return true }
func (*recordingLogger) Sync()                            {}
func (l *recordingLogger) Debug(message string)           { l.record(message) }
func (*recordingLogger) Debugf(string, ...any)            {}
func (l *recordingLogger) Info(message string)            { l.record(message) }
func (*recordingLogger) Infof(string, ...any)             {}
func (l *recordingLogger) Warn(message string)            { l.record(message) }
func (*recordingLogger) Warnf(string, ...any)             {}
func (l *recordingLogger) Error(message string)           { l.record(message) }
func (*recordingLogger) Errorf(string, ...any)            {}
func (*recordingLogger) Panic(string)                     {}
func (*recordingLogger) Panicf(string, ...any)            {}

// withRecordingLogger swaps the package-global logger for a recorder and
// restores it afterwards. Same-package tests can mutate the unexported global,
// which is the only seam available (logx has no public override hook).
func withRecordingLogger(t *testing.T) *recordingLogger {
	t.Helper()

	rec := &recordingLogger{}
	original := logger
	logger = rec
	t.Cleanup(func() { logger = original })

	return rec
}

func TestFormatKV(t *testing.T) {
	tests := []struct {
		name string
		args []any
		want string
	}{
		{name: "NoArgs", args: nil, want: ""},
		{name: "SinglePair", args: []any{"name", "job1"}, want: ", name=job1"},
		{name: "MultiplePairs", args: []any{"name", "job1", "id", 7}, want: ", name=job1, id=7"},
		{name: "OddArgCountFallsBack", args: []any{"name"}, want: ", name"},
		{name: "OddArgCountThreeFallsBack", args: []any{"name", "job1", "dangling"}, want: ", namejob1dangling"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatKV(tt.args...)
			assert.Equal(t, tt.want, got, "formatKV must mirror gocron logFormatArgs pairing and odd-count fallback")
		})
	}
}

func TestCronLoggerRendersStructuredArgs(t *testing.T) {
	rec := withRecordingLogger(t)
	log := newCronLogger()

	log.Info("gocron: limitModeRunner starting", "name", "myjob")

	require.Len(t, rec.messages, 1, "Info should emit exactly one line")
	assert.Equal(t, "gocron: limitModeRunner starting, name=myjob", rec.messages[0],
		"structured kv pairs must be appended as key=val, not consumed as printf operands")
	assert.NotContains(t, rec.messages[0], "%!(EXTRA", "non-formatting path must not produce printf EXTRA corruption")
}

func TestCronLoggerPreservesPercentInMessage(t *testing.T) {
	rec := withRecordingLogger(t)
	log := newCronLogger()

	// A literal '%' in the message must survive verbatim. The old printf-format
	// forwarding would mangle this into "%!d(MISSING)"-style garbage.
	log.Error("job %s failed: 50%% over budget", "name", "report")

	require.Len(t, rec.messages, 1, "Error should emit exactly one line")
	got := rec.messages[0]
	assert.Equal(t, "job %s failed: 50%% over budget, name=report", got, "the message must be passed through literally with kv suffix appended")
	assert.False(t, strings.Contains(got, "%!"), "no printf verb mis-parsing should occur on a percent-bearing message")
}

func TestCronLoggerAllLevels(t *testing.T) {
	rec := withRecordingLogger(t)
	log := newCronLogger()

	log.Debug("d", "k", "v")
	log.Info("i", "k", "v")
	log.Warn("w", "k", "v")
	log.Error("e", "k", "v")

	assert.Equal(t,
		[]string{"d, k=v", "i, k=v", "w, k=v", "e, k=v"},
		rec.messages,
		"every level must route through the non-formatting path with the kv suffix",
	)
}
