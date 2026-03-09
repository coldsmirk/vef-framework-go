package security

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/orm"
)

type TestUserDetails struct {
	Email       string `json:"email"`
	PhoneNumber string `json:"phoneNumber"`
	Age         int    `json:"age"`
}

type TestExternalAppDetails struct {
	AppID     string   `json:"appId"`
	AppSecret string   `json:"appSecret"`
	Scopes    []string `json:"scopes"`
}

// TestNewUser tests new user functionality.
func TestNewUser(t *testing.T) {
	t.Run("CreateUserWithoutRoles", func(t *testing.T) {
		user := NewUser("user123", "John Doe")
		assert.Equal(t, PrincipalTypeUser, user.Type, "Should equal expected value")
		assert.Equal(t, "user123", user.ID, "Should equal expected value")
		assert.Equal(t, "John Doe", user.Name, "Should equal expected value")
		assert.Empty(t, user.Roles, "Should be empty")
		assert.Nil(t, user.Details, "Should be nil")
	})

	t.Run("CreateUserWithRoles", func(t *testing.T) {
		user := NewUser("user456", "Jane Smith", "admin", "editor")
		assert.Equal(t, PrincipalTypeUser, user.Type, "Should equal expected value")
		assert.Equal(t, "user456", user.ID, "Should equal expected value")
		assert.Equal(t, "Jane Smith", user.Name, "Should equal expected value")
		assert.Equal(t, []string{"admin", "editor"}, user.Roles, "Should equal expected value")
	})
}

// TestNewExternalApp tests new external app functionality.
func TestNewExternalApp(t *testing.T) {
	t.Run("CreateExternalAppWithoutRoles", func(t *testing.T) {
		app := NewExternalApp("app123", "Payment Service")
		assert.Equal(t, PrincipalTypeExternalApp, app.Type, "Should equal expected value")
		assert.Equal(t, "app123", app.ID, "Should equal expected value")
		assert.Equal(t, "Payment Service", app.Name, "Should equal expected value")
		assert.Empty(t, app.Roles, "Should be empty")
		assert.Nil(t, app.Details, "Should be nil")
	})

	t.Run("CreateExternalAppWithRoles", func(t *testing.T) {
		app := NewExternalApp("app456", "Auth Service", "service", "trusted")
		assert.Equal(t, PrincipalTypeExternalApp, app.Type, "Should equal expected value")
		assert.Equal(t, "app456", app.ID, "Should equal expected value")
		assert.Equal(t, "Auth Service", app.Name, "Should equal expected value")
		assert.Equal(t, []string{"service", "trusted"}, app.Roles, "Should equal expected value")
	})
}

// TestPrincipalWithRoles tests principal with roles functionality.
func TestPrincipalWithRoles(t *testing.T) {
	t.Run("AddRolesToPrincipal", func(t *testing.T) {
		user := NewUser("user123", "Test User")
		user.WithRoles("admin", "moderator")
		assert.Equal(t, []string{"admin", "moderator"}, user.Roles, "Should equal expected value")
	})

	t.Run("AddRolesMultipleTimes", func(t *testing.T) {
		user := NewUser("user123", "Test User", "viewer")
		user.WithRoles("admin").WithRoles("editor")
		assert.Equal(t, []string{"viewer", "admin", "editor"}, user.Roles, "Should equal expected value")
	})
}

// TestPrincipalSystem tests principal system functionality.
func TestPrincipalSystem(t *testing.T) {
	t.Run("SystemPrincipalHasCorrectValues", func(t *testing.T) {
		assert.Equal(t, PrincipalTypeSystem, PrincipalSystem.Type, "Should equal expected value")
		assert.Equal(t, orm.OperatorSystem, PrincipalSystem.ID, "Should equal expected value")
		assert.Equal(t, "系统", PrincipalSystem.Name, "Should equal expected value")
	})
}

