package security

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

const testSecretHex = security.DefaultJWTSecret

type SignatureAuthenticatorTestSuite struct {
	suite.Suite
}

// generateValidCredentials creates valid signature credentials for testing.
func (s *SignatureAuthenticatorTestSuite) generateValidCredentials(appID, secret string) *security.SignatureCredentials {
	s.T().Helper()

	sig, err := security.NewSignature(secret, security.WithNonceStore(nil))
	s.Require().NoError(err, "Should create signature instance")

	result, err := sig.Sign(appID)
	s.Require().NoError(err, "Should sign successfully")

	return &security.SignatureCredentials{
		Timestamp: result.Timestamp,
		Nonce:     result.Nonce,
		Signature: result.Signature,
	}
}

// TestNew verifies constructor variants.
func (s *SignatureAuthenticatorTestSuite) TestNew() {
	s.Run("WithLoader", func() {
		auth := NewSignatureAuthenticator(new(MockExternalAppLoader), nil)
		s.NotNil(auth, "Authenticator should not be nil")
	})

	s.Run("WithoutLoader", func() {
		auth := NewSignatureAuthenticator(nil, nil)
		s.NotNil(auth, "Authenticator should not be nil even without loader")
	})

	s.Run("WithNonceStore", func() {
		auth := NewSignatureAuthenticator(new(MockExternalAppLoader), new(MockNonceStore))
		s.NotNil(auth, "Authenticator should not be nil")
	})
}

// TestSupports verifies type matching.
func (s *SignatureAuthenticatorTestSuite) TestSupports() {
	auth := NewSignatureAuthenticator(new(MockExternalAppLoader), nil)

	s.True(auth.Supports(AuthTypeSignature), "Should support signature type")

	for _, authType := range []string{"password", "token", "jwt", "openapi", "bearer", ""} {
		s.Run(authType, func() {
			s.False(auth.Supports(authType), "Should not support %q type", authType)
		})
	}
}

