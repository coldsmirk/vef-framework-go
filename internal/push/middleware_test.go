package push

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fastws "github.com/fasthttp/websocket"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/push"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

// FakeAuthManager authenticates by exact token lookup, standing in for the
// configured token mechanism.
type FakeAuthManager struct {
	Principals map[string]*security.Principal
}

func (f *FakeAuthManager) Authenticate(_ context.Context, authentication security.Authentication) (*security.Principal, error) {
	if principal, ok := f.Principals[authentication.Principal]; ok {
		return principal, nil
	}

	return nil, security.ErrTokenInvalid
}

// TestServer runs the push middleware on a real listener so tests exercise
// the genuine upgrade, pumps, and close-frame paths.
type TestServer struct {
	Hub  *Hub
	Addr string
}

func StartTestServer(t *testing.T, cfg *config.PushConfig, securityCfg *config.SecurityConfig, auth security.AuthManager) *TestServer {
	t.Helper()

	hub := NewHub(cfg)
	middleware := NewMiddleware(MiddlewareParams{Hub: hub, Auth: auth, Config: cfg, Security: securityCfg})
	require.NotNil(t, middleware, "Middleware should be built when the endpoint is enabled")

	fiberApp := fiber.New(fiber.Config{
		// The production app maps result.Error onto its transport status; the
		// handshake tests assert those statuses, so the mapping is replicated.
		ErrorHandler: func(ctx fiber.Ctx, err error) error {
			if resultErr, ok := errors.AsType[result.Error](err); ok {
				return ctx.SendStatus(resultErr.Status)
			}

			if fiberErr, ok := errors.AsType[*fiber.Error](err); ok {
				return ctx.SendStatus(fiberErr.Code)
			}

			return ctx.SendStatus(fiber.StatusInternalServerError)
		},
	})
	middleware.Apply(fiberApp)

	listener, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err, "Test listener should bind")

	go func() {
		_ = fiberApp.Listener(listener, fiber.ListenConfig{DisableStartupMessage: true})
	}()

	t.Cleanup(func() {
		_ = fiberApp.Shutdown()
	})

	return &TestServer{Hub: hub, Addr: listener.Addr().String()}
}

func (s *TestServer) URL(token string) string {
	url := "ws://" + s.Addr + "/ws"
	if token != "" {
		url += "?" + security.QueryKeyAccessToken + "=" + token
	}

	return url
}

func Dial(t *testing.T, server *TestServer, token string) *fastws.Conn {
	t.Helper()

	conn, _, err := fastws.DefaultDialer.Dial(server.URL(token), nil)
	require.NoError(t, err, "Handshake with a valid token should succeed")

	t.Cleanup(func() {
		_ = conn.Close()
	})

	return conn
}

func ReadEnvelope(t *testing.T, conn *fastws.Conn) push.Message {
	t.Helper()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)), "Read deadline should apply")

	_, payload, err := conn.ReadMessage()
	require.NoError(t, err, "A push message should arrive")

	var message push.Message
	require.NoError(t, json.Unmarshal(payload, &message), "The frame should carry the JSON envelope")

	return message
}

func ExpectClose(t *testing.T, conn *fastws.Conn, code int) {
	t.Helper()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)), "Read deadline should apply")

	_, _, err := conn.ReadMessage()
	require.Error(t, err, "The server should close the connection")
	assert.True(t, fastws.IsCloseError(err, code), "Close code should be %d, got: %v", code, err)
}

func UserPrincipal(userID string, roles ...string) *security.Principal {
	return &security.Principal{Type: security.PrincipalTypeUser, ID: userID, Name: userID, Roles: roles}
}

// HubSize reads the live connection count under the hub lock; tests poll it
// while server goroutines mutate the registry.
func HubSize(h *Hub) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.conns)
}

func EnabledConfig() *config.PushConfig {
	return &config.PushConfig{Enabled: true}
}

func TestNewMiddleware(t *testing.T) {
	t.Run("DisabledReturnsNil", func(t *testing.T) {
		middleware := NewMiddleware(MiddlewareParams{
			Hub:      NewHub(new(config.PushConfig)),
			Auth:     &FakeAuthManager{},
			Config:   new(config.PushConfig),
			Security: new(config.SecurityConfig),
		})

		assert.Nil(t, middleware, "A disabled endpoint should contribute no middleware")
	})

	t.Run("Identity", func(t *testing.T) {
		middleware := NewMiddleware(MiddlewareParams{
			Hub:      NewHub(EnabledConfig()),
			Auth:     &FakeAuthManager{},
			Config:   EnabledConfig(),
			Security: new(config.SecurityConfig),
		})

		require.NotNil(t, middleware, "An enabled endpoint should contribute the middleware")
		assert.Equal(t, "push", middleware.Name(), "Name should identify the module")
		assert.Equal(t, 450, middleware.Order(), "Order should sit between the API engine and the SPA fallback")
	})
}

