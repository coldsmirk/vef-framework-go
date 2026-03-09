package security

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/security"
)

// newTestJWT creates a JWT instance with default test config.
func newTestJWT(t *testing.T) *security.JWT {
	t.Helper()

	jwt, err := security.NewJWT(&security.JWTConfig{
		Secret:   security.DefaultJWTSecret,
		Audience: security.DefaultJWTAudience,
	})
	require.NoError(t, err, "Should create JWT instance without error")

	return jwt
}

// newTestTokenGenerator creates a token generator with 24h expiry for testing.
func newTestTokenGenerator(jwt *security.JWT) security.TokenGenerator {
	return NewJWTTokenGenerator(jwt, &config.SecurityConfig{
		TokenExpires: 24 * time.Hour,
	})
}

// MockAuthenticator is a mock implementation of security.Authenticator.
type MockAuthenticator struct {
	mock.Mock
}

func (m *MockAuthenticator) Supports(authType string) bool {
	return m.Called(authType).Bool(0)
}

func (m *MockAuthenticator) Authenticate(ctx context.Context, authentication security.Authentication) (*security.Principal, error) {
	args := m.Called(ctx, authentication)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*security.Principal), args.Error(1)
}

// MockUserLoader is a mock implementation of security.UserLoader.
type MockUserLoader struct {
	mock.Mock
}

func (m *MockUserLoader) LoadByUsername(ctx context.Context, username string) (*security.Principal, string, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.String(1), args.Error(2)
	}

	return args.Get(0).(*security.Principal), args.String(1), args.Error(2)
}

func (m *MockUserLoader) LoadByID(ctx context.Context, id string) (*security.Principal, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*security.Principal), args.Error(1)
}

// MockPasswordEncoder is a mock implementation of password.Encoder.
type MockPasswordEncoder struct {
	mock.Mock
}

func (m *MockPasswordEncoder) Encode(password string) (string, error) {
	args := m.Called(password)

	return args.String(0), args.Error(1)
}

func (m *MockPasswordEncoder) Matches(password, encodedPassword string) bool {
	return m.Called(password, encodedPassword).Bool(0)
}

func (m *MockPasswordEncoder) UpgradeEncoding(encodedPassword string) bool {
	return m.Called(encodedPassword).Bool(0)
}

// MockRolePermissionsLoader is a mock implementation of security.RolePermissionsLoader.
type MockRolePermissionsLoader struct {
	mock.Mock
}

func (m *MockRolePermissionsLoader) LoadPermissions(ctx context.Context, role string) (map[string]security.DataScope, error) {
	args := m.Called(ctx, role)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(map[string]security.DataScope), args.Error(1)
}

// MockDataScope is a mock implementation of security.DataScope.
type MockDataScope struct {
	mock.Mock
}

func (m *MockDataScope) Key() string {
	return m.Called().String(0)
}

func (m *MockDataScope) Priority() int {
	return m.Called().Int(0)
}

func (m *MockDataScope) Supports(principal *security.Principal, table *orm.Table) bool {
	return m.Called(principal, table).Bool(0)
}

func (m *MockDataScope) Apply(principal *security.Principal, query orm.SelectQuery) error {
	return m.Called(principal, query).Error(0)
}

// MockExternalAppLoader is a mock implementation of security.ExternalAppLoader.
type MockExternalAppLoader struct {
	mock.Mock
}

func (m *MockExternalAppLoader) LoadByID(ctx context.Context, id string) (*security.Principal, string, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.String(1), args.Error(2)
	}

	return args.Get(0).(*security.Principal), args.String(1), args.Error(2)
}

// MockNonceStore is a mock implementation of security.NonceStore.
type MockNonceStore struct {
	mock.Mock
}

func (m *MockNonceStore) StoreIfAbsent(ctx context.Context, appID, nonce string, ttl time.Duration) (bool, error) {
	args := m.Called(ctx, appID, nonce, ttl)

	return args.Bool(0), args.Error(1)
}
