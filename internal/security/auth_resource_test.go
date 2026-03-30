package security_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	isecurity "github.com/coldsmirk/vef-framework-go/internal/security"
	"github.com/coldsmirk/vef-framework-go/password"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

const testJWTSecret = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// MockUserLoader is a mock implementation of security.UserLoader for testing.
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

// MockUserInfoLoader is a mock implementation of security.UserInfoLoader for testing.
type MockUserInfoLoader struct {
	mock.Mock
}

func (m *MockUserInfoLoader) LoadUserInfo(ctx context.Context, principal *security.Principal, params map[string]any) (*security.UserInfo, error) {
	args := m.Called(ctx, principal, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*security.UserInfo), args.Error(1)
}

// MockPublisher is a mock implementation of event.Publisher for testing.
type MockPublisher struct {
	mock.Mock

	publishedEvents []event.Event
}

func (m *MockPublisher) Publish(evt event.Event) {
	m.Called(evt)
	m.publishedEvents = append(m.publishedEvents, evt)
}

func (m *MockPublisher) GetPublishedEvents() []event.Event {
	return m.publishedEvents
}

func (m *MockPublisher) ClearPublishedEvents() {
	m.publishedEvents = nil
}

// AuthResourceTestSuite is the test suite for AuthResource.
type AuthResourceTestSuite struct {
	apptest.Suite

	userLoader     *MockUserLoader
	userInfoLoader *MockUserInfoLoader
	publisher      *MockPublisher
	testUser       *security.Principal
}

func (suite *AuthResourceTestSuite) SetupSuite() {
	suite.testUser = security.NewUser("user001", "Test User", "admin", "user")
	suite.testUser.Details = map[string]any{
		"email":  "test@example.com",
		"phone":  "1234567890",
		"status": "active",
	}

	suite.userLoader = new(MockUserLoader)
	suite.userInfoLoader = new(MockUserInfoLoader)
	suite.publisher = new(MockPublisher)

	suite.setupTestApp()
}

func (suite *AuthResourceTestSuite) TearDownSuite() {
	suite.TearDownApp()
}

func (suite *AuthResourceTestSuite) SetupTest() {
	suite.userLoader.Calls = nil
	suite.userInfoLoader.Calls = nil
	suite.publisher.Calls = nil
	suite.publisher.ClearPublishedEvents()
}

func (suite *AuthResourceTestSuite) setupTestApp() {
	// Hash the password for test user
	hashedPassword, err := password.NewBcryptEncoder().Encode("password123")
	suite.Require().NoError(err, "Should not return error")

	suite.SetupApp(
		fx.Supply(
			fx.Annotate(
				suite.userLoader,
				fx.As(new(security.UserLoader)),
			),
		),
		fx.Supply(
			fx.Annotate(
				suite.userInfoLoader,
				fx.As(new(security.UserInfoLoader)),
			),
		),
		fx.Replace(
			fx.Annotate(
				suite.publisher,
				fx.As(new(event.Publisher)),
			),
		),
		fx.Replace(
			&config.DataSourceConfig{
				Kind: "sqlite",
			},
			&config.SecurityConfig{
				TokenExpires:     24 * time.Hour,
				RefreshNotBefore: 1 * time.Millisecond,
				LoginRateLimit:   1000,
				RefreshRateLimit: 1000,
			},
			&security.JWTConfig{
				Secret:   testJWTSecret,
				Audience: "test-app",
			},
		),
		fx.Invoke(func() {
			suite.userLoader.On("LoadByUsername", mock.Anything, "testuser").
				Return(suite.testUser, hashedPassword, nil).
				Maybe()

			suite.userLoader.On("LoadByID", mock.Anything, "user001").
				Return(suite.testUser, nil).
				Maybe()

			suite.userLoader.On("LoadByUsername", mock.Anything, "nonexistent").
				Return(nil, "", nil).
				Maybe()

			suite.userLoader.On("LoadByID", mock.Anything, "nonexistent").
				Return(nil, nil).
				Maybe()

			suite.publisher.On("Publish", mock.Anything).
				Maybe()
		}),
	)
}

// extractTokensFromLoginResult extracts the tokens map from a LoginResult response.
func (suite *AuthResourceTestSuite) extractTokensFromLoginResult(data map[string]any) map[string]any {
	suite.T().Helper()

	tokensRaw, ok := data["tokens"]
	suite.True(ok, "LoginResult should contain tokens field")
	suite.NotNil(tokensRaw, "Tokens should not be nil")

	tokens, ok := tokensRaw.(map[string]any)
	suite.True(ok, "Tokens should be a map")

	return tokens
}

func (suite *AuthResourceTestSuite) TestLoginSuccess() {
	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "login",
			Version:  "v1",
		},
		Params: map[string]any{
			"type":        isecurity.AuthTypePassword,
			"principal":   "testuser",
			"credentials": "password123",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Login should succeed")
	suite.Equal(i18n.T(result.OkMessage), body.Message, "Should return success message")

	data := suite.ReadDataAsMap(body.Data)
	suite.Nil(data["challengeToken"], "No challenge token when no challenges")
	suite.Nil(data["challenge"], "No challenge when no challenge providers")

	tokens := suite.extractTokensFromLoginResult(data)
	suite.NotEmpty(tokens["accessToken"], "Access token should not be empty")
	suite.NotEmpty(tokens["refreshToken"], "Refresh token should not be empty")

	suite.userLoader.AssertCalled(suite.T(), "LoadByUsername", mock.Anything, "testuser")
}

// TestLoginInvalidCredentials tests login failures with invalid credentials.
func (suite *AuthResourceTestSuite) TestLoginInvalidCredentials() {
	suite.Run("WrongPassword", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "security/auth",
				Action:   "login",
				Version:  "v1",
			},
			Params: map[string]any{
				"type":        isecurity.AuthTypePassword,
				"principal":   "testuser",
				"credentials": "wrongpassword",
			},
		})

		suite.Equal(401, resp.StatusCode, "Should return 401 Unauthorized")

		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Login should fail with wrong password")
		suite.Equal(result.ErrCodeCredentialsInvalid, body.Code, "Should return credentials invalid error")
	})

	suite.Run("UserNotFound", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "security/auth",
				Action:   "login",
				Version:  "v1",
			},
			Params: map[string]any{
				"type":        isecurity.AuthTypePassword,
				"principal":   "nonexistent",
				"credentials": "password123",
			},
		})

		suite.Equal(401, resp.StatusCode, "Should return 401 Unauthorized")

		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Login should fail with non-existent user")
		suite.Equal(result.ErrCodeCredentialsInvalid, body.Code, "Should return credentials invalid error")
	})
}