// TestPrincipalAnonymous tests principal anonymous functionality.
func TestPrincipalAnonymous(t *testing.T) {
	t.Run("AnonymousPrincipalHasCorrectValues", func(t *testing.T) {
		assert.Equal(t, PrincipalTypeUser, PrincipalAnonymous.Type, "Should equal expected value")
		assert.Equal(t, orm.OperatorAnonymous, PrincipalAnonymous.ID, "Should equal expected value")
		assert.Equal(t, "匿名", PrincipalAnonymous.Name, "Should equal expected value")
	})
}

// TestPrincipalJSONMarshal tests principal JSON marshal functionality.
func TestPrincipalJSONMarshal(t *testing.T) {
	t.Run("MarshalUserWithMapDetails", func(t *testing.T) {
		user := NewUser("user123", "Test User", "admin")
		user.Details = map[string]any{
			"email": "test@example.com",
			"age":   30,
		}

		data, err := json.Marshal(user)
		require.NoError(t, err, "Should not return error")

		var result map[string]any

		err = json.Unmarshal(data, &result)
		require.NoError(t, err, "Should not return error")

		assert.Equal(t, "user", result["type"], "Should equal expected value")
		assert.Equal(t, "user123", result["id"], "Should equal expected value")
		assert.Equal(t, "Test User", result["name"], "Should equal expected value")
		assert.Contains(t, result, "details", "Should contain expected value")
	})

	t.Run("MarshalUserWithoutDetails", func(t *testing.T) {
		user := NewUser("user123", "Test User")

		data, err := json.Marshal(user)
		require.NoError(t, err, "Should not return error")

		var result map[string]any

		err = json.Unmarshal(data, &result)
		require.NoError(t, err, "Should not return error")

		assert.Equal(t, "user", result["type"], "Should equal expected value")
		assert.Nil(t, result["details"], "Should be nil")
	})
}

// TestPrincipalJSONUnmarshal tests principal JSON unmarshal functionality.
func TestPrincipalJSONUnmarshal(t *testing.T) {
	t.Run("UnmarshalUserWithMapDetails", func(t *testing.T) {
		jsonData := `{
			"type": "user",
			"id": "user123",
			"name": "Test User",
			"roles": ["admin", "editor"],
			"details": {
				"email": "test@example.com",
				"age": 30
			}
		}`

		var principal Principal

		err := json.Unmarshal([]byte(jsonData), &principal)
		require.NoError(t, err, "Should unmarshal user with map details")

		assert.Equal(t, PrincipalTypeUser, principal.Type, "Should equal expected value")
		assert.Equal(t, "user123", principal.ID, "Should equal expected value")
		assert.Equal(t, "Test User", principal.Name, "Should equal expected value")
		assert.Equal(t, []string{"admin", "editor"}, principal.Roles, "Should equal expected value")

		detailsPtr, ok := principal.Details.(*map[string]any)
		require.True(t, ok, "Details should be a map")

		details := *detailsPtr
		assert.Equal(t, "test@example.com", details["email"], "Should equal expected value")
		assert.Equal(t, float64(30), details["age"], "Should equal expected value")
	})

	t.Run("UnmarshalUserWithStructDetails", func(t *testing.T) {
		originalType := userDetailsType
		defer func() { userDetailsType = originalType }()

		SetUserDetailsType[TestUserDetails]()

		jsonData := `{
			"type": "user",
			"id": "user456",
			"name": "Jane Doe",
			"roles": ["viewer"],
			"details": {
				"email": "jane@example.com",
				"phoneNumber": "+1234567890",
				"age": 25
			}
		}`

		var principal Principal

		err := json.Unmarshal([]byte(jsonData), &principal)
		require.NoError(t, err, "Should unmarshal user with struct details")

		details, ok := principal.Details.(*TestUserDetails)
		require.True(t, ok, "Details should be TestUserDetails")
		assert.Equal(t, "jane@example.com", details.Email, "Should equal expected value")
		assert.Equal(t, "+1234567890", details.PhoneNumber, "Should equal expected value")
		assert.Equal(t, 25, details.Age, "Should equal expected value")
	})

	t.Run("UnmarshalExternalAppWithStructDetails", func(t *testing.T) {
		originalType := externalAppDetailsType
		defer func() { externalAppDetailsType = originalType }()

		SetExternalAppDetailsType[TestExternalAppDetails]()

		jsonData := `{
			"type": "external_app",
			"id": "app123",
			"name": "Auth Service",
			"roles": ["service"],
			"details": {
				"appId": "app_123456",
				"appSecret": "secret_abc",
				"scopes": ["read", "write"]
			}
		}`

		var principal Principal

		err := json.Unmarshal([]byte(jsonData), &principal)
		require.NoError(t, err, "Should unmarshal external app with struct details")

		details, ok := principal.Details.(*TestExternalAppDetails)
		require.True(t, ok, "Details should be TestExternalAppDetails")
		assert.Equal(t, "app_123456", details.AppID, "Should equal expected value")
		assert.Equal(t, "secret_abc", details.AppSecret, "Should equal expected value")
		assert.Equal(t, []string{"read", "write"}, details.Scopes, "Should equal expected value")
	})

	t.Run("UnmarshalSystemPrincipal", func(t *testing.T) {
		jsonData := `{
			"type": "system",
			"id": "system",
			"name": "系统",
			"details": null
		}`

		var principal Principal

		err := json.Unmarshal([]byte(jsonData), &principal)
		require.NoError(t, err, "Should not return error")

		assert.Equal(t, PrincipalTypeSystem, principal.Type, "Should equal expected value")
		assert.Equal(t, orm.OperatorSystem, principal.ID, "Should equal expected value")
		assert.Nil(t, principal.Details, "Should be nil")
	})

	t.Run("UnmarshalInvalidJson", func(t *testing.T) {
		jsonData := `{invalid json}`

		var principal Principal

		err := json.Unmarshal([]byte(jsonData), &principal)
		assert.Error(t, err, "Should return error")
	})
}

