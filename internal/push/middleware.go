package push

import (
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
)

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
	cfg       *config.PushConfig
	tokenType string
}

// MiddlewareParams contains dependencies for creating the middleware.
type MiddlewareParams struct {
	fx.In

	Hub      *Hub
	Auth     security.AuthManager
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
		ctx.Locals(localTokenHash, security.HashOpaqueToken(token))
	}

	return ctx.Next()
}

// serve owns one connection's lifecycle: register, run the pumps, and tear
// down. The contrib wrapper pools the Conn object after this handler returns,
// so the writer must be fully stopped before handing the socket back.
func (m *Middleware) serve(ws *websocket.Conn) {
	principal, ok := ws.Locals(localPrincipal).(*security.Principal)
	if !ok {
		return
	}

	tokenHash, _ := ws.Locals(localTokenHash).(string)
	conn := newConnection(ws, principal, tokenHash, m.cfg.EffectiveSendBuffer())

	if err := m.hub.register(conn); err != nil {
		refuse(ws, err, m.cfg.EffectiveWriteTimeout())

		return
	}

	go conn.writePump(m.cfg.EffectivePingInterval(), m.cfg.EffectiveWriteTimeout())

	conn.readPump(pongWait(m.cfg.EffectivePingInterval()))

	conn.terminate()
	<-conn.writeDone
	m.hub.unregister(conn)
}

// refuse closes a just-upgraded socket that the hub did not admit; the writer
// never started, so the close frame is written directly.
func refuse(ws *websocket.Conn, err error, writeTimeout time.Duration) {
	code := push.CloseTooManyConnections
	if errors.Is(err, errHubClosed) {
		code = websocket.CloseGoingAway
	}

	payload := websocket.FormatCloseMessage(code, err.Error())
	_ = ws.WriteControl(websocket.CloseMessage, payload, time.Now().Add(writeTimeout))
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
