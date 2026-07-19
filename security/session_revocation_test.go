package security

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// RecordingRevocationListener captures every notification it receives.
type RecordingRevocationListener struct {
	Calls [][]SessionRevocation
}

func (l *RecordingRevocationListener) OnSessionsRevoked(_ context.Context, revocations []SessionRevocation) {
	l.Calls = append(l.Calls, revocations)
}

func TestSessionRevocationNotifier(t *testing.T) {
	ctx := context.Background()

	t.Run("FansOutToListeners", func(t *testing.T) {
		first := new(RecordingRevocationListener)
		second := new(RecordingRevocationListener)
		notifier := NewSessionRevocationNotifier([]SessionRevocationListener{first, second})

		revocation := SessionRevocation{SessionID: "s1", UserID: "alice"}
		notifier.NotifyRevoked(ctx, revocation)

		assert.Equal(t, [][]SessionRevocation{{revocation}}, first.Calls, "First listener should receive the revocation")
		assert.Equal(t, [][]SessionRevocation{{revocation}}, second.Calls, "Second listener should receive the revocation")
	})

	t.Run("DropsNilListeners", func(t *testing.T) {
		listener := new(RecordingRevocationListener)
		notifier := NewSessionRevocationNotifier([]SessionRevocationListener{nil, listener})

		notifier.NotifyRevoked(ctx, SessionRevocation{SessionID: "s1"})

		assert.Len(t, listener.Calls, 1, "A nil group entry must not break the fan-out")
	})

	t.Run("EmptyRevocationsAreDropped", func(t *testing.T) {
		listener := new(RecordingRevocationListener)
		notifier := NewSessionRevocationNotifier([]SessionRevocationListener{listener})

		notifier.NotifyRevoked(ctx)

		assert.Empty(t, listener.Calls, "An empty revocation set should not reach listeners")
	})

	t.Run("NilReceiverIsSafe", func(t *testing.T) {
		var notifier *SessionRevocationNotifier

		assert.NotPanics(t, func() {
			notifier.NotifyRevoked(ctx, SessionRevocation{SessionID: "s1"})
		}, "A nil notifier must be a silent no-op")
	})
}