// TestAttemptUnmarshalDetails tests attempt unmarshal details functionality.
func TestAttemptUnmarshalDetails(t *testing.T) {
	t.Run("UnmarshalUserDetailsFromMap", func(t *testing.T) {
		originalType := userDetailsType
		defer func() { userDetailsType = originalType }() // Reset at end

		SetUserDetailsType[TestUserDetails]()

		user := NewUser("user123", "Test User")
		detailsMap := map[string]any{
			"email":       "test@example.com",
			"phoneNumber": "+1234567890",
			"age":         30,
		}

		user.AttemptUnmarshalDetails(detailsMap)

		details, ok := user.Details.(*TestUserDetails)
		require.True(t, ok, "Should be ok")
		assert.Equal(t, "test@example.com", details.Email, "Should equal expected value")
		assert.Equal(t, "+1234567890", details.PhoneNumber, "Should equal expected value")
		assert.Equal(t, 30, details.Age, "Should equal expected value")
	})

	t.Run("UnmarshalExternalAppDetailsFromMap", func(t *testing.T) {
		originalType := externalAppDetailsType
		defer func() { externalAppDetailsType = originalType }()

		SetExternalAppDetailsType[TestExternalAppDetails]()

		app := NewExternalApp("app123", "Test App")
		detailsMap := map[string]any{
			"appId":     "app_123",
			"appSecret": "secret",
			"scopes":    []any{"read", "write"},
		}

		app.AttemptUnmarshalDetails(detailsMap)

		details, ok := app.Details.(*TestExternalAppDetails)
		require.True(t, ok, "Details should be TestExternalAppDetails")
		assert.Equal(t, "app_123", details.AppID, "Should equal expected value")
		assert.Equal(t, "secret", details.AppSecret, "Should equal expected value")
	})

	t.Run("DetailsTypeIsMapKeepAsIs", func(t *testing.T) {
		user := NewUser("user123", "Test User")
		detailsMap := map[string]any{
			"key": "value",
		}

		user.AttemptUnmarshalDetails(detailsMap)
		assert.Equal(t, detailsMap, user.Details, "Details should remain as map")
	})

	t.Run("NonMapDetailsForUserType", func(t *testing.T) {
		user := NewUser("user123", "Test User")
		stringDetails := "string details"

		user.AttemptUnmarshalDetails(stringDetails)
		assert.Equal(t, stringDetails, user.Details, "Should equal expected value")
	})

	t.Run("SystemPrincipalKeepsDetailsAsIs", func(t *testing.T) {
		principal := &Principal{
			Type: PrincipalTypeSystem,
			ID:   "system",
			Name: "System",
		}

		details := map[string]any{"key": "value"}
		principal.AttemptUnmarshalDetails(details)
		assert.Equal(t, details, principal.Details, "Should equal expected value")
	})

	t.Run("DecodeWithPartialFieldsCreatesStruct", func(t *testing.T) {
		originalType := userDetailsType
		defer func() { userDetailsType = originalType }()

		SetUserDetailsType[TestUserDetails]()

		user := NewUser("user123", "Test User")
		partialDetails := map[string]any{
			"email":        "test@example.com",
			"invalidField": "value",
		}

		user.AttemptUnmarshalDetails(partialDetails)
		details, ok := user.Details.(*TestUserDetails)
		require.True(t, ok, "Details should be TestUserDetails")
		assert.Equal(t, "test@example.com", details.Email, "Should equal expected value")
		assert.Equal(t, "", details.PhoneNumber, "Unset field should have zero value")
	})
}

