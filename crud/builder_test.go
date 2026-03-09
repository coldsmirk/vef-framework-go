package crud_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// TestBuilderMethods tests the baseBuilder configuration methods.
func TestBuilderMethods(t *testing.T) {
	t.Run("EnableAudit", func(t *testing.T) {
		c := crud.NewCreate[orm.Model, orm.Model]().EnableAudit()
		specs := c.Provide()
		require.Len(t, specs, 1, "Should return exactly 1 operation spec")
		assert.True(t, specs[0].EnableAudit, "EnableAudit should set enableAudit to true")
	})

	t.Run("Timeout", func(t *testing.T) {
		c := crud.NewCreate[orm.Model, orm.Model]().Timeout(5 * time.Second)
		specs := c.Provide()
		assert.Equal(t, 5*time.Second, specs[0].Timeout, "Timeout should be set")
	})

	t.Run("PermToken", func(t *testing.T) {
		c := crud.NewCreate[orm.Model, orm.Model]().PermToken("admin:create")
		specs := c.Provide()
		assert.Equal(t, "admin:create", specs[0].PermToken, "PermToken should be set")
	})

	t.Run("RateLimit", func(t *testing.T) {
		c := crud.NewCreate[orm.Model, orm.Model]().RateLimit(100, 1*time.Minute)
		specs := c.Provide()
		assert.NotNil(t, specs[0].RateLimit, "RateLimit should be set")
		assert.Equal(t, 100, specs[0].RateLimit.Max, "RateLimit max should be set")
		assert.Equal(t, 1*time.Minute, specs[0].RateLimit.Period, "RateLimit period should be set")
	})

	t.Run("ResourceKind", func(t *testing.T) {
		c := crud.NewCreate[orm.Model, orm.Model]().ResourceKind(api.KindREST)
		assert.NotNil(t, c, "ResourceKind should return the builder")
	})

	t.Run("CombinedOptions", func(t *testing.T) {
		c := crud.NewCreate[orm.Model, orm.Model]().
			EnableAudit().
			Timeout(10*time.Second).
			PermToken("user:write").
			RateLimit(50, 5*time.Minute).
			Public()
		specs := c.Provide()
		assert.True(t, specs[0].EnableAudit, "EnableAudit should be true")
		assert.Equal(t, 10*time.Second, specs[0].Timeout, "Timeout should be set")
		assert.Equal(t, "user:write", specs[0].PermToken, "PermToken should be set")
		assert.True(t, specs[0].Public, "Public should be true")
		assert.NotNil(t, specs[0].RateLimit, "RateLimit should be set")
	})
}

// TestAction tests the Action method validation for API builders.
func TestAction(t *testing.T) {
	t.Run("ValidActionNames", func(t *testing.T) {
		validActions := []struct {
			name   string
			action string
		}{
			{"SingleWord", "create"},
			{"SnakeCase", "find_page"},
			{"MultipleWords", "get_user_info"},
			{"TwoWords", "create_user"},
			{"WithMany", "delete_many"},
		}

		for _, tc := range validActions {
			t.Run(tc.name, func(t *testing.T) {
				assert.NotPanics(t, func() {
					_ = crud.NewCreate[orm.Model, orm.Model]().Action(tc.action)
				}, "Should accept valid snake_case action name")
			})
		}
	})

	t.Run("InvalidActionNames", func(t *testing.T) {
		invalidActions := []struct {
			name   string
			action string
		}{
			{"EmptyString", ""},
			{"CamelCase", "findPage"},
			{"PascalCase", "CreateUser"},
			{"StartsWithNumber", "1_create"},
			{"DoubleUnderscore", "create__user"},
			{"TrailingUnderscore", "create_user_"},
			{"LeadingUnderscore", "_create"},
			{"ContainsUppercase", "create_User"},
			{"ContainsHyphen", "create-user"},
			{"ContainsSpace", "create user"},
			{"ContainsDot", "create.user"},
			{"ContainsSlash", "create/user"},
		}

		for _, tc := range invalidActions {
			t.Run(tc.name, func(t *testing.T) {
				assert.Panics(t, func() {
					_ = crud.NewCreate[orm.Model, orm.Model]().Action(tc.action)
				}, "Should panic for invalid action name format")
			})
		}
	})
}