// TestAuthenticate verifies all authentication paths.
func (s *SignatureAuthenticatorTestSuite) TestAuthenticate() {
	ctx := context.Background()

	s.Run("MissingAppID", func() {
		auth := NewSignatureAuthenticator(new(MockExternalAppLoader), nil)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeSignature,
			Principal: "",
			Credentials: &security.SignatureCredentials{
				Timestamp: time.Now().Unix(),
				Nonce:     "abcdefghijklmnop",
				Signature: "signature",
			},
		})
		s.Require().Error(err, "Should return error for empty appID")
	})

	s.Run("InvalidCredentials", func() {
		auth := NewSignatureAuthenticator(new(MockExternalAppLoader), nil)

		testCases := []struct {
			name        string
			credentials any
		}{
			{"NilCredentials", nil},
			{"StringCredentials", "string credentials"},
			{"WrongStructType", struct{ Foo string }{Foo: "bar"}},
			{"IntCredentials", 12345},
			{"MapCredentials", map[string]any{"key": "value"}},
		}

		for _, tc := range testCases {
			s.Run(tc.name, func() {
				_, err := auth.Authenticate(ctx, security.Authentication{
					Type:        AuthTypeSignature,
					Principal:   "app1",
					Credentials: tc.credentials,
				})
				s.Require().Error(err, "Should return error for invalid credentials type")
			})
		}
	})

	s.Run("AppNotFound", func() {
		loader := new(MockExternalAppLoader)
		loader.On("LoadByID", mock.Anything, "app1").Return(nil, "", nil)

		auth := NewSignatureAuthenticator(loader, nil)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeSignature,
			Principal: "app1",
			Credentials: &security.SignatureCredentials{
				Timestamp: time.Now().Unix(),
				Nonce:     "abcdefghijklmnop",
				Signature: "signature",
			},
		})
		s.Require().Error(err, "Should return error when app is not found")
		loader.AssertExpectations(s.T())
	})

	s.Run("LoaderReturnsError", func() {
		loader := new(MockExternalAppLoader)
		expectedErr := errors.New("database connection failed")
		loader.On("LoadByID", mock.Anything, "app1").Return(nil, "", expectedErr)

		auth := NewSignatureAuthenticator(loader, nil)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeSignature,
			Principal: "app1",
			Credentials: &security.SignatureCredentials{
				Timestamp: time.Now().Unix(),
				Nonce:     "abcdefghijklmnop",
				Signature: "signature",
			},
		})
		s.ErrorIs(err, expectedErr, "Should return loader error")
		loader.AssertExpectations(s.T())
	})

	s.Run("EmptySecret", func() {
		loader := new(MockExternalAppLoader)
		principal := security.NewExternalApp("app1", "Test App", "api_user")
		loader.On("LoadByID", mock.Anything, "app1").Return(principal, "", nil)

		auth := NewSignatureAuthenticator(loader, nil)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeSignature,
			Principal: "app1",
			Credentials: &security.SignatureCredentials{
				Timestamp: time.Now().Unix(),
				Nonce:     "abcdefghijklmnop",
				Signature: "signature",
			},
		})
		s.Require().Error(err, "Should return error when secret is empty")
		loader.AssertExpectations(s.T())
	})

	s.Run("NilLoader", func() {
		auth := NewSignatureAuthenticator(nil, nil)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeSignature,
			Principal: "app1",
			Credentials: &security.SignatureCredentials{
				Timestamp: time.Now().Unix(),
				Nonce:     "abcdefghijklmnop",
				Signature: "signature",
			},
		})
		s.Require().Error(err, "Should return error when loader is nil")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeNotImplemented, resErr.Code, "Should return not implemented code")
	})

	s.Run("InvalidSecretFormat", func() {
		loader := new(MockExternalAppLoader)
		principal := security.NewExternalApp("app1", "Test App", "api_user")
		loader.On("LoadByID", mock.Anything, "app1").Return(principal, "not-valid-hex", nil)

		auth := NewSignatureAuthenticator(loader, nil)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeSignature,
			Principal: "app1",
			Credentials: &security.SignatureCredentials{
				Timestamp: time.Now().Unix(),
				Nonce:     "abcdefghijklmnop",
				Signature: "0000000000000000000000000000000000000000000000000000000000000000",
			},
		})
		s.Require().Error(err, "Should return error when secret is not valid hex")
		loader.AssertExpectations(s.T())
	})
}

// TestTimestampValidation verifies timestamp validation scenarios.
func (s *SignatureAuthenticatorTestSuite) TestTimestampValidation() {
	ctx := context.Background()

	s.Run("ExpiredTimestamp", func() {
		loader := new(MockExternalAppLoader)
		principal := security.NewExternalApp("app1", "Test App", "api_user")
		loader.On("LoadByID", mock.Anything, "app1").Return(principal, testSecretHex, nil)

		auth := NewSignatureAuthenticator(loader, nil)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeSignature,
			Principal: "app1",
			Credentials: &security.SignatureCredentials{
				Timestamp: time.Now().Add(-10 * time.Minute).Unix(),
				Nonce:     "abcdefghijklmnop",
				Signature: "0000000000000000000000000000000000000000000000000000000000000000",
			},
		})
		s.Require().Error(err, "Should return error for expired timestamp")
		loader.AssertExpectations(s.T())
	})

	s.Run("FutureTimestamp", func() {
		loader := new(MockExternalAppLoader)
		principal := security.NewExternalApp("app1", "Test App", "api_user")
		loader.On("LoadByID", mock.Anything, "app1").Return(principal, testSecretHex, nil)

		auth := NewSignatureAuthenticator(loader, nil)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeSignature,
			Principal: "app1",
			Credentials: &security.SignatureCredentials{
				Timestamp: time.Now().Add(10 * time.Minute).Unix(),
				Nonce:     "abcdefghijklmnop",
				Signature: "0000000000000000000000000000000000000000000000000000000000000000",
			},
		})
		s.Require().Error(err, "Should return error for future timestamp")
		loader.AssertExpectations(s.T())
	})

	s.Run("ValidTimestampWithinTolerance", func() {
		loader := new(MockExternalAppLoader)
		nonceStore := new(MockNonceStore)

		principal := security.NewExternalApp("app1", "Test App", "api_user")
		loader.On("LoadByID", mock.Anything, "app1").Return(principal, testSecretHex, nil)
		nonceStore.On("StoreIfAbsent", mock.Anything, "app1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(true, nil)

		auth := NewSignatureAuthenticator(loader, nonceStore)
		credentials := s.generateValidCredentials("app1", testSecretHex)

		got, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypeSignature,
			Principal:   "app1",
			Credentials: credentials,
		})
		s.Require().NoError(err, "Should authenticate with valid timestamp")
		s.Equal("app1", got.ID, "Principal ID should match")
		loader.AssertExpectations(s.T())
		nonceStore.AssertExpectations(s.T())
	})
}

