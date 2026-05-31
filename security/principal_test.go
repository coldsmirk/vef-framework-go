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
		assert.Equal(t, PrincipalTypeUser, user.Type, "New user should have user principal type")
		assert.Equal(t, "user123", user.ID, "New user should preserve ID")
		assert.Equal(t, "John Doe", user.Name, "New user should preserve name")
		assert.Empty(t, user.Roles, "New user without roles should have empty roles")
		assert.Nil(t, user.Details, "New user without details should keep details nil")
	})

	t.Run("CreateUserWithRoles", func(t *testing.T) {
		user := NewUser("user456", "Jane Smith", "admin", "editor")
		assert.Equal(t, PrincipalTypeUser, user.Type, "New user with roles should have user principal type")
		assert.Equal(t, "user456", user.ID, "New user with roles should preserve ID")
		assert.Equal(t, "Jane Smith", user.Name, "New user with roles should preserve name")
		assert.Equal(t, []string{"admin", "editor"}, user.Roles, "New user should preserve supplied roles")
	})
}

// TestNewExternalApp tests new external app functionality.
func TestNewExternalApp(t *testing.T) {
	t.Run("CreateExternalAppWithoutRoles", func(t *testing.T) {
		app := NewExternalApp("app123", "Payment Service")
		assert.Equal(t, PrincipalTypeExternalApp, app.Type, "New external app should have external app principal type")
		assert.Equal(t, "app123", app.ID, "New external app should preserve ID")
		assert.Equal(t, "Payment Service", app.Name, "New external app should preserve name")
		assert.Empty(t, app.Roles, "New external app without roles should have empty roles")
		assert.Nil(t, app.Details, "New external app without details should keep details nil")
	})

	t.Run("CreateExternalAppWithRoles", func(t *testing.T) {
		app := NewExternalApp("app456", "Auth Service", "service", "trusted")
		assert.Equal(t, PrincipalTypeExternalApp, app.Type, "New external app with roles should have external app principal type")
		assert.Equal(t, "app456", app.ID, "New external app with roles should preserve ID")
		assert.Equal(t, "Auth Service", app.Name, "New external app with roles should preserve name")
		assert.Equal(t, []string{"service", "trusted"}, app.Roles, "New external app should preserve supplied roles")
	})
}

// TestPrincipalWithRoles tests principal with roles functionality.
func TestPrincipalWithRoles(t *testing.T) {
	t.Run("AddRolesToPrincipal", func(t *testing.T) {
		user := NewUser("user123", "Test User")
		user.WithRoles("admin", "moderator")
		assert.Equal(t, []string{"admin", "moderator"}, user.Roles, "WithRoles should append roles to principal")
	})

	t.Run("AddRolesMultipleTimes", func(t *testing.T) {
		user := NewUser("user123", "Test User", "viewer")
		user.WithRoles("admin").WithRoles("editor")
		assert.Equal(t, []string{"viewer", "admin", "editor"}, user.Roles, "Multiple WithRoles calls should preserve existing and appended roles")
	})
}

// TestPrincipalSystem tests principal system functionality.
func TestPrincipalSystem(t *testing.T) {
	t.Run("SystemPrincipalHasCorrectValues", func(t *testing.T) {
		assert.Equal(t, PrincipalTypeSystem, PrincipalSystem.Type, "System principal should use system type")
		assert.Equal(t, orm.OperatorSystem, PrincipalSystem.ID, "System principal should use system operator ID")
		assert.Equal(t, "系统", PrincipalSystem.Name, "System principal should use localized system name")
	})
}

