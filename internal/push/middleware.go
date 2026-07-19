package push

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/push"
	"github.com/coldsmirk/vef-framework-go/security"
)

// Locals keys carrying the handshake identity from the auth handler into the
// upgraded connection.
const (
	localPrincipal = "vef:push:principal"
	localTokenHash = "vef:push:token_hash"
	localSessionID = "vef:push:session_id"
)

// sessionRecheckTimeout bounds the post-register session lookup so a stalled
// store cannot pin a connection in its pending quarantine.
const sessionRecheckTimeout = 5 * time.Second

// tokenExtractor mirrors the bearer strategy's chain: the browser WebSocket
// API cannot set an Authorization header, so the standard access-token query
// parameter is the practical channel.
var tokenExtractor = extractors.Chain(
	extractors.FromAuthHeader(security.AuthSchemeBearer),
	extractors.FromQuery(security.QueryKeyAccessToken),
)

// Middleware mounts the push WebSocket endpoint: an app.Middleware
// registering a real route (MCP precedent) that authenticates the handshake
// with the configured token mechanism before upgrading.
type Middleware struct {
	hub       *Hub
	auth      security.AuthManager
	store     security.SessionStore
	cfg       *config.PushConfig
	tokenType string
}

// MiddlewareParams contains dependencies for creating the middleware.
type MiddlewareParams struct {
	fx.In

	Hub      *Hub
	Auth     security.AuthManager
	Store    security.SessionStore
	Config   *config.PushConfig
	Security *config.SecurityConfig
}

// NewMiddleware creates the push endpoint middleware. Returns nil while the
// endpoint is disabled.
func NewMiddleware(params MiddlewareParams) app.Middleware {
	if !params.Config.Enabled {
		return nil
	}

	return &Middleware{
		hub:       params.Hub,
		auth:      params.Auth,
		store:     params.Store,
		cfg:       params.Config,
		tokenType: string(params.Security.EffectiveTokenType()),
	}
}

func (*Middleware) Name() string {
	return "push"
}

// Order places the endpoint after the API engine and before the SPA fallback,
// alongside the integration inbound gateway and MCP.
func (*Middleware) Order() int {
	return 450
}

func (m *Middleware) Apply(router fiber.Router) {
	router.Get(m.cfg.EffectivePath(), m.authenticate, websocket.New(m.serve, websocket.Config{
		Origins:        m.cfg.AllowedOrigins,
		RecoverHandler: recoverHandler,
	}))
	logger.Infof("Push endpoint registered at %s", m.cfg.EffectivePath())
}

// authenticate guards the upgrade: the token authenticates through the
// configured mechanism before any socket exists, and the resulting identity
// rides Locals into the upgraded connection.
func (m *Middleware) authenticate(ctx fiber.Ctx) error {
	if !websocket.IsWebSocketUpgrade(ctx) {
		return fiber.ErrUpgradeRequired
	}

	token, err := tokenExtractor.Extract(ctx)
	if err != nil || token == "" {
		return security.ErrTokenInvalid
	}

	principal, err := m.auth.Authenticate(ctx.Context(), security.Authentication{
		Type:      m.tokenType,
		Principal: token,
	})
	if err != nil {
		return err
	}

	ctx.Locals(localPrincipal, principal)

	if m.tokenType == string(config.TokenTypeOpaque) {
		tokenHash := security.HashOpaqueToken(token)
		ctx.Locals(localTokenHash, tokenHash)

		// The session ID keys the instant revocation kick. A store error fails
		// open (the periodic sweep still covers the connection); a vanished
		// session means the token was revoked inside the handshake window.
		session, err := m.store.Lookup(ctx.Context(), tokenHash)
		if err == nil && session == nil {
			return security.ErrTokenInvalid
		}

		if session != nil {
			ctx.Locals(localSessionID, session.ID)
		}
	}

	return ctx.Next()
}

// serve owns one connection's lifecycle: register, run the pumps, and tear
// down. The contrib wrapper pools the Conn object after this handler returns,
// so the writer must be fully stopped before handing the socket back.
func (m *Middleware) serve(ws *websocket.Conn) {
	principal, ok := ws.Locals(localPrincipal).(*security.Principal)
	if !ok {
		_ = ws.Close()

		return
	}

	tokenHash, _ := ws.Locals(localTokenHash).(string)
	sessionID, _ := ws.Locals(localSessionID).(string)
	conn := newConnection(ws, principal, tokenHash, sessionID, m.cfg.EffectiveSendBuffer())
	// Quarantined until the session recheck passes: a pending connection is
	// kickable and holds a cap slot, but receives no pushes.
	conn.pending = true

	if err := m.hub.register(conn); err != nil {
		refuse(ws, err, m.cfg.EffectiveWriteTimeout())

		return
	}

	go conn.writePump(m.cfg.EffectivePingInterval(), m.cfg.EffectiveWriteTimeout())

	// The teardown is deferred so it also runs when anything below panics
	// (e.g. an application-provided session store): serve's defers unwind
	// before the contrib recover/release, so the writer is always fully
	// stopped — and the hub entry gone — before the wrapper is recycled.
	defer func() {
		conn.terminate()
		<-conn.writeDone
		m.hub.unregister(conn)
	}()

	m.recheckSession(conn)
	m.hub.activate(conn)

	conn.readPump(pongWait(m.cfg.EffectivePingInterval()))
}

// recheckSession closes the revocation window the handshake leaves open: a
// revocation landing between the auth lookup and register scans the hub
// before this connection is visible, so the session is looked up once more
// now that it is registered — any later revocation reaches the connection
// through the listener kick. The lookup is bounded so a stalled store cannot
// pin the connection in its quarantine; store errors and timeouts fail open
// (the periodic sweep still covers the connection).
func (m *Middleware) recheckSession(conn *connection) {
	if conn.sessionID == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), sessionRecheckTimeout)
	defer cancel()

	if session, err := m.store.Lookup(ctx, conn.tokenHash); err == nil && session == nil {
		conn.close(push.CloseSessionInvalid, "session revoked")
	}
}

// refuse closes a just-upgraded socket that the hub did not admit; the writer
// never started, so the close frame is written directly. The socket must be
// closed here too — the contrib wrapper runs with KeepHijackedConns, so a
// handler return never closes the underlying connection on its own.
func refuse(ws *websocket.Conn, err error, writeTimeout time.Duration) {
	code := push.CloseTooManyConnections
	if errors.Is(err, errHubClosed) {
		code = websocket.CloseGoingAway
	}

	payload := websocket.FormatCloseMessage(code, err.Error())
	_ = ws.WriteControl(websocket.CloseMessage, payload, time.Now().Add(writeTimeout))
	_ = ws.Close()
}

// pongWait derives the read deadline from the heartbeat period: two missed
// pongs drop the connection.
func pongWait(pingInterval time.Duration) time.Duration {
	return 2 * pingInterval
}

// recoverHandler replaces the contrib default, which echoes the panic value
// to the client; a handler panic is logged server-side only.
func recoverHandler(*websocket.Conn) {
	//nolint:revive // the contrib wrapper invokes this function itself via defer, so recover is effective here
	if r := recover(); r != nil {
		logger.Errorf("Push connection handler panicked: %v", r)
	}
}