// TestSignatureValidation verifies signature validation scenarios.
func (s *SignatureAuthenticatorTestSuite) TestSignatureValidation() {
	ctx := context.Background()

	s.Run("InvalidSignature", func() {
		loader := new(MockExternalAppLoader)
		nonceStore := new(MockNonceStore)

		principal := security.NewExternalApp("app1", "Test App", "api_user")
		loader.On("LoadByID", mock.Anything, "app1").Return(principal, testSecretHex, nil)
		nonceStore.On("StoreIfAbsent", mock.Anything, "app1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(true, nil)

		auth := NewSignatureAuthenticator(loader, nonceStore)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeSignature,
			Principal: "app1",
			Credentials: &security.SignatureCredentials{
				Timestamp: time.Now().Unix(),
				Nonce:     "abcdefghijklmnop",
				Signature: "0000000000000000000000000000000000000000000000000000000000000000",
			},
		})
		s.Require().Error(err, "Should return error for invalid signature")
		loader.AssertExpectations(s.T())
	})

	s.Run("MalformedSignature", func() {
		loader := new(MockExternalAppLoader)
		nonceStore := new(MockNonceStore)

		principal := security.NewExternalApp("app1", "Test App", "api_user")
		loader.On("LoadByID", mock.Anything, "app1").Return(principal, testSecretHex, nil)
		nonceStore.On("StoreIfAbsent", mock.Anything, "app1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(true, nil)

		auth := NewSignatureAuthenticator(loader, nonceStore)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeSignature,
			Principal: "app1",
			Credentials: &security.SignatureCredentials{
				Timestamp: time.Now().Unix(),
				Nonce:     "abcdefghijklmnop",
				Signature: "not-valid-hex",
			},
		})
		s.Require().Error(err, "Should return error for malformed signature")
		loader.AssertExpectations(s.T())
	})

	s.Run("WrongSecret", func() {
		loader := new(MockExternalAppLoader)
		nonceStore := new(MockNonceStore)

		principal := security.NewExternalApp("app1", "Test App", "api_user")
		wrongSecret := "0000000000000000000000000000000000000000000000000000000000000000"
		loader.On("LoadByID", mock.Anything, "app1").Return(principal, wrongSecret, nil)
		nonceStore.On("StoreIfAbsent", mock.Anything, "app1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(true, nil)

		auth := NewSignatureAuthenticator(loader, nonceStore)
		credentials := s.generateValidCredentials("app1", testSecretHex)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypeSignature,
			Principal:   "app1",
			Credentials: credentials,
		})
		s.Require().Error(err, "Should return error when secret doesn't match")
		loader.AssertExpectations(s.T())
	})
}