func TestHandshake(t *testing.T) {
	auth := &FakeAuthManager{Principals: map[string]*security.Principal{"good": UserPrincipal("alice")}}
	server := StartTestServer(t, EnabledConfig(), new(config.SecurityConfig), auth)

	t.Run("RejectsMissingToken", func(t *testing.T) {
		conn, resp, err := fastws.DefaultDialer.Dial(server.URL(""), nil)
		require.Error(t, err, "A tokenless handshake must fail")
		require.Nil(t, conn, "No connection should be established")
		assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode, "The handshake should be refused with 401")
	})

	t.Run("RejectsInvalidToken", func(t *testing.T) {
		conn, resp, err := fastws.DefaultDialer.Dial(server.URL("bad"), nil)
		require.Error(t, err, "An invalid token must fail the handshake")
		require.Nil(t, conn, "No connection should be established")
		assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode, "The handshake should be refused with 401")
	})

	t.Run("RejectsPlainHTTP", func(t *testing.T) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+server.Addr+"/ws", nil)
		require.NoError(t, err, "Request should build")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "A plain GET should get a response")

		defer resp.Body.Close()

		assert.Equal(t, fiber.StatusUpgradeRequired, resp.StatusCode, "A non-upgrade request should get 426")
	})

	t.Run("AcceptsValidToken", func(t *testing.T) {
		conn := Dial(t, server, "good")

		assert.NotNil(t, conn, "A valid token should open the socket")
		assert.Eventually(t, func() bool { return HubSize(server.Hub) == 1 }, time.Second, 10*time.Millisecond,
			"The connection should be registered in the hub")
	})
}

func TestPushDelivery(t *testing.T) {
	auth := &FakeAuthManager{Principals: map[string]*security.Principal{
		"alice-token": UserPrincipal("alice", "admin"),
		"bob-token":   UserPrincipal("bob"),
	}}
	server := StartTestServer(t, EnabledConfig(), new(config.SecurityConfig), auth)

	alice1 := Dial(t, server, "alice-token")
	alice2 := Dial(t, server, "alice-token")
	bob := Dial(t, server, "bob-token")

	require.Eventually(t, func() bool { return HubSize(server.Hub) == 3 }, time.Second, 10*time.Millisecond,
		"All three connections should register")

	ctx := context.Background()

	require.NoError(t, server.Hub.Push(ctx, push.NewMessage("to.users", "A"), push.ToUsers("alice")),
		"User-targeted push should succeed")
	require.NoError(t, server.Hub.Push(ctx, push.NewMessage("to.roles", "R"), push.ToRoles("admin")),
		"Role-targeted push should succeed")
	require.NoError(t, server.Hub.Push(ctx, push.NewMessage("to.all", "B"), push.Broadcast()),
		"Broadcast push should succeed")

	for _, conn := range []*fastws.Conn{alice1, alice2} {
		first := ReadEnvelope(t, conn)
		assert.Equal(t, "to.users", first.Type, "Alice should receive the user-targeted message first")
		assert.Equal(t, "A", first.Payload, "Payload should ride the envelope")
		assert.NotEmpty(t, first.ID, "The envelope should carry a message id")
		assert.False(t, first.Time.IsZero(), "The envelope should carry the send time")

		assert.Equal(t, "to.roles", ReadEnvelope(t, conn).Type, "Alice holds the admin role and should get the role push")
		assert.Equal(t, "to.all", ReadEnvelope(t, conn).Type, "Alice should get the broadcast")
	}

	first := ReadEnvelope(t, bob)
	assert.Equal(t, "to.all", first.Type, "Bob's first message should be the broadcast — user and role pushes must not reach him")

	t.Run("OverlappingTargetsDeliverOnce", func(t *testing.T) {
		require.NoError(t, server.Hub.Push(ctx, push.NewMessage("dedup", nil), push.ToUsers("alice"), push.Broadcast()),
			"Push with overlapping targets should succeed")
		require.NoError(t, server.Hub.Push(ctx, push.NewMessage("marker", nil), push.Broadcast()),
			"Marker push should succeed")

		assert.Equal(t, "dedup", ReadEnvelope(t, alice1).Type, "Alice should receive the overlapping-target message")
		assert.Equal(t, "marker", ReadEnvelope(t, alice1).Type, "Exactly one copy should be delivered despite two matching targets")
	})
}