// TestLoginMissingParameters tests login failures with missing or invalid parameters.
func (suite *AuthResourceTestSuite) TestLoginMissingParameters() {
	suite.Run("MissingUsername", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "security/auth",
				Action:   "login",
				Version:  "v1",
			},
			Params: map[string]any{
				"type":        isecurity.AuthTypePassword,
				"credentials": "password123",
			},
		})

		suite.Equal(400, resp.StatusCode, "Should return 400 Bad Request")

		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Login should fail without username")
		suite.Equal(result.ErrCodeBadRequest, body.Code, "Should return bad request error")
	})

	suite.Run("MissingPassword", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "security/auth",
				Action:   "login",
				Version:  "v1",
			},
			Params: map[string]any{
				"type":      isecurity.AuthTypePassword,
				"principal": "testuser",
			},
		})

		suite.Equal(400, resp.StatusCode, "Should return 400 Bad Request")

		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Login should fail without password")
		suite.Equal(result.ErrCodeBadRequest, body.Code, "Should return bad request error")
	})

	suite.Run("EmptyPassword", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "security/auth",
				Action:   "login",
				Version:  "v1",
			},
			Params: map[string]any{
				"type":        isecurity.AuthTypePassword,
				"principal":   "testuser",
				"credentials": "",
			},
		})

		suite.Equal(401, resp.StatusCode, "Should return 401 Unauthorized")

		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Login should fail with empty password")
		suite.Equal(result.ErrCodeCredentialsInvalid, body.Code, "Should return credentials invalid error")
	})

	suite.Run("LoaderRecordNotFoundError", func() {
		username := "loaderNotFound"
		suite.userLoader.On("LoadByUsername", mock.Anything, username).
			Return((*security.Principal)(nil), "", result.ErrRecordNotFound).
			Once()

		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "security/auth",
				Action:   "login",
				Version:  "v1",
			},
			Params: map[string]any{
				"type":        isecurity.AuthTypePassword,
				"principal":   username,
				"credentials": "password123",
			},
		})

		suite.Equal(401, resp.StatusCode, "Should return 401 Unauthorized")

		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Login should fail when loader reports record not found")
		suite.Equal(result.ErrCodeCredentialsInvalid, body.Code, "Should return credentials invalid error")
		suite.userLoader.AssertExpectations(suite.T())
	})

	suite.Run("LoaderUnexpectedError", func() {
		username := "loaderUnexpected"
		suite.userLoader.On("LoadByUsername", mock.Anything, username).
			Return((*security.Principal)(nil), "", errors.New("loader failure")).
			Once()

		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "security/auth",
				Action:   "login",
				Version:  "v1",
			},
			Params: map[string]any{
				"type":        isecurity.AuthTypePassword,
				"principal":   username,
				"credentials": "password123",
			},
		})

		suite.Equal(401, resp.StatusCode, "Should return 401 Unauthorized")

		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Login should fail when loader returns unexpected error")
		suite.Equal(result.ErrCodeCredentialsInvalid, body.Code, "Should return credentials invalid error")
		suite.userLoader.AssertExpectations(suite.T())
	})
}

func (suite *AuthResourceTestSuite) TestRefreshSuccess() {
	loginResp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "login",
			Version:  "v1",
		},
		Params: map[string]any{
			"type":        isecurity.AuthTypePassword,
			"principal":   "testuser",
			"credentials": "password123",
		},
	})

	loginBody := suite.ReadResult(loginResp)
	suite.True(loginBody.IsOk(), "Login should succeed")

	loginData := suite.ReadDataAsMap(loginBody.Data)
	tokens := suite.extractTokensFromLoginResult(loginData)
	refreshToken := tokens["refreshToken"].(string)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "refresh",
			Version:  "v1",
		},
		Params: map[string]any{
			"refreshToken": refreshToken,
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Refresh should succeed")
	suite.Equal(i18n.T(result.OkMessage), body.Message, "Should return success message")

	data := suite.ReadDataAsMap(body.Data)
	suite.Contains(data, "accessToken", "Response should contain access token")
	suite.Contains(data, "refreshToken", "Response should contain refresh token")
	suite.NotEmpty(data["accessToken"], "Access token should not be empty")
	suite.NotEmpty(data["refreshToken"], "Refresh token should not be empty")

	suite.NotEqual(tokens["accessToken"], data["accessToken"], "New access token should be different")

	suite.userLoader.AssertCalled(suite.T(), "LoadByID", mock.Anything, "user001")
}

// TestRefreshInvalidToken tests refresh failures with invalid tokens.
func (suite *AuthResourceTestSuite) TestRefreshInvalidToken() {
	suite.Run("InvalidToken", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "security/auth",
				Action:   "refresh",
				Version:  "v1",
			},
			Params: map[string]any{
				"refreshToken": "invalid.token.here",
			},
		})

		suite.True(resp.StatusCode == 200 || resp.StatusCode == 401,
			"Should return 200 or 401, got %d", resp.StatusCode)

		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Refresh should fail with invalid token")
		suite.Equal(result.ErrCodeTokenInvalid, body.Code, "Should return token invalid error")
	})

	suite.Run("EmptyToken", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "security/auth",
				Action:   "refresh",
				Version:  "v1",
			},
			Params: map[string]any{
				"refreshToken": "",
			},
		})

		suite.Equal(400, resp.StatusCode, "Should return 400 Bad Request")

		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Refresh should fail with empty token")
		suite.Equal(result.ErrCodeBadRequest, body.Code, "Should return bad request error")
	})

	suite.Run("MissingToken", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "security/auth",
				Action:   "refresh",
				Version:  "v1",
			},
			Params: map[string]any{},
		})

		suite.Equal(400, resp.StatusCode, "Should return 400 Bad Request")

		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Refresh should fail without token")
		suite.Equal(result.ErrCodeBadRequest, body.Code, "Should return bad request error")
	})
}

// TestRefreshWithAccessToken tests that refresh fails when using an access token instead of refresh token.
func (suite *AuthResourceTestSuite) TestRefreshWithAccessToken() {
	loginResp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "login",
			Version:  "v1",
		},
		Params: map[string]any{
			"type":        isecurity.AuthTypePassword,
			"principal":   "testuser",
			"credentials": "password123",
		},
	})

	loginBody := suite.ReadResult(loginResp)
	suite.True(loginBody.IsOk(), "Login should succeed")

	loginData := suite.ReadDataAsMap(loginBody.Data)
	tokens := suite.extractTokensFromLoginResult(loginData)
	accessToken := tokens["accessToken"].(string)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "refresh",
			Version:  "v1",
		},
		Params: map[string]any{
			"refreshToken": accessToken,
		},
	})

	suite.Equal(401, resp.StatusCode, "Should return 401 Unauthorized")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Refresh should fail with access token")
	suite.Equal(result.ErrCodeTokenInvalid, body.Code, "Should return token invalid error")
}

// TestRefreshUserNotFound tests refresh failure when user is not found.
func (suite *AuthResourceTestSuite) TestRefreshUserNotFound() {
	loginResp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "login",
			Version:  "v1",
		},
		Params: map[string]any{
			"type":        isecurity.AuthTypePassword,
			"principal":   "testuser",
			"credentials": "password123",
		},
	})

	loginBody := suite.ReadResult(loginResp)
	suite.True(loginBody.IsOk(), "Login should succeed")

	loginData := suite.ReadDataAsMap(loginBody.Data)
	tokens := suite.extractTokensFromLoginResult(loginData)
	refreshToken := tokens["refreshToken"].(string)

	prevExpected := append([]*mock.Call(nil), suite.userLoader.ExpectedCalls...)
	defer func() { suite.userLoader.ExpectedCalls = prevExpected }()

	call := suite.userLoader.On("LoadByID", mock.Anything, mock.Anything).Return((*security.Principal)(nil), nil).Once()
	if n := len(suite.userLoader.ExpectedCalls); n > 1 {
		last := suite.userLoader.ExpectedCalls[n-1]
		suite.userLoader.ExpectedCalls = append([]*mock.Call{last}, suite.userLoader.ExpectedCalls[:n-1]...)
		_ = call
	}

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "refresh",
			Version:  "v1",
		},
		Params: map[string]any{
			"refreshToken": refreshToken,
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Refresh should fail when user not found")
	suite.Equal(result.ErrCodeRecordNotFound, body.Code, "Should return record not found error")
}