// TestNonceValidation verifies nonce validation scenarios.
func (s *SignatureAuthenticatorTestSuite) TestNonceValidation() {
	ctx := context.Background()

	s.Run("NonceAlreadyUsed", func() {
		loader := new(MockExternalAppLoader)
		nonceStore := new(MockNonceStore)

		principal := security.NewExternalApp("app1", "Test App", "api_user")
		loader.On("LoadByID", mock.Anything, "app1").Return(principal, testSecretHex, nil)
		nonceStore.On("StoreIfAbsent", mock.Anything, "app1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(false, nil)

		auth := NewSignatureAuthenticator(loader, nonceStore)
		credentials := s.generateValidCredentials("app1", testSecretHex)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypeSignature,
			Principal:   "app1",
			Credentials: credentials,
		})
		s.Require().Error(err, "Should return error when nonce is already used")
		loader.AssertExpectations(s.T())
		nonceStore.AssertExpectations(s.T())
	})

	s.Run("NonceStoreStoreIfAbsentCheckError", func() {
		loader := new(MockExternalAppLoader)
		nonceStore := new(MockNonceStore)

		principal := security.NewExternalApp("app1", "Test App", "api_user")
		loader.On("LoadByID", mock.Anything, "app1").Return(principal, testSecretHex, nil)
		nonceStore.On("StoreIfAbsent", mock.Anything, "app1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(false, errors.New("redis connection failed"))

		auth := NewSignatureAuthenticator(loader, nonceStore)
		credentials := s.generateValidCredentials("app1", testSecretHex)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypeSignature,
			Principal:   "app1",
			Credentials: credentials,
		})
		s.Require().Error(err, "Should return error when nonce store fails")
		loader.AssertExpectations(s.T())
		nonceStore.AssertExpectations(s.T())
	})

	s.Run("NonceRequired", func() {
		loader := new(MockExternalAppLoader)
		principal := security.NewExternalApp("app1", "Test App", "api_user")
		loader.On("LoadByID", mock.Anything, "app1").Return(principal, testSecretHex, nil)

		auth := NewSignatureAuthenticator(loader, nil)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeSignature,
			Principal: "app1",
			Credentials: &security.SignatureCredentials{
				Timestamp: time.Now().Unix(),
				Nonce:     "",
				Signature: "0000000000000000000000000000000000000000000000000000000000000000",
			},
		})
		s.Require().Error(err, "Should return error when nonce is empty")
		s.ErrorIs(err, result.ErrNonceRequired, "Should return nonce required error")
		loader.AssertExpectations(s.T())
	})

	s.Run("NonceStoreStoreIfAbsentWriteError", func() {
		loader := new(MockExternalAppLoader)
		nonceStore := new(MockNonceStore)

		principal := security.NewExternalApp("app1", "Test App", "api_user")
		loader.On("LoadByID", mock.Anything, "app1").Return(principal, testSecretHex, nil)
		nonceStore.On("StoreIfAbsent", mock.Anything, "app1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(false, errors.New("redis write failed"))

		auth := NewSignatureAuthenticator(loader, nonceStore)
		credentials := s.generateValidCredentials("app1", testSecretHex)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypeSignature,
			Principal:   "app1",
			Credentials: credentials,
		})
		s.Require().Error(err, "Should return error when nonce store write fails")
		loader.AssertExpectations(s.T())
		nonceStore.AssertExpectations(s.T())
	})
}

