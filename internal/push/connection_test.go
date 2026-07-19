package push

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/security"
)

// ConnectionClosing reports whether the connection has entered shutdown.
func ConnectionClosing(c *connection) bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

func TestNewConnectionSnapshotsRoles(t *testing.T) {
	principal := &security.Principal{ID: "alice", Roles: []string{"admin"}}
	conn := newConnection(nil, principal, "hash", "s1", 2)

	principal.Roles[0] = "mutated"

	assert.Equal(t, []string{"admin"}, conn.roles,
		"Roles must be snapshotted at the connection boundary, not aliased to the principal")
}

func TestConnectionEnqueue(t *testing.T) {
	t.Run("FullBufferRefuses", func(t *testing.T) {
		conn := newTestConnectionWithBuffer(1)

		assert.True(t, conn.enqueue([]byte("first")), "A free slot should accept the message")
		assert.False(t, conn.enqueue([]byte("second")), "A full buffer marks the client slow and must refuse")
	})

	t.Run("ClosedConnectionRefuses", func(t *testing.T) {
		conn := newTestConnectionWithBuffer(4)
		conn.close(4000, "test")

		assert.False(t, conn.enqueue([]byte("late")), "A closing connection must refuse new messages")
	})
}

func TestConnectionClose(t *testing.T) {
	conn := newTestConnectionWithBuffer(1)

	require.False(t, ConnectionClosing(conn), "A fresh connection should not be closing")

	conn.close(4401, "session invalid")
	conn.close(4000, "second close is ignored")

	assert.True(t, ConnectionClosing(conn), "close should enter shutdown")
	assert.Equal(t, 4401, conn.closeCode, "The first close code should win")
	assert.Equal(t, "session invalid", conn.closeReason, "The first close reason should win")
}

func TestConnectionTerminate(t *testing.T) {
	t.Run("SignalsWithoutTouchingTheSocket", func(t *testing.T) {
		// ws is nil: any socket touch would panic. terminate must be a pure
		// signal — the writer goroutine owns the physical close.
		conn := newTestConnectionWithBuffer(1)

		conn.terminate()

		assert.True(t, ConnectionClosing(conn), "terminate should enter shutdown")
		assert.Zero(t, conn.closeCode, "terminate carries no close code")
	})

	t.Run("KeepsAGracefulCloseCode", func(t *testing.T) {
		conn := newTestConnectionWithBuffer(1)

		conn.close(4401, "session invalid")
		conn.terminate()

		assert.Equal(t, 4401, conn.closeCode, "A later terminate must not downgrade the graceful close")
	})
}

func newTestConnectionWithBuffer(size int) *connection {
	return &connection{
		userID:    "u",
		send:      make(chan []byte, size),
		done:      make(chan struct{}),
		writeDone: make(chan struct{}),
	}
}
