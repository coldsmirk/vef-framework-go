package param

import (
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/logx"
)

// WarnRecordingLogger captures formatted Warnf calls so a test can assert how
// often a report actually reaches the log.
type WarnRecordingLogger struct {
	mu    sync.Mutex
	warns []string
}

func (l *WarnRecordingLogger) Warnf(template string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.warns = append(l.warns, fmt.Sprintf(template, args...))
}

func (l *WarnRecordingLogger) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	return len(l.warns)
}

func (l *WarnRecordingLogger) Last() string {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.warns) == 0 {
		return ""
	}

	return l.warns[len(l.warns)-1]
}

func (l *WarnRecordingLogger) Named(string) logx.Logger       { return l }
func (l *WarnRecordingLogger) WithCallerSkip(int) logx.Logger { return l }
func (*WarnRecordingLogger) Enabled(logx.Level) bool          { return true }
func (*WarnRecordingLogger) Sync()                            {}
func (*WarnRecordingLogger) Debug(string)                     {}
func (*WarnRecordingLogger) Debugf(string, ...any)            {}
func (*WarnRecordingLogger) Info(string)                      {}
func (*WarnRecordingLogger) Infof(string, ...any)             {}
func (*WarnRecordingLogger) Warn(string)                      {}
func (*WarnRecordingLogger) Error(string)                     {}
func (*WarnRecordingLogger) Errorf(string, ...any)            {}
func (*WarnRecordingLogger) Panic(string)                     {}
func (*WarnRecordingLogger) Panicf(string, ...any)            {}

// installWarnRecorder swaps the package logger and clears the report state for
// one test, restoring both afterwards so tests stay independent.
func installWarnRecorder(t *testing.T) *WarnRecordingLogger {
	t.Helper()

	recorder := new(WarnRecordingLogger)
	previous := logger
	logger = recorder

	reportedUnmapped = sync.Map{}

	reportedUnmappedCount.Store(0)

	t.Cleanup(func() {
		logger = previous
		reportedUnmapped = sync.Map{}

		reportedUnmappedCount.Store(0)
	})

	return recorder
}

func TestWarnUnmappedParams(t *testing.T) {
	identifier := api.Identifier{Resource: "sys/cron/schedule", Action: "update", Version: "v1"}

	t.Run("ReportsOncePerOperationAndKeySet", func(t *testing.T) {
		recorder := installWarnRecorder(t)

		for range 5 {
			warnUnmappedParams(identifier, []string{"startsAt"})
		}

		assert.Equal(t, 1, recorder.Count(),
			"The drift describes one client build, so repeating it per request would bury the signal")
	})

	t.Run("NamesTheOperationAndTheKeys", func(t *testing.T) {
		recorder := installWarnRecorder(t)

		warnUnmappedParams(identifier, []string{"startsAt", "trigger.at"})

		assert.Contains(t, recorder.Last(), "sys/cron/schedule:update:v1",
			"The report must name the operation the drift reached")
		assert.Contains(t, recorder.Last(), "startsAt, trigger.at",
			"The report must name every ignored key, nested ones by path")
	})

	t.Run("ReportsEachDistinctKeySet", func(t *testing.T) {
		recorder := installWarnRecorder(t)

		warnUnmappedParams(identifier, []string{"startsAt"})
		warnUnmappedParams(identifier, []string{"endsAt"})

		assert.Equal(t, 2, recorder.Count(),
			"A second drift on the same operation must still surface")
	})

	t.Run("ReportsEachDistinctOperation", func(t *testing.T) {
		recorder := installWarnRecorder(t)

		warnUnmappedParams(identifier, []string{"startsAt"})
		warnUnmappedParams(api.Identifier{Resource: "sys/cron/run", Action: "list", Version: "v1"},
			[]string{"startsAt"})

		assert.Equal(t, 2, recorder.Count(),
			"The same key set reaching another operation is a separate drift")
	})

	t.Run("StaysSilentWhenEveryKeyMaps", func(t *testing.T) {
		recorder := installWarnRecorder(t)

		warnUnmappedParams(identifier, nil)

		assert.Zero(t, recorder.Count(), "A fully mapped payload must not log at all")
	})

	t.Run("StopsAtTheCap", func(t *testing.T) {
		recorder := installWarnRecorder(t)

		for i := range maxUnmappedReports + 10 {
			warnUnmappedParams(identifier, []string{"field" + strconv.Itoa(i)})
		}

		assert.Equal(t, maxUnmappedReports, recorder.Count(),
			"The report key carries caller-supplied names, so the set must stop growing at the cap")
	})
}