// TestIPWhitelist verifies IP whitelist and app config scenarios.
func (s *SignatureAuthenticatorTestSuite) TestIPWhitelist() {
	ctx := context.Background()

	s.Run("DisabledApp", func() {
		loader := new(MockExternalAppLoader)

		principal := security.NewExternalApp("app1", "Test App", "api_user")
		principal.Details = &security.ExternalAppConfig{
			Enabled: false,
		}
		loader.On("LoadByID", mock.Anything, "app1").Return(principal, testSecretHex, nil)

		auth := NewSignatureAuthenticator(loader, nil)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:      AuthTypeSignature,
			Principal: "app1",
			Credentials: &security.SignatureCredentials{
				Timestamp: time.Now().Unix(),
				Nonce:     "abcdefghijklmnop",
				Signature: "signature",
			},
		})
		s.Require().Error(err, "Should return error for disabled app")
		loader.AssertExpectations(s.T())
	})

	s.Run("EnabledAppWithoutWhitelist", func() {
		loader := new(MockExternalAppLoader)
		nonceStore := new(MockNonceStore)

		principal := security.NewExternalApp("app1", "Test App", "api_user")
		principal.Details = &security.ExternalAppConfig{
			Enabled:     true,
			IPWhitelist: "",
		}
		loader.On("LoadByID", mock.Anything, "app1").Return(principal, testSecretHex, nil)
		nonceStore.On("StoreIfAbsent", mock.Anything, "app1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(true, nil)

		auth := NewSignatureAuthenticator(loader, nonceStore)
		credentials := s.generateValidCredentials("app1", testSecretHex)

		got, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypeSignature,
			Principal:   "app1",
			Credentials: credentials,
		})
		s.Require().NoError(err, "Should authenticate when app is enabled without whitelist")
		s.Equal("app1", got.ID, "Principal ID should match")
		loader.AssertExpectations(s.T())
		nonceStore.AssertExpectations(s.T())
	})

	s.Run("WhitelistWithEmptyRequestIP", func() {
		loader := new(MockExternalAppLoader)
		nonceStore := new(MockNonceStore)

		principal := security.NewExternalApp("app1", "Test App", "api_user")
		principal.Details = &security.ExternalAppConfig{
			Enabled:     true,
			IPWhitelist: "192.168.1.0/24",
		}
		loader.On("LoadByID", mock.Anything, "app1").Return(principal, testSecretHex, nil)
		nonceStore.On("StoreIfAbsent", mock.Anything, "app1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(true, nil)

		auth := NewSignatureAuthenticator(loader, nonceStore)
		credentials := s.generateValidCredentials("app1", testSecretHex)

		// No IP set in context — should pass whitelist check
		got, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypeSignature,
			Principal:   "app1",
			Credentials: credentials,
		})
		s.Require().NoError(err, "Should pass when request IP is empty")
		s.Equal("app1", got.ID, "Principal ID should match")
		loader.AssertExpectations(s.T())
		nonceStore.AssertExpectations(s.T())
	})

	s.Run("WhitelistIPAllowed", func() {
		loader := new(MockExternalAppLoader)
		nonceStore := new(MockNonceStore)

		principal := security.NewExternalApp("app1", "Test App", "api_user")
		principal.Details = &security.ExternalAppConfig{
			Enabled:     true,
			IPWhitelist: "192.168.1.0/24",
		}
		loader.On("LoadByID", mock.Anything, "app1").Return(principal, testSecretHex, nil)
		nonceStore.On("StoreIfAbsent", mock.Anything, "app1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(true, nil)

		auth := NewSignatureAuthenticator(loader, nonceStore)
		credentials := s.generateValidCredentials("app1", testSecretHex)

		ipCtx := contextx.SetRequestIP(ctx, "192.168.1.100")
		got, err := auth.Authenticate(ipCtx, security.Authentication{
			Type:        AuthTypeSignature,
			Principal:   "app1",
			Credentials: credentials,
		})
		s.Require().NoError(err, "Should pass when IP is in whitelist")
		s.Equal("app1", got.ID, "Principal ID should match")
		loader.AssertExpectations(s.T())
		nonceStore.AssertExpectations(s.T())
	})

	s.Run("WhitelistIPBlocked", func() {
		loader := new(MockExternalAppLoader)

		principal := security.NewExternalApp("app1", "Test App", "api_user")
		principal.Details = &security.ExternalAppConfig{
			Enabled:     true,
			IPWhitelist: "192.168.1.0/24",
		}
		loader.On("LoadByID", mock.Anything, "app1").Return(principal, testSecretHex, nil)

		auth := NewSignatureAuthenticator(loader, nil)

		ipCtx := contextx.SetRequestIP(ctx, "10.0.0.1")
		_, err := auth.Authenticate(ipCtx, security.Authentication{
			Type:      AuthTypeSignature,
			Principal: "app1",
			Credentials: &security.SignatureCredentials{
				Timestamp: time.Now().Unix(),
				Nonce:     "abcdefghijklmnop",
				Signature: "0000000000000000000000000000000000000000000000000000000000000000",
			},
		})
		s.Require().Error(err, "Should return error when IP is not in whitelist")
		s.ErrorIs(err, result.ErrIPNotAllowed, "Should return IP not allowed error")
		loader.AssertExpectations(s.T())
	})

	s.Run("NoExternalAppConfig", func() {
		loader := new(MockExternalAppLoader)
		nonceStore := new(MockNonceStore)

		principal := security.NewExternalApp("app1", "Test App", "api_user")
		loader.On("LoadByID", mock.Anything, "app1").Return(principal, testSecretHex, nil)
		nonceStore.On("StoreIfAbsent", mock.Anything, "app1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(true, nil)

		auth := NewSignatureAuthenticator(loader, nonceStore)
		credentials := s.generateValidCredentials("app1", testSecretHex)

		got, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypeSignature,
			Principal:   "app1",
			Credentials: credentials,
		})
		s.Require().NoError(err, "Should authenticate when no ExternalAppConfig is set")
		s.Equal("app1", got.ID, "Principal ID should match")
		loader.AssertExpectations(s.T())
		nonceStore.AssertExpectations(s.T())
	})
}

