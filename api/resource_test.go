package api_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/api"
)

// TestValidateActionName covers the public action-name validation contract for
// both resource kinds, including the REST sub-resource parsing edge cases.
func TestValidateActionName(t *testing.T) {
	t.Run("RPC", func(t *testing.T) {
		tests := []struct {
			name    string
			action  string
			wantErr error
		}{
			{"SingleWord", "create", nil},
			{"SnakeCase", "find_page", nil},
			{"MultiWord", "get_user_info", nil},
			{"Empty", "", api.ErrEmptyActionName},
			{"CamelCase", "findPage", api.ErrInvalidActionName},
			{"PascalCase", "CreateUser", api.ErrInvalidActionName},
			{"LeadingDigit", "1_create", api.ErrInvalidActionName},
			{"DoubleUnderscore", "create__user", api.ErrInvalidActionName},
			{"TrailingUnderscore", "create_user_", api.ErrInvalidActionName},
			{"LeadingUnderscore", "_create", api.ErrInvalidActionName},
			{"Hyphenated", "create-user", api.ErrInvalidActionName},
			{"WithSpace", "create user", api.ErrInvalidActionName},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				err := api.ValidateActionName(tc.action, api.KindRPC)
				if tc.wantErr == nil {
					assert.NoError(t, err, "expected %q to be a valid RPC action", tc.action)

					return
				}

				require.Error(t, err, "expected %q to be an invalid RPC action", tc.action)
				assert.ErrorIs(t, err, tc.wantErr, "error should wrap the expected sentinel for %q", tc.action)
			})
		}
	})

	t.Run("REST", func(t *testing.T) {
		tests := []struct {
			name    string
			action  string
			wantErr error
		}{
			{"VerbOnly", "get", nil},
			{"VerbAll", "all", nil},
			{"VerbWithSubResource", "post user-friends", nil},
			{"VerbWithNestedSubResource", "get sys/data-dict", nil},
			{"Empty", "", api.ErrEmptyActionName},
			{"UnknownVerb", "fetch", api.ErrInvalidActionName},
			{"UnknownVerbWithSubResource", "fetch user", api.ErrInvalidActionName},
			{"ThreeParts", "get user friends", api.ErrInvalidActionName},
			{"MultiSpaceBetween", "get  user", api.ErrInvalidActionName},
			{"TrailingSpace", "get ", api.ErrInvalidActionName},
			{"SnakeCaseSubResource", "get user_friends", api.ErrInvalidActionName},
			{"UppercaseSubResource", "get User", api.ErrInvalidActionName},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				err := api.ValidateActionName(tc.action, api.KindREST)
				if tc.wantErr == nil {
					assert.NoError(t, err, "expected %q to be a valid REST action", tc.action)

					return
				}

				require.Error(t, err, "expected %q to be an invalid REST action", tc.action)
				assert.ErrorIs(t, err, tc.wantErr, "error should wrap the expected sentinel for %q", tc.action)
			})
		}
	})

	t.Run("UnknownKind", func(t *testing.T) {
		err := api.ValidateActionName("create", api.Kind(0))
		require.Error(t, err, "unknown kind should be rejected")
		assert.ErrorIs(t, err, api.ErrInvalidResourceKind, "error should wrap ErrInvalidResourceKind")
	})
}

// recoverErr runs fn and returns the error it panics with, or nil if it does
// not panic. The resource constructors panic with the validation error, which
// is the only way to drive the unexported validate/validateVersion paths.
func recoverErr(t *testing.T, fn func()) (err error) {
	t.Helper()

	defer func() {
		if r := recover(); r != nil {
			recovered, ok := r.(error)
			require.True(t, ok, "panic value should be an error, got %T", r)

			err = recovered
		}
	}()

	fn()

	return nil
}