func (suite *AuthResourceTestSuite) TestLogoutSuccess() {
	loginResp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "login",
			Version:  "v1",
		},
		Params: map[string]any{
			"type":        isecurity.AuthTypePassword,
			"principal":   "testuser",
			"credentials": "password123",
		},
	})

	loginBody := suite.ReadResult(loginResp)
	suite.True(loginBody.IsOk(), "Login should succeed")

	loginData := suite.ReadDataAsMap(loginBody.Data)
	tokens := suite.extractTokensFromLoginResult(loginData)
	accessToken := tokens["accessToken"].(string)

	resp := suite.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "logout",
			Version:  "v1",
		},
	}, accessToken)

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Logout should succeed")
	suite.Equal(i18n.T(result.OkMessage), body.Message, "Should return success message")
}

// TestLoginAndRefreshFlow tests the complete login and refresh flow.
func (suite *AuthResourceTestSuite) TestLoginAndRefreshFlow() {
	loginResp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "login",
			Version:  "v1",
		},
		Params: map[string]any{
			"type":        isecurity.AuthTypePassword,
			"principal":   "testuser",
			"credentials": "password123",
		},
	})

	loginBody := suite.ReadResult(loginResp)
	suite.True(loginBody.IsOk(), "Login should succeed")

	loginData := suite.ReadDataAsMap(loginBody.Data)
	tokens1 := suite.extractTokensFromLoginResult(loginData)

	refreshResp1 := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "refresh",
			Version:  "v1",
		},
		Params: map[string]any{
			"refreshToken": tokens1["refreshToken"],
		},
	})

	refreshBody1 := suite.ReadResult(refreshResp1)
	suite.True(refreshBody1.IsOk(), "First refresh should succeed")

	tokens2 := suite.ReadDataAsMap(refreshBody1.Data)

	suite.NotEqual(tokens1["accessToken"], tokens2["accessToken"], "New access token should be different")
	suite.NotEqual(tokens1["refreshToken"], tokens2["refreshToken"], "New refresh token should be different")

	refreshResp2 := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "refresh",
			Version:  "v1",
		},
		Params: map[string]any{
			"refreshToken": tokens2["refreshToken"],
		},
	})

	refreshBody2 := suite.ReadResult(refreshResp2)
	suite.True(refreshBody2.IsOk(), "Second refresh should succeed")

	tokens3 := suite.ReadDataAsMap(refreshBody2.Data)

	suite.NotEqual(tokens2["accessToken"], tokens3["accessToken"], "Tokens should keep changing")
	suite.NotEqual(tokens2["refreshToken"], tokens3["refreshToken"], "Tokens should keep changing")

	logoutResp := suite.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "logout",
			Version:  "v1",
		},
	}, tokens3["accessToken"].(string))

	logoutBody := suite.ReadResult(logoutResp)
	suite.True(logoutBody.IsOk(), "Logout should succeed")
}

func (suite *AuthResourceTestSuite) TestTokenDetails() {
	loginResp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "login",
			Version:  "v1",
		},
		Params: map[string]any{
			"type":        isecurity.AuthTypePassword,
			"principal":   "testuser",
			"credentials": "password123",
		},
	})

	loginBody := suite.ReadResult(loginResp)
	suite.True(loginBody.IsOk(), "Login should succeed")

	loginData := suite.ReadDataAsMap(loginBody.Data)
	tokens := suite.extractTokensFromLoginResult(loginData)
	accessToken := tokens["accessToken"].(string)
	refreshToken := tokens["refreshToken"].(string)

	suite.NotEmpty(accessToken, "Access token should not be empty")
	suite.NotEmpty(refreshToken, "Refresh token should not be empty")

	suite.NotEqual(accessToken, refreshToken, "Tokens should be different")

	suite.Equal(3, len(strings.Split(accessToken, ".")), "Access token should be JWT format (3 parts)")
	suite.Equal(3, len(strings.Split(refreshToken, ".")), "Refresh token should be JWT format (3 parts)")
}

func (suite *AuthResourceTestSuite) TestGetUserInfoSuccess() {
	loginResp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "login",
			Version:  "v1",
		},
		Params: map[string]any{
			"type":        isecurity.AuthTypePassword,
			"principal":   "testuser",
			"credentials": "password123",
		},
	})

	loginBody := suite.ReadResult(loginResp)
	suite.True(loginBody.IsOk(), "Login should succeed")

	loginData := suite.ReadDataAsMap(loginBody.Data)
	tokens := suite.extractTokensFromLoginResult(loginData)
	accessToken := tokens["accessToken"].(string)

	avatarURL := "https://example.com/avatar.jpg"
	expectedUserInfo := &security.UserInfo{
		ID:     "user001",
		Name:   "Test User",
		Gender: security.GenderMale,
		Avatar: &avatarURL,
		PermTokens: []string{
			"user:read",
			"user:write",
			"order:read",
		},
		Menus: []security.UserMenu{
			{
				Type: security.UserMenuTypeDirectory,
				Path: "/system",
				Name: "System Management",
				Icon: new("setting"),
				Children: []security.UserMenu{
					{
						Type: security.UserMenuTypeMenu,
						Path: "/system/users",
						Name: "User Management",
						Icon: new("user"),
					},
				},
			},
		},
	}

	suite.userInfoLoader.On("LoadUserInfo", mock.Anything, mock.MatchedBy(func(p *security.Principal) bool {
		return p.ID == "user001"
	}), mock.Anything).Return(expectedUserInfo, nil).Once()

	resp := suite.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "get_user_info",
			Version:  "v1",
		},
	}, accessToken)

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Get user info should succeed")
	suite.Equal(i18n.T(result.OkMessage), body.Message, "Should return success message")

	data := suite.ReadDataAsMap(body.Data)
	suite.Equal("user001", data["id"], "User ID should match")
	suite.Equal("Test User", data["name"], "User name should match")
	suite.Equal("male", data["gender"], "Gender should match")
	suite.Equal(avatarURL, data["avatar"], "Avatar URL should match")

	permTokens, ok := data["permTokens"].([]any)
	suite.True(ok, "Permission tokens should be an array")
	suite.Len(permTokens, 3, "Should have 3 permission tokens")
	suite.Contains(permTokens, "user:read", "Should contain user:read permission")
	suite.Contains(permTokens, "user:write", "Should contain user:write permission")
	suite.Contains(permTokens, "order:read", "Should contain order:read permission")

	menus, ok := data["menus"].([]any)
	suite.True(ok, "Menus should be an array")
	suite.Len(menus, 1, "Should have 1 menu")

	firstMenu := menus[0].(map[string]any)
	suite.Equal("directory", firstMenu["type"], "Menu type should be directory")
	suite.Equal("/system", firstMenu["path"], "Menu path should match")
	suite.Equal("System Management", firstMenu["name"], "Menu name should match")
	suite.Equal("setting", firstMenu["icon"], "Menu icon should match")

	children, ok := firstMenu["children"].([]any)
	suite.True(ok, "Children should be an array")
	suite.Len(children, 1, "Should have 1 child menu")

	suite.userInfoLoader.AssertExpectations(suite.T())
}