// TestSuccessfulAuthentication verifies successful authentication scenarios.
func (s *SignatureAuthenticatorTestSuite) TestSuccessfulAuthentication() {
	ctx := context.Background()

	s.Run("WithNonceStore", func() {
		loader := new(MockExternalAppLoader)
		nonceStore := new(MockNonceStore)

		principal := security.NewExternalApp("app1", "Test App", "api_user")
		loader.On("LoadByID", mock.Anything, "app1").Return(principal, testSecretHex, nil)
		nonceStore.On("StoreIfAbsent", mock.Anything, "app1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(true, nil)

		auth := NewSignatureAuthenticator(loader, nonceStore)
		credentials := s.generateValidCredentials("app1", testSecretHex)

		got, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypeSignature,
			Principal:   "app1",
			Credentials: credentials,
		})
		s.Require().NoError(err, "Should authenticate successfully")
		s.Equal("app1", got.ID, "Principal ID should match")
		s.Equal("Test App", got.Name, "Principal name should match")
		s.Equal(security.PrincipalTypeExternalApp, got.Type, "Principal type should be external app")
		loader.AssertExpectations(s.T())
		nonceStore.AssertExpectations(s.T())
	})

	s.Run("WithoutNonceStore", func() {
		loader := new(MockExternalAppLoader)

		principal := security.NewExternalApp("app1", "Test App", "api_user")
		loader.On("LoadByID", mock.Anything, "app1").Return(principal, testSecretHex, nil)

		auth := NewSignatureAuthenticator(loader, nil)
		credentials := s.generateValidCredentials("app1", testSecretHex)

		got, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypeSignature,
			Principal:   "app1",
			Credentials: credentials,
		})
		s.Require().NoError(err, "Should authenticate successfully without nonce store")
		s.Equal("app1", got.ID, "Principal ID should match")
		loader.AssertExpectations(s.T())
	})

	s.Run("MultipleApps", func() {
		loader := new(MockExternalAppLoader)
		nonceStore := new(MockNonceStore)

		secret1 := testSecretHex
		secret2 := "bf7786789ce92be8d04d5b62e233ff72fa861ff6e53dfbc2d44c3a4a47cd25d3"

		principal1 := security.NewExternalApp("app1", "App One", "api_user")
		principal2 := security.NewExternalApp("app2", "App Two", "api_admin")

		loader.On("LoadByID", mock.Anything, "app1").Return(principal1, secret1, nil)
		loader.On("LoadByID", mock.Anything, "app2").Return(principal2, secret2, nil)
		nonceStore.On("StoreIfAbsent", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(true, nil)

		auth := NewSignatureAuthenticator(loader, nonceStore)

		creds1 := s.generateValidCredentials("app1", secret1)
		got1, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypeSignature,
			Principal:   "app1",
			Credentials: creds1,
		})
		s.Require().NoError(err, "Should authenticate app1")
		s.Equal("app1", got1.ID, "Should equal expected value")
		s.Equal("App One", got1.Name, "Should equal expected value")

		creds2 := s.generateValidCredentials("app2", secret2)
		got2, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypeSignature,
			Principal:   "app2",
			Credentials: creds2,
		})
		s.Require().NoError(err, "Should authenticate app2")
		s.Equal("app2", got2.ID, "Should equal expected value")
		s.Equal("App Two", got2.Name, "Should equal expected value")

		loader.AssertExpectations(s.T())
	})
}

func TestSignatureAuthenticator(t *testing.T) {
	suite.Run(t, new(SignatureAuthenticatorTestSuite))
}