// TestNewResourceNameValidation drives the unexported name rules through the
// public RPC/REST constructors, asserting the panic carries the right sentinel.
func TestNewResourceNameValidation(t *testing.T) {
	tests := []struct {
		name    string
		kind    api.Kind
		resName string
		wantErr error
	}{
		{"RPCValid", api.KindRPC, "user", nil},
		{"RPCValidNested", api.KindRPC, "sys/data_dict", nil},
		{"RESTValid", api.KindREST, "user", nil},
		{"RESTValidNested", api.KindREST, "sys/data-dict", nil},
		{"Empty", api.KindRPC, "", api.ErrEmptyResourceName},
		{"LeadingSlash", api.KindRPC, "/user", api.ErrResourceNameSlash},
		{"TrailingSlash", api.KindRPC, "user/", api.ErrResourceNameSlash},
		{"DoubleSlash", api.KindRPC, "sys//user", api.ErrResourceNameDoubleSlash},
		{"RPCHyphenInvalid", api.KindRPC, "data-dict", api.ErrInvalidResourceName},
		{"RPCUppercaseInvalid", api.KindRPC, "User", api.ErrInvalidResourceName},
		{"RESTUnderscoreInvalid", api.KindREST, "data_dict", api.ErrInvalidResourceName},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			construct := func() {
				switch tc.kind {
				case api.KindRPC:
					api.NewRPCResource(tc.resName)
				case api.KindREST:
					api.NewRESTResource(tc.resName)
				}
			}

			err := recoverErr(t, construct)
			if tc.wantErr == nil {
				assert.NoError(t, err, "expected %q to construct without panic", tc.resName)

				return
			}

			require.Error(t, err, "expected %q to panic during construction", tc.resName)
			assert.ErrorIs(t, err, tc.wantErr, "panic error should wrap the expected sentinel for %q", tc.resName)
		})
	}
}

// TestNewResourceVersionValidation drives the unexported version rule through
// WithVersion on the public constructor.
func TestNewResourceVersionValidation(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantErr error
	}{
		{"DefaultEmpty", "", nil},
		{"V1", "v1", nil},
		{"V10", "v10", nil},
		{"MissingPrefix", "1", api.ErrInvalidVersionFormat},
		{"NonNumeric", "vx", api.ErrInvalidVersionFormat},
		{"Uppercase", "V1", api.ErrInvalidVersionFormat},
		{"WithSuffix", "v1beta", api.ErrInvalidVersionFormat},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := recoverErr(t, func() {
				api.NewRPCResource("user", api.WithVersion(tc.version))
			})
			if tc.wantErr == nil {
				assert.NoError(t, err, "expected version %q to be accepted", tc.version)

				return
			}

			require.Error(t, err, "expected version %q to panic", tc.version)
			assert.ErrorIs(t, err, tc.wantErr, "panic error should wrap ErrInvalidVersionFormat for %q", tc.version)
		})
	}
}

// TestNewResourceOperationValidation ensures operation specs are validated
// against the resource kind during construction.
func TestNewResourceOperationValidation(t *testing.T) {
	t.Run("EmptyActionRejected", func(t *testing.T) {
		err := recoverErr(t, func() {
			api.NewRPCResource("user", api.WithOperations(api.OperationSpec{Action: ""}))
		})
		require.Error(t, err, "empty operation action should panic")
		assert.ErrorIs(t, err, api.ErrEmptyActionName, "panic error should wrap ErrEmptyActionName")
	})

	t.Run("KindMismatchRejected", func(t *testing.T) {
		// A hyphenated action is valid for REST but not for RPC snake_case.
		err := recoverErr(t, func() {
			api.NewRPCResource("user", api.WithOperations(api.OperationSpec{Action: "create-user"}))
		})
		require.Error(t, err, "RPC resource should reject a hyphenated action")
		assert.ErrorIs(t, err, api.ErrInvalidActionName, "panic error should wrap ErrInvalidActionName")
	})

	t.Run("ValidConstructs", func(t *testing.T) {
		err := recoverErr(t, func() {
			api.NewRPCResource("user", api.WithOperations(api.OperationSpec{Action: "create"}))
		})
		assert.NoError(t, err, "a valid RPC resource should construct without panic")
	})
}

// TestKindString covers the Kind stringer including the unknown fallback.
func TestKindString(t *testing.T) {
	tests := []struct {
		kind api.Kind
		want string
	}{
		{api.KindRPC, "rpc"},
		{api.KindREST, "rest"},
		{api.Kind(0), "unknown"},
		{api.Kind(99), "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.kind.String(), "Kind(%d).String()", tc.kind)
		})
	}
}

// errSentinel is used to confirm ValidateActionName errors remain comparable.
var errSentinel = errors.New("sentinel")

func TestValidateActionNameErrorsAreWrapped(t *testing.T) {
	// Guard against a regression where the sentinels stop wrapping: a random
	// unrelated sentinel must never match.
	err := api.ValidateActionName("Bad Action", api.KindRPC)
	require.Error(t, err, "invalid action should error")
	assert.NotErrorIs(t, err, errSentinel, "unrelated sentinel must not match")
}