func (suite *AuthResourceTestSuite) TestGetUserInfoUnauthenticated() {
	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "get_user_info",
			Version:  "v1",
		},
	})

	suite.Equal(401, resp.StatusCode, "Should return 401 Unauthorized")
}

func (suite *AuthResourceTestSuite) TestGetUserInfoLoaderError() {
	loginResp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "login",
			Version:  "v1",
		},
		Params: map[string]any{
			"type":        isecurity.AuthTypePassword,
			"principal":   "testuser",
			"credentials": "password123",
		},
	})

	loginBody := suite.ReadResult(loginResp)
	suite.True(loginBody.IsOk(), "Login should succeed")

	loginData := suite.ReadDataAsMap(loginBody.Data)
	tokens := suite.extractTokensFromLoginResult(loginData)
	accessToken := tokens["accessToken"].(string)

	suite.userInfoLoader.On("LoadUserInfo", mock.Anything, mock.MatchedBy(func(p *security.Principal) bool {
		return p.ID == "user001"
	}), mock.Anything).Return((*security.UserInfo)(nil), errors.New("database connection failed")).Once()

	resp := suite.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "get_user_info",
			Version:  "v1",
		},
	}, accessToken)

	suite.Equal(500, resp.StatusCode, "Should return 500 Internal Server Error")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Get user info should fail when loader returns error")

	suite.userInfoLoader.AssertExpectations(suite.T())
}

func (suite *AuthResourceTestSuite) TestGetUserInfoWithEmptyMenus() {
	loginResp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "login",
			Version:  "v1",
		},
		Params: map[string]any{
			"type":        isecurity.AuthTypePassword,
			"principal":   "testuser",
			"credentials": "password123",
		},
	})

	loginBody := suite.ReadResult(loginResp)
	suite.True(loginBody.IsOk(), "Login should succeed")

	loginData := suite.ReadDataAsMap(loginBody.Data)
	tokens := suite.extractTokensFromLoginResult(loginData)
	accessToken := tokens["accessToken"].(string)

	expectedUserInfo := &security.UserInfo{
		ID:         "user001",
		Name:       "Test User",
		Gender:     security.GenderUnknown,
		PermTokens: []string{},
		Menus:      []security.UserMenu{},
	}

	suite.userInfoLoader.On("LoadUserInfo", mock.Anything, mock.MatchedBy(func(p *security.Principal) bool {
		return p.ID == "user001"
	}), mock.Anything).Return(expectedUserInfo, nil).Once()

	resp := suite.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "get_user_info",
			Version:  "v1",
		},
	}, accessToken)

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Get user info should succeed")

	data := suite.ReadDataAsMap(body.Data)
	suite.Equal("user001", data["id"], "User ID should match")
	suite.Equal("Test User", data["name"], "User name should match")
	suite.Equal("unknown", data["gender"], "Gender should be unknown")
	suite.Nil(data["avatar"], "Avatar should be null when not set")

	permTokens, ok := data["permTokens"].([]any)
	suite.True(ok, "Permission tokens should be an array")
	suite.Len(permTokens, 0, "Permission tokens should be empty")

	menus, ok := data["menus"].([]any)
	suite.True(ok, "Menus should be an array")
	suite.Len(menus, 0, "Menus should be empty")

	suite.userInfoLoader.AssertExpectations(suite.T())
}

func (suite *AuthResourceTestSuite) TestLoginEventPublished() {
	suite.Run("LoginSuccessEvent", func() {
		suite.publisher.ClearPublishedEvents()

		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "security/auth",
				Action:   "login",
				Version:  "v1",
			},
			Params: map[string]any{
				"type":        isecurity.AuthTypePassword,
				"principal":   "testuser",
				"credentials": "password123",
			},
		})

		suite.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Login should succeed")

		events := suite.publisher.GetPublishedEvents()
		suite.Len(events, 1, "Should publish exactly one event")

		loginEvent, ok := events[0].(*security.LoginEvent)
		suite.True(ok, "Event should be LoginEvent type")
		suite.NotNil(loginEvent, "Login event should not be nil")

		suite.Equal("password", loginEvent.AuthType, "AuthType should be password")
		suite.Require().NotNil(loginEvent.UserID, "UserID should not be nil for successful login")
		suite.Equal("user001", *loginEvent.UserID, "UserID should match")
		suite.Equal("testuser", loginEvent.Username, "Username should match")
		suite.True(loginEvent.IsOk, "IsOk should be true for successful login")
		suite.Empty(loginEvent.FailReason, "FailReason should be empty for successful login")
		suite.Equal(0, loginEvent.ErrorCode, "ErrorCode should be 0 for successful login")
		suite.NotEmpty(loginEvent.LoginIP, "LoginIP should not be empty")
		suite.NotEmpty(loginEvent.TraceID, "TraceID should not be empty")
	})

	suite.Run("LoginFailureEvent", func() {
		suite.publisher.ClearPublishedEvents()

		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "security/auth",
				Action:   "login",
				Version:  "v1",
			},
			Params: map[string]any{
				"type":        isecurity.AuthTypePassword,
				"principal":   "testuser",
				"credentials": "wrongpassword",
			},
		})

		suite.Equal(401, resp.StatusCode, "Should return 401 Unauthorized")

		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Login should fail")

		events := suite.publisher.GetPublishedEvents()
		suite.Len(events, 1, "Should publish exactly one event")

		loginEvent, ok := events[0].(*security.LoginEvent)
		suite.True(ok, "Event should be LoginEvent type")
		suite.NotNil(loginEvent, "Login event should not be nil")

		suite.Equal("password", loginEvent.AuthType, "AuthType should be password")
		suite.Nil(loginEvent.UserID, "UserID should be nil for failed login")
		suite.Equal("testuser", loginEvent.Username, "Username should match")
		suite.False(loginEvent.IsOk, "IsOk should be false for failed login")
		suite.NotEmpty(loginEvent.FailReason, "FailReason should not be empty for failed login")
		suite.Equal(result.ErrCodeCredentialsInvalid, loginEvent.ErrorCode, "ErrorCode should match")
		suite.NotEmpty(loginEvent.LoginIP, "LoginIP should not be empty")
		suite.NotEmpty(loginEvent.TraceID, "TraceID should not be empty")
	})

	suite.Run("UserNotFoundEvent", func() {
		suite.publisher.ClearPublishedEvents()

		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "security/auth",
				Action:   "login",
				Version:  "v1",
			},
			Params: map[string]any{
				"type":        isecurity.AuthTypePassword,
				"principal":   "nonexistent",
				"credentials": "password123",
			},
		})

		suite.Equal(401, resp.StatusCode, "Should return 401 Unauthorized")

		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Login should fail")

		events := suite.publisher.GetPublishedEvents()
		suite.Len(events, 1, "Should publish exactly one event")

		loginEvent, ok := events[0].(*security.LoginEvent)
		suite.True(ok, "Event should be LoginEvent type")

		suite.Equal("password", loginEvent.AuthType, "AuthType should be password")
		suite.Nil(loginEvent.UserID, "UserID should be nil for non-existent user")
		suite.Equal("nonexistent", loginEvent.Username, "Username should match")
		suite.False(loginEvent.IsOk, "IsOk should be false")
		suite.Equal(result.ErrCodeCredentialsInvalid, loginEvent.ErrorCode, "ErrorCode should match")
	})
}

func TestAuthResource(t *testing.T) {
	suite.Run(t, new(AuthResourceTestSuite))
}