// TestSetUserDetailsType tests set user details type functionality.
func TestSetUserDetailsType(t *testing.T) {
	t.Run("SetValidStructType", func(t *testing.T) {
		originalType := userDetailsType
		defer func() { userDetailsType = originalType }()

		SetUserDetailsType[TestUserDetails]()
		assert.Equal(t, "TestUserDetails", userDetailsType.Name(), "Type name should be TestUserDetails")
	})

	t.Run("PanicOnNonStructType", func(t *testing.T) {
		assert.Panics(t, func() {
			SetUserDetailsType[string]()
		}, "Should panic on non-struct type")
	})
}

// TestSetExternalAppDetailsType tests set external app details type functionality.
func TestSetExternalAppDetailsType(t *testing.T) {
	t.Run("SetValidStructType", func(t *testing.T) {
		originalType := externalAppDetailsType
		defer func() { externalAppDetailsType = originalType }()

		SetExternalAppDetailsType[TestExternalAppDetails]()
		assert.Equal(t, "TestExternalAppDetails", externalAppDetailsType.Name(), "Type name should be TestExternalAppDetails")
	})

	t.Run("PanicOnNonStructType", func(t *testing.T) {
		assert.Panics(t, func() {
			SetExternalAppDetailsType[int]()
		}, "Should panic on non-struct type")
	})
}

// TestPrincipalRoundTrip tests principal round trip functionality.
func TestPrincipalRoundTrip(t *testing.T) {
	t.Run("MarshalAndUnmarshalUser", func(t *testing.T) {
		original := NewUser("user123", "Test User", "admin", "editor")
		original.Details = map[string]any{
			"email": "test@example.com",
			"age":   30,
		}

		// Marshal
		data, err := json.Marshal(original)
		require.NoError(t, err, "Should not return error")

		// Unmarshal
		var restored Principal

		err = json.Unmarshal(data, &restored)
		require.NoError(t, err, "Should not return error")

		assert.Equal(t, original.Type, restored.Type, "Should equal expected value")
		assert.Equal(t, original.ID, restored.ID, "Should equal expected value")
		assert.Equal(t, original.Name, restored.Name, "Should equal expected value")
		assert.Equal(t, original.Roles, restored.Roles, "Should equal expected value")
	})

	t.Run("MarshalAndUnmarshalExternalApp", func(t *testing.T) {
		original := NewExternalApp("app123", "Auth Service", "service")
		original.Details = map[string]any{
			"appID":  "123",
			"scopes": []string{"read", "write"},
		}

		// Marshal
		data, err := json.Marshal(original)
		require.NoError(t, err, "Should not return error")

		// Unmarshal
		var restored Principal

		err = json.Unmarshal(data, &restored)
		require.NoError(t, err, "Should not return error")

		assert.Equal(t, original.Type, restored.Type, "Should equal expected value")
		assert.Equal(t, original.ID, restored.ID, "Should equal expected value")
		assert.Equal(t, original.Name, restored.Name, "Should equal expected value")
		assert.Equal(t, original.Roles, restored.Roles, "Should equal expected value")
	})
}
