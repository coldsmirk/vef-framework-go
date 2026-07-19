package push

import (
	"slices"
	"sync"
	"time"

	"github.com/gofiber/contrib/v3/websocket"

	"github.com/coldsmirk/vef-framework-go/security"
)

// readLimitBytes bounds inbound data frames. The channel is downstream-only,
// so clients have nothing meaningful to send beyond control frames; the limit
// only caps junk.
const readLimitBytes = 4096

// connection is one authenticated client socket: an identity snapshot taken
// at handshake plus a bounded outbound queue drained by a single writer
// goroutine (the underlying conn forbids concurrent writes).
type connection struct {
	ws        *websocket.Conn
	userID    string
	roles     []string
	tokenHash string // empty under the stateless jwt mechanism
	sessionID string // keys the instant revocation kick; empty under jwt

	send      chan []byte
	done      chan struct{}
	writeDone chan struct{}

	closeOnce   sync.Once
	closeCode   int
	closeReason string
}

func newConnection(ws *websocket.Conn, principal *security.Principal, tokenHash, sessionID string, sendBuffer int) *connection {
	return &connection{
		ws:        ws,
		userID:    principal.ID,
		roles:     slices.Clone(principal.Roles),
		tokenHash: tokenHash,
		sessionID: sessionID,
		send:      make(chan []byte, sendBuffer),
		done:      make(chan struct{}),
		writeDone: make(chan struct{}),
	}
}

// enqueue offers a marshaled message to the writer; false means the
// connection is already closing or its buffer is full (slow client).
func (c *connection) enqueue(payload []byte) bool {
	select {
	case <-c.done:
		return false
	default:
	}

	select {
	case c.send <- payload:
		return true
	default:
		return false
	}
}

// close requests a graceful shutdown: the writer delivers a close frame with
// the given code before tearing the socket down.
func (c *connection) close(code int, reason string) {
	c.closeOnce.Do(func() {
		c.closeCode = code
		c.closeReason = reason
		close(c.done)
	})
}

// terminate requests an abrupt shutdown without a close frame — the path for
// dead or slow peers. It only signals: the writer goroutine owns the socket
// and tears it down on its own exit (a blocked write is bounded by the write
// deadline), so a stale terminate can never race the writer's close frame or
// touch a socket the contrib wrapper has already recycled.
func (c *connection) terminate() {
	c.closeOnce.Do(func() {
		close(c.done)
	})
}

// writePump is the connection's single writer: it drains the outbound queue,
// emits heartbeat pings, and on shutdown delivers the close frame. It owns
// every write on the socket AND the physical close — every exit path tears
// the socket down here, so no other goroutine ever closes it.
func (c *connection) writePump(pingInterval, writeTimeout time.Duration) {
	defer func() {
		_ = c.ws.Close()
		close(c.writeDone)
	}()

	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case payload := <-c.send:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.ws.WriteMessage(websocket.TextMessage, payload); err != nil {
				c.terminate()

				return
			}

		case <-ticker.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.terminate()

				return
			}

		case <-c.done:
			// closeCode 0 marks a terminate(): abort without a close frame.
			if c.closeCode != 0 {
				payload := websocket.FormatCloseMessage(c.closeCode, c.closeReason)
				_ = c.ws.WriteControl(websocket.CloseMessage, payload, time.Now().Add(writeTimeout))
			}

			return
		}
	}
}

// readPump blocks on the socket until the peer goes away. Inbound data frames
// are discarded (the channel is downstream-only); pongs refresh the read
// deadline, so a peer missing two heartbeats times the read out.
func (c *connection) readPump(pongWait time.Duration) {
	c.ws.SetReadLimit(readLimitBytes)
	_ = c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		return c.ws.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		if _, _, err := c.ws.ReadMessage(); err != nil {
			return
		}
	}
}