// --- Challenge Flow Test Suite ---

// MockChallengeProvider is a mock implementation of security.ChallengeProvider for testing.
type MockChallengeProvider struct {
	mock.Mock
}

func (m *MockChallengeProvider) Type() string {
	args := m.Called()

	return args.String(0)
}

func (m *MockChallengeProvider) Order() int {
	args := m.Called()

	return args.Int(0)
}

func (m *MockChallengeProvider) Evaluate(ctx context.Context, principal *security.Principal) (*security.LoginChallenge, error) {
	args := m.Called(ctx, principal)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*security.LoginChallenge), args.Error(1)
}

func (m *MockChallengeProvider) Resolve(ctx context.Context, principal *security.Principal, response any) (*security.Principal, error) {
	args := m.Called(ctx, principal, response)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*security.Principal), args.Error(1)
}

// ChallengeFlowTestSuite tests the login challenge flow.
type ChallengeFlowTestSuite struct {
	apptest.Suite

	userLoader        *MockUserLoader
	publisher         *MockPublisher
	challengeProvider *MockChallengeProvider
	testUser          *security.Principal
}

func (s *ChallengeFlowTestSuite) SetupSuite() {
	s.testUser = security.NewUser("user001", "Test User", "admin")
	s.testUser.Details = map[string]any{
		"email": "test@example.com",
	}

	s.userLoader = new(MockUserLoader)
	s.publisher = new(MockPublisher)
	s.challengeProvider = new(MockChallengeProvider)
	s.challengeProvider.On("Type").Return("totp")
	s.challengeProvider.On("Order").Return(0).Maybe()

	hashedPassword, err := password.NewBcryptEncoder().Encode("password123")
	s.Require().NoError(err, "Should not return error")

	s.SetupApp(
		fx.Supply(
			fx.Annotate(
				s.userLoader,
				fx.As(new(security.UserLoader)),
			),
		),
		fx.Supply(
			fx.Annotate(
				s.challengeProvider,
				fx.As(new(security.ChallengeProvider)),
				fx.ResultTags(`group:"vef:security:challenge_providers"`),
			),
		),
		fx.Replace(
			fx.Annotate(
				s.publisher,
				fx.As(new(event.Publisher)),
			),
		),
		fx.Replace(
			&config.DataSourceConfig{
				Kind: "sqlite",
			},
			&config.SecurityConfig{
				TokenExpires:     24 * time.Hour,
				RefreshNotBefore: 1 * time.Millisecond,
				LoginRateLimit:   1000,
				RefreshRateLimit: 1000,
			},
			&security.JWTConfig{
				Secret:   testJWTSecret,
				Audience: "test-app",
			},
		),
		fx.Invoke(func() {
			s.userLoader.On("LoadByUsername", mock.Anything, "testuser").
				Return(s.testUser, hashedPassword, nil).
				Maybe()

			s.userLoader.On("LoadByID", mock.Anything, "user001").
				Return(s.testUser, nil).
				Maybe()

			s.publisher.On("Publish", mock.Anything).Maybe()
		}),
	)
}

func (s *ChallengeFlowTestSuite) TearDownSuite() {
	s.TearDownApp()
}

func (s *ChallengeFlowTestSuite) SetupTest() {
	s.userLoader.Calls = nil
	s.publisher.Calls = nil
	s.publisher.ClearPublishedEvents()
	s.challengeProvider.Calls = nil
}

// loginAndGetResult performs a login and returns the parsed data map.
func (s *ChallengeFlowTestSuite) loginAndGetResult() map[string]any {
	s.T().Helper()

	resp := s.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "login",
			Version:  "v1",
		},
		Params: map[string]any{
			"type":        isecurity.AuthTypePassword,
			"principal":   "testuser",
			"credentials": "password123",
		},
	})

	s.Equal(200, resp.StatusCode, "Should match expected value")

	body := s.ReadResult(resp)
	s.True(body.IsOk(), "Condition should be true")

	return s.ReadDataAsMap(body.Data)
}

// TestLoginWithChallenge tests that login returns a challenge when a provider is active.
func (s *ChallengeFlowTestSuite) TestLoginWithChallenge() {
	s.challengeProvider.On("Type").Return("totp").Maybe()
	s.challengeProvider.On("Evaluate", mock.Anything, mock.Anything).
		Return(&security.LoginChallenge{
			Type:     "totp",
			Required: true,
		}, nil).Once()

	data := s.loginAndGetResult()

	s.NotEmpty(data["challengeToken"], "Should return a challenge token")
	s.Nil(data["tokens"], "Should not return tokens when challenges are pending")

	challenge, ok := data["challenge"].(map[string]any)
	s.Require().True(ok, "Challenge should be an object")
	s.Equal("totp", challenge["type"], "Should match expected value")
	s.Equal(true, challenge["required"], "Should match expected value")

	s.challengeProvider.AssertExpectations(s.T())
}

// TestLoginNoChallengeWhenEvaluateReturnsNil tests that login returns tokens
// when the challenge provider evaluates to nil (challenge not needed).
func (s *ChallengeFlowTestSuite) TestLoginNoChallengeWhenEvaluateReturnsNil() {
	s.challengeProvider.On("Type").Return("totp").Maybe()
	s.challengeProvider.On("Evaluate", mock.Anything, mock.Anything).
		Return((*security.LoginChallenge)(nil), nil).Once()

	data := s.loginAndGetResult()

	s.Nil(data["challengeToken"], "No challenge token when challenge not needed")
	s.Nil(data["challenge"], "No challenge when challenge not needed")

	tokensRaw, ok := data["tokens"]
	s.Require().True(ok, "Should have tokens")
	s.NotNil(tokensRaw, "Should not be nil")

	tokens := tokensRaw.(map[string]any)
	s.NotEmpty(tokens["accessToken"], "Should not be empty")
	s.NotEmpty(tokens["refreshToken"], "Should not be empty")
}

// TestResolveChallengeSuccess tests the full login→challenge→resolve→tokens flow.
func (s *ChallengeFlowTestSuite) TestResolveChallengeSuccess() {
	// Step 1: Login returns challenge
	s.challengeProvider.On("Type").Return("totp").Maybe()
	s.challengeProvider.On("Evaluate", mock.Anything, mock.Anything).
		Return(&security.LoginChallenge{
			Type:     "totp",
			Required: true,
		}, nil).Once()

	data := s.loginAndGetResult()
	challengeToken := data["challengeToken"].(string)

	// Step 2: Resolve challenge → get tokens
	s.challengeProvider.On("Resolve", mock.Anything, mock.Anything, "123456").
		Return(s.testUser, nil).Once()

	resp := s.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "resolve_challenge",
			Version:  "v1",
		},
		Params: map[string]any{
			"challengeToken": challengeToken,
			"type":           "totp",
			"response":       "123456",
		},
	})

	s.Equal(200, resp.StatusCode, "Should match expected value")

	body := s.ReadResult(resp)
	s.True(body.IsOk(), "Condition should be true")

	resolveData := s.ReadDataAsMap(body.Data)
	s.Nil(resolveData["challengeToken"], "No challenge token when all resolved")

	tokensRaw, ok := resolveData["tokens"]
	s.Require().True(ok, "Should have tokens after resolving all challenges")
	s.NotNil(tokensRaw, "Should not be nil")

	tokens := tokensRaw.(map[string]any)
	s.NotEmpty(tokens["accessToken"], "Should not be empty")
	s.NotEmpty(tokens["refreshToken"], "Should not be empty")

	s.challengeProvider.AssertExpectations(s.T())
}