// TestPrincipalAnonymous tests principal anonymous functionality.
func TestPrincipalAnonymous(t *testing.T) {
	t.Run("AnonymousPrincipalHasCorrectValues", func(t *testing.T) {
		assert.Equal(t, PrincipalTypeUser, PrincipalAnonymous.Type, "Anonymous principal should use user type")
		assert.Equal(t, orm.OperatorAnonymous, PrincipalAnonymous.ID, "Anonymous principal should use anonymous operator ID")
		assert.Equal(t, "匿名", PrincipalAnonymous.Name, "Anonymous principal should use localized anonymous name")
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
		require.NoError(t, err, "User with map details should marshal to JSON")

		var result map[string]any

		err = json.Unmarshal(data, &result)
		require.NoError(t, err, "Marshaled user with map details should unmarshal into map")

		assert.Equal(t, "user", result["type"], "Marshaled user should include type")
		assert.Equal(t, "user123", result["id"], "Marshaled user should include ID")
		assert.Equal(t, "Test User", result["name"], "Marshaled user should include name")
		assert.Contains(t, result, "details", "Marshaled user should include details field")
	})

	t.Run("MarshalUserWithoutDetails", func(t *testing.T) {
		user := NewUser("user123", "Test User")

		data, err := json.Marshal(user)
		require.NoError(t, err, "User without details should marshal to JSON")

		var result map[string]any

		err = json.Unmarshal(data, &result)
		require.NoError(t, err, "Marshaled user without details should unmarshal into map")

		assert.Equal(t, "user", result["type"], "Marshaled user without details should include type")
		assert.Nil(t, result["details"], "Marshaled user without details should keep details nil")
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

		assert.Equal(t, PrincipalTypeUser, principal.Type, "Unmarshaled user should have user principal type")
		assert.Equal(t, "user123", principal.ID, "Unmarshaled user should preserve ID")
		assert.Equal(t, "Test User", principal.Name, "Unmarshaled user should preserve name")
		assert.Equal(t, []string{"admin", "editor"}, principal.Roles, "Unmarshaled user should preserve roles")

		detailsPtr, ok := principal.Details.(*map[string]any)
		require.True(t, ok, "Details should be a map")

		details := *detailsPtr
		assert.Equal(t, "test@example.com", details["email"], "Unmarshaled map details should preserve email")
		assert.Equal(t, float64(30), details["age"], "Unmarshaled map details should preserve age")
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
		assert.Equal(t, "jane@example.com", details.Email, "Unmarshaled user details should preserve email")
		assert.Equal(t, "+1234567890", details.PhoneNumber, "Unmarshaled user details should preserve phone number")
		assert.Equal(t, 25, details.Age, "Unmarshaled user details should preserve age")
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
		assert.Equal(t, "app_123456", details.AppID, "Unmarshaled external app details should preserve app ID")
		assert.Equal(t, "secret_abc", details.AppSecret, "Unmarshaled external app details should preserve app secret")
		assert.Equal(t, []string{"read", "write"}, details.Scopes, "Unmarshaled external app details should preserve scopes")
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
		require.NoError(t, err, "System principal JSON should unmarshal")

		assert.Equal(t, PrincipalTypeSystem, principal.Type, "Unmarshaled system principal should have system type")
		assert.Equal(t, orm.OperatorSystem, principal.ID, "Unmarshaled system principal should preserve system ID")
		assert.Nil(t, principal.Details, "Unmarshaled system principal should keep details nil")
	})

	t.Run("UnmarshalInvalidJson", func(t *testing.T) {
		jsonData := `{invalid json}`

		var principal Principal

		err := json.Unmarshal([]byte(jsonData), &principal)
		assert.Error(t, err, "Invalid principal JSON should return an error")
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
		require.True(t, ok, "User details map should convert to TestUserDetails")
		assert.Equal(t, "test@example.com", details.Email, "Converted user details should preserve email")
		assert.Equal(t, "+1234567890", details.PhoneNumber, "Converted user details should preserve phone number")
		assert.Equal(t, 30, details.Age, "Converted user details should preserve age")
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
		assert.Equal(t, "app_123", details.AppID, "Converted external app details should preserve app ID")
		assert.Equal(t, "secret", details.AppSecret, "Converted external app details should preserve app secret")
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
		assert.Equal(t, stringDetails, user.Details, "Non-map user details should remain unchanged")
	})

	t.Run("SystemPrincipalKeepsDetailsAsIs", func(t *testing.T) {
		principal := &Principal{
			Type: PrincipalTypeSystem,
			ID:   "system",
			Name: "System",
		}

		details := map[string]any{"key": "value"}
		principal.AttemptUnmarshalDetails(details)
		assert.Equal(t, details, principal.Details, "System principal details should remain unchanged")
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
		assert.Equal(t, "test@example.com", details.Email, "Partial user details should preserve present email")
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
		require.NoError(t, err, "Original user principal should marshal")

		// Unmarshal
		var restored Principal

		err = json.Unmarshal(data, &restored)
		require.NoError(t, err, "Marshaled user principal should unmarshal")

		assert.Equal(t, original.Type, restored.Type, "User round trip should preserve type")
		assert.Equal(t, original.ID, restored.ID, "User round trip should preserve ID")
		assert.Equal(t, original.Name, restored.Name, "User round trip should preserve name")
		assert.Equal(t, original.Roles, restored.Roles, "User round trip should preserve roles")
	})

	t.Run("MarshalAndUnmarshalExternalApp", func(t *testing.T) {
		original := NewExternalApp("app123", "Auth Service", "service")
		original.Details = map[string]any{
			"appID":  "123",
			"scopes": []string{"read", "write"},
		}

		// Marshal
		data, err := json.Marshal(original)
		require.NoError(t, err, "Original external app principal should marshal")

		// Unmarshal
		var restored Principal

		err = json.Unmarshal(data, &restored)
		require.NoError(t, err, "Marshaled external app principal should unmarshal")

		assert.Equal(t, original.Type, restored.Type, "External app round trip should preserve type")
		assert.Equal(t, original.ID, restored.ID, "External app round trip should preserve ID")
		assert.Equal(t, original.Name, restored.Name, "External app round trip should preserve name")
		assert.Equal(t, original.Roles, restored.Roles, "External app round trip should preserve roles")
	})
}
