package storage_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/security"
	"github.com/coldsmirk/vef-framework-go/storage"
)

// The unit tests in proxy_middleware_test.go cover the proxy's own auth
// branch; this suite covers the wiring it depends on. The regression it
// fences was invisible to both the unit tests and the framework's own
// defaults: DefaultFileACL ignores the principal entirely, so a proxy that
// never resolved one still passed every test — until an application
// supplied a FileACL that reads it, and found nil behind a perfectly valid
// Authorization header.
//
// Lives in the external test package because internal/storage cannot import
// apptest (which boots the whole graph, storage included).
type ProxyPrincipalTestSuite struct {
	apptest.Suite

	acl *RecordingProxyACL
}

// RecordingProxyACL captures what the booted framework hands its ACL.
type RecordingProxyACL struct {
	seen   *security.Principal
	called bool
}

func (a *RecordingProxyACL) CanRead(_ context.Context, principal *security.Principal, _ string) (bool, error) {
	a.called = true
	a.seen = principal

	return true, nil
}

func (s *ProxyPrincipalTestSuite) SetupSuite() {
	s.acl = new(RecordingProxyACL)

	s.SetupApp(
		fx.Replace(
			&config.StorageConfig{
				Provider:           config.StorageMemory,
				AutoMigrate:        true,
				MaxUploadSize:      testMaxUploadSize,
				AllowPublicUploads: true,
			},
			&security.JWTConfig{
				Secret:   security.DefaultJWTSecret,
				Audience: "test_app",
			},
		),
		// Exactly what an application does with vef.SupplyFileACL.
		fx.Decorate(func(storage.FileACL) storage.FileACL { return s.acl }),
	)
}

func (s *ProxyPrincipalTestSuite) TearDownSuite() {
	s.TearDownApp()
}

func (s *ProxyPrincipalTestSuite) SetupTest() {
	*s.acl = RecordingProxyACL{}
}

// get issues a download against the proxy, optionally bearing a token.
func (s *ProxyPrincipalTestSuite) get(key, token string) {
	s.T().Helper()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/"+key, nil)
	if token != "" {
		req.Header.Set(fiber.HeaderAuthorization, security.AuthSchemeBearer+" "+token)
	}

	resp, err := s.App.Test(req, 30*time.Second)
	s.Require().NoError(err, "Request should complete")
	s.Require().NoError(resp.Body.Close(), "Response body should close")
}

func (s *ProxyPrincipalTestSuite) TestPrivateDownloadCarriesTheCallerIdentity() {
	const key = "priv/2026/08/06/secret.bin"

	s.Run("AuthenticatedRequest", func() {
		token := s.GenerateToken(&security.Principal{ID: "u-1", Name: "alice", Roles: []string{"user"}})

		s.get(key, token)

		s.Require().True(s.acl.called, "The ACL should be consulted for a priv/ key")
		s.Require().NotNil(s.acl.seen, "A valid Authorization header must reach the ACL as a principal")
		s.Equal("u-1", s.acl.seen.ID, "The ACL should see the token's identity")
		s.Equal([]string{"user"}, s.acl.seen.Roles, "The principal should arrive whole, roles included")
	})

	s.Run("AnonymousRequest", func() {
		*s.acl = RecordingProxyACL{}

		s.get(key, "")

		s.Require().True(s.acl.called, "The ACL still decides for an anonymous caller")
		s.Nil(s.acl.seen, "A request with no credential reaches the ACL as nil")
	})
}

func TestProxyPrincipalTestSuite(t *testing.T) {
	suite.Run(t, new(ProxyPrincipalTestSuite))
}