// TestResolveChallengeEmptyToken tests that resolve_challenge rejects empty tokens.
func (s *ChallengeFlowTestSuite) TestResolveChallengeEmptyToken() {
	s.challengeProvider.On("Type").Return("totp").Maybe()

	resp := s.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "resolve_challenge",
			Version:  "v1",
		},
		Params: map[string]any{
			"challengeToken": "",
			"type":           "totp",
			"response":       "123456",
		},
	})

	s.Equal(400, resp.StatusCode, "Should match expected value")

	body := s.ReadResult(resp)
	s.False(body.IsOk(), "Condition should be false")
	s.Equal(result.ErrCodeBadRequest, body.Code, "Should match expected value")
}

// TestResolveChallengeInvalidToken tests that resolve_challenge rejects malformed tokens.
func (s *ChallengeFlowTestSuite) TestResolveChallengeInvalidToken() {
	s.challengeProvider.On("Type").Return("totp").Maybe()

	resp := s.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "resolve_challenge",
			Version:  "v1",
		},
		Params: map[string]any{
			"challengeToken": "invalid.token.here",
			"type":           "totp",
			"response":       "123456",
		},
	})

	s.Equal(401, resp.StatusCode, "Should match expected value")

	body := s.ReadResult(resp)
	s.False(body.IsOk(), "Condition should be false")
	s.Equal(result.ErrCodeChallengeTokenInvalid, body.Code, "Should match expected value")
}

// TestResolveChallengeWrongType tests that resolve_challenge rejects a type not in pending list.
func (s *ChallengeFlowTestSuite) TestResolveChallengeWrongType() {
	s.challengeProvider.On("Type").Return("totp").Maybe()
	s.challengeProvider.On("Evaluate", mock.Anything, mock.Anything).
		Return(&security.LoginChallenge{
			Type:     "totp",
			Required: true,
		}, nil).Once()

	data := s.loginAndGetResult()
	challengeToken := data["challengeToken"].(string)

	resp := s.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "resolve_challenge",
			Version:  "v1",
		},
		Params: map[string]any{
			"challengeToken": challengeToken,
			"type":           "unknown_type",
			"response":       "123456",
		},
	})

	s.Equal(400, resp.StatusCode, "Should match expected value")

	body := s.ReadResult(resp)
	s.False(body.IsOk(), "Condition should be false")
	s.Equal(result.ErrCodeChallengeTypeInvalid, body.Code, "Should match expected value")
}

// TestResolveChallengeProviderRejectsResponse tests that resolve_challenge
// propagates errors from ChallengeProvider.Resolve.
func (s *ChallengeFlowTestSuite) TestResolveChallengeProviderRejectsResponse() {
	s.challengeProvider.On("Type").Return("totp").Maybe()
	s.challengeProvider.On("Evaluate", mock.Anything, mock.Anything).
		Return(&security.LoginChallenge{
			Type:     "totp",
			Required: true,
		}, nil).Once()

	data := s.loginAndGetResult()
	challengeToken := data["challengeToken"].(string)

	s.challengeProvider.On("Resolve", mock.Anything, mock.Anything, "wrong_code").
		Return((*security.Principal)(nil), result.Err(
			i18n.T(result.ErrMessageChallengeResolveFailed),
			result.WithCode(result.ErrCodeChallengeResolveFailed),
			result.WithStatus(400),
		)).Once()

	resp := s.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "resolve_challenge",
			Version:  "v1",
		},
		Params: map[string]any{
			"challengeToken": challengeToken,
			"type":           "totp",
			"response":       "wrong_code",
		},
	})

	s.Equal(400, resp.StatusCode, "Should match expected value")

	body := s.ReadResult(resp)
	s.False(body.IsOk(), "Condition should be false")
	s.Equal(result.ErrCodeChallengeResolveFailed, body.Code, "Should match expected value")
}

// TestLoginEvaluateChallengeError tests that login propagates errors from challenge evaluation.
func (s *ChallengeFlowTestSuite) TestLoginEvaluateChallengeError() {
	s.challengeProvider.On("Type").Return("totp").Maybe()
	s.challengeProvider.On("Evaluate", mock.Anything, mock.Anything).
		Return((*security.LoginChallenge)(nil), errors.New("totp service unavailable")).Once()

	resp := s.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "login",
			Version:  "v1",
		},
		Params: map[string]any{
			"type":        isecurity.AuthTypePassword,
			"principal":   "testuser",
			"credentials": "password123",
		},
	})

	s.Equal(500, resp.StatusCode, "Should return 500 when challenge evaluation fails")

	body := s.ReadResult(resp)
	s.False(body.IsOk(), "Login should fail when challenge evaluation fails")

	s.challengeProvider.AssertExpectations(s.T())
}

// TestGetUserInfoNilLoader tests that get_user_info returns not implemented when no UserInfoLoader.
func (s *ChallengeFlowTestSuite) TestGetUserInfoNilLoader() {
	// ChallengeFlowTestSuite does not inject UserInfoLoader, so it should be nil
	s.challengeProvider.On("Type").Return("totp").Maybe()
	s.challengeProvider.On("Evaluate", mock.Anything, mock.Anything).
		Return((*security.LoginChallenge)(nil), nil).Once()

	// Login to get a token (no challenge)
	data := s.loginAndGetResult()
	tokensRaw := data["tokens"].(map[string]any)
	accessToken := tokensRaw["accessToken"].(string)

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "get_user_info",
			Version:  "v1",
		},
	}, accessToken)

	body := s.ReadResult(resp)
	s.False(body.IsOk(), "Get user info should fail when loader is nil")
	s.Equal(result.ErrCodeNotImplemented, body.Code, "Should return not implemented error")
}

func TestChallengeFlow(t *testing.T) {
	suite.Run(t, new(ChallengeFlowTestSuite))
}

// --- Error Path Test Suite ---

type MockAuthManager struct {
	mock.Mock
}

func (m *MockAuthManager) Authenticate(ctx context.Context, authentication security.Authentication) (*security.Principal, error) {
	args := m.Called(ctx, authentication)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*security.Principal), args.Error(1)
}

type MockTokenGenerator struct {
	mock.Mock
}

func (m *MockTokenGenerator) Generate(principal *security.Principal) (*security.AuthTokens, error) {
	args := m.Called(principal)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*security.AuthTokens), args.Error(1)
}

type MockChallengeTokenStore struct {
	mock.Mock
}

func (m *MockChallengeTokenStore) Generate(principal *security.Principal, pending, resolved []string) (string, error) {
	args := m.Called(principal, pending, resolved)

	return args.String(0), args.Error(1)
}

func (m *MockChallengeTokenStore) Parse(token string) (*security.ChallengeState, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*security.ChallengeState), args.Error(1)
}

// AuthResourceErrorPathTestSuite tests error paths in AuthResource using mocked dependencies.
type AuthResourceErrorPathTestSuite struct {
	apptest.Suite

	authManager         *MockAuthManager
	tokenGenerator      *MockTokenGenerator
	challengeTokenStore *MockChallengeTokenStore
	publisher           *MockPublisher
	challengeProviderA  *MockChallengeProvider
	challengeProviderB  *MockChallengeProvider
	testUser            *security.Principal
}