func TestConnectionLimit(t *testing.T) {
	auth := &FakeAuthManager{Principals: map[string]*security.Principal{"alice-token": UserPrincipal("alice")}}
	cfg := EnabledConfig()
	cfg.MaxConnectionsPerUser = 1
	server := StartTestServer(t, cfg, new(config.SecurityConfig), auth)

	first := Dial(t, server, "alice-token")
	require.Eventually(t, func() bool { return HubSize(server.Hub) == 1 }, time.Second, 10*time.Millisecond,
		"The first connection should register")

	second := Dial(t, server, "alice-token")
	ExpectClose(t, second, push.CloseTooManyConnections)

	require.NoError(t, server.Hub.Push(context.Background(), push.NewMessage("still.alive", nil), push.ToUsers("alice")),
		"Push to the surviving connection should succeed")
	assert.Equal(t, "still.alive", ReadEnvelope(t, first).Type, "The first connection should stay functional")
}

func TestSessionSweepEndToEnd(t *testing.T) {
	token := "opaque-token"
	store := security.NewMemorySessionStore()
	require.NoError(t, store.Create(context.Background(), security.HashOpaqueToken(token),
		security.Session{ID: "s1", UserID: "alice", ExpiresAt: time.Now().Add(time.Hour)}, time.Hour),
		"Session should be created")

	auth := &FakeAuthManager{Principals: map[string]*security.Principal{token: UserPrincipal("alice")}}
	securityCfg := &config.SecurityConfig{TokenType: config.TokenTypeOpaque}
	server := StartTestServer(t, EnabledConfig(), securityCfg, auth)
	sweeper := newSessionSweeper(server.Hub, store, time.Minute)

	conn := Dial(t, server, token)
	require.Eventually(t, func() bool { return HubSize(server.Hub) == 1 }, time.Second, 10*time.Millisecond,
		"The connection should register")

	sweeper.sweep(context.Background())
	require.NoError(t, server.Hub.Push(context.Background(), push.NewMessage("alive", nil), push.ToUsers("alice")),
		"Push after a clean sweep should succeed")
	assert.Equal(t, "alive", ReadEnvelope(t, conn).Type, "A live session must survive the sweep")

	require.NoError(t, store.Revoke(context.Background(), "s1"), "Session should revoke")
	sweeper.sweep(context.Background())
	ExpectClose(t, conn, push.CloseSessionInvalid)

	assert.Eventually(t, func() bool { return HubSize(server.Hub) == 0 }, time.Second, 10*time.Millisecond,
		"The kicked connection should unregister")
}

func TestHubShutdownEndToEnd(t *testing.T) {
	auth := &FakeAuthManager{Principals: map[string]*security.Principal{"good": UserPrincipal("alice")}}
	server := StartTestServer(t, EnabledConfig(), new(config.SecurityConfig), auth)

	conn := Dial(t, server, "good")
	require.Eventually(t, func() bool { return HubSize(server.Hub) == 1 }, time.Second, 10*time.Millisecond,
		"The connection should register")

	server.Hub.Shutdown()
	ExpectClose(t, conn, fastws.CloseGoingAway)

	late := Dial(t, server, "good")
	ExpectClose(t, late, fastws.CloseGoingAway)
}

func TestHeartbeatDropsUnresponsiveClient(t *testing.T) {
	auth := &FakeAuthManager{Principals: map[string]*security.Principal{"good": UserPrincipal("alice")}}
	cfg := EnabledConfig()
	cfg.PingInterval = 100 * time.Millisecond
	server := StartTestServer(t, cfg, new(config.SecurityConfig), auth)

	// The client never reads, so pings are never answered and the server's
	// read deadline (two ping periods) expires.
	Dial(t, server, "good")
	require.Eventually(t, func() bool { return HubSize(server.Hub) == 1 }, time.Second, 10*time.Millisecond,
		"The connection should register")

	assert.Eventually(t, func() bool { return HubSize(server.Hub) == 0 }, 3*time.Second, 20*time.Millisecond,
		"An unresponsive client should be dropped after missing two heartbeats")
}
