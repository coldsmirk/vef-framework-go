package security

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

type PasswordAuthenticatorTestSuite struct {
	suite.Suite
}

// TestSupports verifies type matching.
func (s *PasswordAuthenticatorTestSuite) TestSupports() {
	auth := NewPasswordAuthenticator(nil, nil)
	s.True(auth.Supports(AuthTypePassword), "Should support password type")
	s.False(auth.Supports("token"), "Should not support token type")
	s.False(auth.Supports(""), "Should not support empty type")
}

// TestAuthenticate verifies all authentication paths.
func (s *PasswordAuthenticatorTestSuite) TestAuthenticate() {
	ctx := context.Background()

	s.Run("NilLoader", func() {
		auth := NewPasswordAuthenticator(nil, nil)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypePassword,
			Principal:   "alice",
			Credentials: "password123",
		})
		s.Require().Error(err, "Should return error when loader is nil")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeNotImplemented, resErr.Code, "Should return not implemented code")
	})

	s.Run("EmptyUsername", func() {
		loader := new(MockUserLoader)
		encoder := new(MockPasswordEncoder)
		auth := NewPasswordAuthenticator(loader, encoder)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypePassword,
			Principal:   "",
			Credentials: "password123",
		})
		s.Require().Error(err, "Should return error for empty username")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodePrincipalInvalid, resErr.Code, "Should return principal invalid code")
	})

	s.Run("SystemPrincipalForbidden", func() {
		systemPrincipals := []string{orm.OperatorSystem, orm.OperatorCronJob, orm.OperatorAnonymous}

		for _, principal := range systemPrincipals {
			s.Run(principal, func() {
				loader := new(MockUserLoader)
				encoder := new(MockPasswordEncoder)
				auth := NewPasswordAuthenticator(loader, encoder)

				_, err := auth.Authenticate(ctx, security.Authentication{
					Type:        AuthTypePassword,
					Principal:   principal,
					Credentials: "password123",
				})
				s.Require().Error(err, "Should reject system principal")

				resErr, ok := result.AsErr(err)
				s.Require().True(ok, "Should return a result.Error")
				s.Equal(result.ErrCodePrincipalInvalid, resErr.Code, "Should return principal invalid code")
			})
		}
	})

	s.Run("NilCredentials", func() {
		loader := new(MockUserLoader)
		encoder := new(MockPasswordEncoder)
		auth := NewPasswordAuthenticator(loader, encoder)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypePassword,
			Principal:   "alice",
			Credentials: nil,
		})
		s.Require().Error(err, "Should return error for nil credentials")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeCredentialsInvalid, resErr.Code, "Should return credentials invalid code")
	})

	s.Run("CredentialsNotString", func() {
		loader := new(MockUserLoader)
		encoder := new(MockPasswordEncoder)
		auth := NewPasswordAuthenticator(loader, encoder)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypePassword,
			Principal:   "alice",
			Credentials: 12345,
		})
		s.Require().Error(err, "Should return error for non-string credentials")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeCredentialsInvalid, resErr.Code, "Should return credentials invalid code")
	})

	s.Run("EmptyPassword", func() {
		loader := new(MockUserLoader)
		encoder := new(MockPasswordEncoder)
		auth := NewPasswordAuthenticator(loader, encoder)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypePassword,
			Principal:   "alice",
			Credentials: "",
		})
		s.Require().Error(err, "Should return error for empty password")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeCredentialsInvalid, resErr.Code, "Should return credentials invalid code")
	})

	s.Run("UserNotFound", func() {
		loader := new(MockUserLoader)
		loader.On("LoadByUsername", mock.Anything, "alice").Return(nil, "", result.ErrRecordNotFound)

		encoder := new(MockPasswordEncoder)
		auth := NewPasswordAuthenticator(loader, encoder)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypePassword,
			Principal:   "alice",
			Credentials: "password123",
		})
		s.Require().Error(err, "Should return error when user not found")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeCredentialsInvalid, resErr.Code, "Should return credentials invalid to avoid leaking user existence")
		loader.AssertExpectations(s.T())
	})

	s.Run("LoaderReturnsGenericError", func() {
		loader := new(MockUserLoader)
		loader.On("LoadByUsername", mock.Anything, "alice").Return(nil, "", errors.New("db error"))

		encoder := new(MockPasswordEncoder)
		auth := NewPasswordAuthenticator(loader, encoder)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypePassword,
			Principal:   "alice",
			Credentials: "password123",
		})
		s.Require().Error(err, "Should return error on loader failure")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeCredentialsInvalid, resErr.Code, "Should return credentials invalid to mask internal error")
	})

	s.Run("NilPrincipalFromLoader", func() {
		loader := new(MockUserLoader)
		loader.On("LoadByUsername", mock.Anything, "alice").Return(nil, "hash", nil)

		encoder := new(MockPasswordEncoder)
		auth := NewPasswordAuthenticator(loader, encoder)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypePassword,
			Principal:   "alice",
			Credentials: "password123",
		})
		s.Require().Error(err, "Should return error when principal is nil")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeCredentialsInvalid, resErr.Code, "Should return credentials invalid code")
	})

	s.Run("EmptyPasswordHash", func() {
		loader := new(MockUserLoader)
		principal := security.NewUser("user1", "Alice")
		loader.On("LoadByUsername", mock.Anything, "alice").Return(principal, "", nil)

		encoder := new(MockPasswordEncoder)
		auth := NewPasswordAuthenticator(loader, encoder)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypePassword,
			Principal:   "alice",
			Credentials: "password123",
		})
		s.Require().Error(err, "Should return error when password hash is empty")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeCredentialsInvalid, resErr.Code, "Should return credentials invalid code")
	})

	s.Run("PasswordMismatch", func() {
		loader := new(MockUserLoader)
		principal := security.NewUser("user1", "Alice")
		loader.On("LoadByUsername", mock.Anything, "alice").Return(principal, "$2a$hash", nil)

		encoder := new(MockPasswordEncoder)
		encoder.On("Matches", "wrongpass", "$2a$hash").Return(false)

		auth := NewPasswordAuthenticator(loader, encoder)

		_, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypePassword,
			Principal:   "alice",
			Credentials: "wrongpass",
		})
		s.Require().Error(err, "Should return error on password mismatch")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeCredentialsInvalid, resErr.Code, "Should return credentials invalid code")
		encoder.AssertExpectations(s.T())
	})

	s.Run("SuccessfulAuthentication", func() {
		principal := security.NewUser("user1", "Alice", "admin")
		loader := new(MockUserLoader)
		loader.On("LoadByUsername", mock.Anything, "alice").Return(principal, "$2a$hash", nil)

		encoder := new(MockPasswordEncoder)
		encoder.On("Matches", "correct", "$2a$hash").Return(true)

		auth := NewPasswordAuthenticator(loader, encoder)

		got, err := auth.Authenticate(ctx, security.Authentication{
			Type:        AuthTypePassword,
			Principal:   "alice",
			Credentials: "correct",
		})
		s.Require().NoError(err, "Should authenticate successfully")
		s.Equal("user1", got.ID, "Should return expected principal ID")
		s.Equal("Alice", got.Name, "Should return expected principal name")
		s.Equal([]string{"admin"}, got.Roles, "Should return expected roles")
		loader.AssertExpectations(s.T())
		encoder.AssertExpectations(s.T())
	})
}

func TestPasswordAuthenticator(t *testing.T) {
	suite.Run(t, new(PasswordAuthenticatorTestSuite))
}