func (s *AuthResourceErrorPathTestSuite) SetupSuite() {
	s.testUser = security.NewUser("user001", "Test User", "admin")

	s.authManager = new(MockAuthManager)
	s.tokenGenerator = new(MockTokenGenerator)
	s.challengeTokenStore = new(MockChallengeTokenStore)
	s.publisher = new(MockPublisher)
	s.challengeProviderA = new(MockChallengeProvider)
	s.challengeProviderB = new(MockChallengeProvider)

	s.challengeProviderA.On("Type").Return("totp")
	s.challengeProviderA.On("Order").Return(10)
	s.challengeProviderB.On("Type").Return("sms")
	s.challengeProviderB.On("Order").Return(20)
	s.publisher.On("Publish", mock.Anything).Maybe()

	s.SetupApp(
		fx.Decorate(func() security.AuthManager { return s.authManager }),
		fx.Decorate(func() security.TokenGenerator { return s.tokenGenerator }),
		fx.Decorate(func() security.ChallengeTokenStore { return s.challengeTokenStore }),
		fx.Supply(
			fx.Annotate(
				s.challengeProviderA,
				fx.As(new(security.ChallengeProvider)),
				fx.ResultTags(`group:"vef:security:challenge_providers"`),
			),
		),
		fx.Supply(
			fx.Annotate(
				s.challengeProviderB,
				fx.As(new(security.ChallengeProvider)),
				fx.ResultTags(`group:"vef:security:challenge_providers"`),
			),
		),
		// UserLoader is required by PasswordAuthenticator.
		fx.Supply(
			fx.Annotate(
				new(MockUserLoader),
				fx.As(new(security.UserLoader)),
			),
		),
		fx.Replace(
			fx.Annotate(
				s.publisher,
				fx.As(new(event.Publisher)),
			),
		),
		fx.Replace(
			&config.DataSourceConfig{Kind: "sqlite"},
			&config.SecurityConfig{
				TokenExpires:     24 * time.Hour,
				RefreshNotBefore: 1 * time.Millisecond,
				LoginRateLimit:   1000,
				RefreshRateLimit: 1000,
			},
			&security.JWTConfig{
				Secret:   testJWTSecret,
				Audience: "test-app",
			},
		),
	)
}

func (s *AuthResourceErrorPathTestSuite) TearDownSuite() {
	s.TearDownApp()
}

// resetMock clears all recorded calls and expected calls on a testify mock.
func resetMock(m *mock.Mock) {
	m.Calls = nil
	m.ExpectedCalls = nil
}

func (s *AuthResourceErrorPathTestSuite) SetupTest() {
	resetMock(&s.authManager.Mock)
	resetMock(&s.tokenGenerator.Mock)
	resetMock(&s.challengeTokenStore.Mock)
	resetMock(&s.challengeProviderA.Mock)
	resetMock(&s.challengeProviderB.Mock)

	s.publisher.Calls = nil
	s.publisher.ClearPublishedEvents()

	// Re-register stable expectations.
	s.challengeProviderA.On("Type").Return("totp")
	s.challengeProviderA.On("Order").Return(10)
	s.challengeProviderB.On("Type").Return("sms")
	s.challengeProviderB.On("Order").Return(20)
	s.publisher.On("Publish", mock.Anything).Maybe()
}

func (*AuthResourceErrorPathTestSuite) loginRequest() api.Request {
	return api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "login",
			Version:  "v1",
		},
		Params: map[string]any{
			"type":        "password",
			"principal":   "testuser",
			"credentials": "password123",
		},
	}
}

func (*AuthResourceErrorPathTestSuite) resolveChallengeRequest() api.Request {
	return api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "resolve_challenge",
			Version:  "v1",
		},
		Params: map[string]any{
			"challengeToken": "valid-challenge-token",
			"type":           "totp",
			"response":       "123456",
		},
	}
}

// skipChallenges stubs both providers to return no challenge.
func (s *AuthResourceErrorPathTestSuite) skipChallenges() {
	s.challengeProviderA.On("Evaluate", mock.Anything, mock.Anything).
		Return((*security.LoginChallenge)(nil), nil).Maybe()
	s.challengeProviderB.On("Evaluate", mock.Anything, mock.Anything).
		Return((*security.LoginChallenge)(nil), nil).Maybe()
}

// TestLoginNonResultError covers the branch where AuthManager.Authenticate
// returns a plain error (not a result.Err), triggering the else-branch.
func (s *AuthResourceErrorPathTestSuite) TestLoginNonResultError() {
	s.authManager.On("Authenticate", mock.Anything, mock.Anything).
		Return((*security.Principal)(nil), errors.New("unexpected db failure")).Once()

	resp := s.MakeRPCRequest(s.loginRequest())

	s.Equal(500, resp.StatusCode, "Should match expected value")

	body := s.ReadResult(resp)
	s.False(body.IsOk(), "Condition should be false")
	s.Equal(result.ErrCodeUnknown, body.Code, "Should match expected value")

	events := s.publisher.GetPublishedEvents()
	s.Require().Len(events, 1, "Should have expected length")
	loginEvent := events[0].(*security.LoginEvent)
	s.False(loginEvent.IsOk, "Condition should be false")
	s.Equal("unexpected db failure", loginEvent.FailReason, "Should match expected value")
	s.Equal(result.ErrCodeUnknown, loginEvent.ErrorCode, "Should match expected value")
}

// TestLoginTokenGenerateError covers the branch where authentication succeeds
// with no challenges but TokenGenerator.Generate fails.
func (s *AuthResourceErrorPathTestSuite) TestLoginTokenGenerateError() {
	s.authManager.On("Authenticate", mock.Anything, mock.Anything).
		Return(s.testUser, nil).Once()
	s.skipChallenges()
	s.tokenGenerator.On("Generate", s.testUser).
		Return((*security.AuthTokens)(nil), errors.New("token signing failed")).Once()

	resp := s.MakeRPCRequest(s.loginRequest())

	s.Equal(500, resp.StatusCode, "Should match expected value")

	body := s.ReadResult(resp)
	s.False(body.IsOk(), "Condition should be false")
}

// TestLoginChallengeStoreError covers the branch where authentication succeeds,
// a challenge is present, but ChallengeTokenStore.Generate fails.
func (s *AuthResourceErrorPathTestSuite) TestLoginChallengeStoreError() {
	s.authManager.On("Authenticate", mock.Anything, mock.Anything).
		Return(s.testUser, nil).Once()
	s.challengeProviderA.On("Evaluate", mock.Anything, mock.Anything).
		Return(&security.LoginChallenge{Type: "totp", Required: true}, nil).Once()
	s.challengeTokenStore.On("Generate", s.testUser, mock.Anything, mock.Anything).
		Return("", errors.New("store unavailable")).Once()

	resp := s.MakeRPCRequest(s.loginRequest())

	s.Equal(500, resp.StatusCode, "Should match expected value")

	body := s.ReadResult(resp)
	s.False(body.IsOk(), "Condition should be false")
}

// TestRefreshTokenGenerateError covers the branch where refresh authentication
// succeeds but TokenGenerator.Generate fails.
func (s *AuthResourceErrorPathTestSuite) TestRefreshTokenGenerateError() {
	s.authManager.On("Authenticate", mock.Anything, mock.Anything).
		Return(s.testUser, nil).Once()
	s.tokenGenerator.On("Generate", s.testUser).
		Return((*security.AuthTokens)(nil), errors.New("token generation failed")).Once()

	resp := s.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "refresh",
			Version:  "v1",
		},
		Params: map[string]any{
			"refreshToken": "valid-refresh-token",
		},
	})

	s.Equal(500, resp.StatusCode, "Should match expected value")

	body := s.ReadResult(resp)
	s.False(body.IsOk(), "Condition should be false")
}

// TestResolveChallengeProviderNotFound covers the branch where the challenge type
// exists in the pending list but the provider is not registered.
func (s *AuthResourceErrorPathTestSuite) TestResolveChallengeProviderNotFound() {
	s.challengeTokenStore.On("Parse", "valid-challenge-token").
		Return(&security.ChallengeState{
			Principal: s.testUser,
			Pending:   []string{"email"},
		}, nil).Once()

	resp := s.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "resolve_challenge",
			Version:  "v1",
		},
		Params: map[string]any{
			"challengeToken": "valid-challenge-token",
			"type":           "email",
			"response":       "123456",
		},
	})

	body := s.ReadResult(resp)
	s.False(body.IsOk(), "Condition should be false")
	s.Equal(result.ErrCodeChallengeTypeInvalid, body.Code, "Should match expected value")
}

// TestResolveChallengeTokenGenerateError covers the branch where all challenges
// are resolved but TokenGenerator.Generate fails.
func (s *AuthResourceErrorPathTestSuite) TestResolveChallengeTokenGenerateError() {
	s.challengeTokenStore.On("Parse", "valid-challenge-token").
		Return(&security.ChallengeState{
			Principal: s.testUser,
			Pending:   []string{"totp"},
		}, nil).Once()
	s.challengeProviderA.On("Resolve", mock.Anything, s.testUser, "123456").
		Return(s.testUser, nil).Once()
	s.tokenGenerator.On("Generate", s.testUser).
		Return((*security.AuthTokens)(nil), errors.New("token signing failed")).Once()

	resp := s.MakeRPCRequest(s.resolveChallengeRequest())

	s.Equal(500, resp.StatusCode, "Should match expected value")

	body := s.ReadResult(resp)
	s.False(body.IsOk(), "Condition should be false")
}

// TestResolveChallengeMoreRemain covers the branch where resolving one challenge
// leaves others pending, returning a new challenge token with the next challenge.
func (s *AuthResourceErrorPathTestSuite) TestResolveChallengeMoreRemain() {
	s.challengeTokenStore.On("Parse", "valid-challenge-token").
		Return(&security.ChallengeState{
			Principal: s.testUser,
			Pending:   []string{"totp", "sms"},
		}, nil).Once()
	s.challengeProviderA.On("Resolve", mock.Anything, s.testUser, "123456").
		Return(s.testUser, nil).Once()
	s.challengeProviderB.On("Evaluate", mock.Anything, s.testUser).
		Return(&security.LoginChallenge{Type: "sms", Required: true}, nil).Once()
	s.challengeTokenStore.On("Generate", s.testUser, []string{"sms"}, []string{"totp"}).
		Return("new-challenge-token", nil).Once()

	resp := s.MakeRPCRequest(s.resolveChallengeRequest())

	s.Equal(200, resp.StatusCode, "Should match expected value")

	body := s.ReadResult(resp)
	s.True(body.IsOk(), "Condition should be true")

	data := s.ReadDataAsMap(body.Data)
	s.Equal("new-challenge-token", data["challengeToken"], "Should match expected value")

	challenge, ok := data["challenge"].(map[string]any)
	s.Require().True(ok, "Challenge should be an object")
	s.Equal("sms", challenge["type"], "Should match expected value")
	s.Equal(true, challenge["required"], "Should match expected value")
}

// TestResolveChallengeStoreErrorOnRemain covers the branch where remaining challenges
// exist but ChallengeTokenStore.Generate fails for the new token.
func (s *AuthResourceErrorPathTestSuite) TestResolveChallengeStoreErrorOnRemain() {
	s.challengeTokenStore.On("Parse", "valid-challenge-token").
		Return(&security.ChallengeState{
			Principal: s.testUser,
			Pending:   []string{"totp", "sms"},
		}, nil).Once()
	s.challengeProviderA.On("Resolve", mock.Anything, s.testUser, "123456").
		Return(s.testUser, nil).Once()
	s.challengeProviderB.On("Evaluate", mock.Anything, s.testUser).
		Return(&security.LoginChallenge{Type: "sms", Required: true}, nil).Once()
	s.challengeTokenStore.On("Generate", s.testUser, []string{"sms"}, []string{"totp"}).
		Return("", errors.New("store failure")).Once()

	resp := s.MakeRPCRequest(s.resolveChallengeRequest())

	s.Equal(500, resp.StatusCode, "Should match expected value")

	body := s.ReadResult(resp)
	s.False(body.IsOk(), "Condition should be false")
}

// TestResolveChallengeEvaluateErrorOnRemain covers the branch where remaining
// challenges exist but the next provider.Evaluate fails.
func (s *AuthResourceErrorPathTestSuite) TestResolveChallengeEvaluateErrorOnRemain() {
	s.challengeTokenStore.On("Parse", "valid-challenge-token").
		Return(&security.ChallengeState{
			Principal: s.testUser,
			Pending:   []string{"totp", "sms"},
		}, nil).Once()
	s.challengeProviderA.On("Resolve", mock.Anything, s.testUser, "123456").
		Return(s.testUser, nil).Once()
	s.challengeProviderB.On("Evaluate", mock.Anything, s.testUser).
		Return((*security.LoginChallenge)(nil), errors.New("sms service unavailable")).Once()

	resp := s.MakeRPCRequest(s.resolveChallengeRequest())

	s.Equal(500, resp.StatusCode, "Should match expected value")

	body := s.ReadResult(resp)
	s.False(body.IsOk(), "Condition should be false")
}

// TestResolveChallengeRemainingProviderNotFound covers the skip branch
// when the next pending type has no registered provider — it is skipped
// and since no more challenges remain, auth tokens are issued.
func (s *AuthResourceErrorPathTestSuite) TestResolveChallengeRemainingProviderNotFound() {
	s.challengeTokenStore.On("Parse", "valid-challenge-token").
		Return(&security.ChallengeState{
			Principal: s.testUser,
			Pending:   []string{"totp", "unknown_type"},
		}, nil).Once()
	s.challengeProviderA.On("Resolve", mock.Anything, s.testUser, "123456").
		Return(s.testUser, nil).Once()
	s.tokenGenerator.On("Generate", s.testUser).
		Return(&security.AuthTokens{AccessToken: "at", RefreshToken: "rt"}, nil).Once()

	resp := s.MakeRPCRequest(s.resolveChallengeRequest())

	s.Equal(200, resp.StatusCode, "Should match expected value")

	body := s.ReadResult(resp)
	s.True(body.IsOk(), "Condition should be true")

	data := s.ReadDataAsMap(body.Data)
	s.Nil(data["challengeToken"], "No challenge token when all resolved")
	s.Nil(data["challenge"], "No challenge when all resolved")

	tokensRaw, ok := data["tokens"].(map[string]any)
	s.Require().True(ok, "Should have tokens")
	s.NotEmpty(tokensRaw["accessToken"], "Should not be empty")
	s.NotEmpty(tokensRaw["refreshToken"], "Should not be empty")
}

func TestAuthResourceErrorPath(t *testing.T) {
	suite.Run(t, new(AuthResourceErrorPathTestSuite))
}
