package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/auth"
)

// plainCodec builds a key-less codec for validation tests.
func plainCodec(t *testing.T) *SecretCodec {
	t.Helper()

	codec, err := NewSecretCodec(new(config.IntegrationConfig))
	require.NoError(t, err, "Codec construction should succeed")

	return codec
}

func TestValidateContract(t *testing.T) {
	valid := json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`)

	tests := []struct {
		name     string
		contract integration.Contract
		wantCode int
	}{
		{name: "EmptySchemasPass", contract: integration.Contract{}},
		{name: "ValidSchemasPass", contract: integration.Contract{InputSchema: valid, OutputSchema: valid}},
		{
			name:     "MalformedJSONFails",
			contract: integration.Contract{InputSchema: json.RawMessage(`{`)},
			wantCode: integration.ErrCodeInvalidSchema,
		},
		{
			name:     "RemoteRefFails",
			contract: integration.Contract{OutputSchema: json.RawMessage(`{"$ref":"https://example.com/schema.json"}`)},
			wantCode: integration.ErrCodeInvalidSchema,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContract(&tt.contract)

			if tt.wantCode == 0 {
				assert.NoError(t, err, "Contract should validate")

				return
			}

			require.Error(t, err, "Contract should be rejected")
			assert.ErrorIs(t, err, integration.ErrInvalidSchema(""), "Error should carry the invalid-schema code")
		})
	}
}

func TestValidateAdapterScript(t *testing.T) {
	t.Run("ValidScriptCompiles", func(t *testing.T) {
		assert.NoError(t, ValidateAdapterScript("return { ok: true }"), "Valid script should compile")
	})

	t.Run("TopLevelReturnIsSupported", func(t *testing.T) {
		assert.NoError(t, ValidateAdapterScript("if (input) { return input } return null"),
			"The function wrapper should make top-level return legal")
	})

	t.Run("SyntaxErrorFails", func(t *testing.T) {
		err := ValidateAdapterScript("return {")
		require.Error(t, err, "Broken script should be rejected")
		assert.ErrorIs(t, err, integration.ErrInvalidScript(""), "Error should carry the invalid-script code")
	})
}

func TestValidateSystem(t *testing.T) {
	registry := auth.NewRegistry(nil)
	codec := plainCodec(t)

	tests := []struct {
		name    string
		system  integration.System
		wantErr error
	}{
		{name: "NoAuthPasses", system: integration.System{BaseURL: "https://his.example.com"}},
		{
			name: "ValidAuthPasses",
			system: integration.System{
				BaseURL: "https://his.example.com",
				Auth:    &integration.AuthConfig{Scheme: auth.SchemeBearer, Params: map[string]string{"token": "t"}},
			},
		},
		{
			name:    "RelativeBaseURLFails",
			system:  integration.System{BaseURL: "his.example.com/api"},
			wantErr: integration.ErrInvalidBaseURL,
		},
		{
			name: "UnknownSchemeFails",
			system: integration.System{
				BaseURL: "https://his.example.com",
				Auth:    &integration.AuthConfig{Scheme: "kerberos"},
			},
			wantErr: integration.ErrUnknownAuthScheme("kerberos"),
		},
		{
			name: "MissingSchemeParamFails",
			system: integration.System{
				BaseURL: "https://his.example.com",
				Auth:    &integration.AuthConfig{Scheme: auth.SchemeBasic, Params: map[string]string{"username": "u"}},
			},
			wantErr: integration.ErrInvalidAuthParams(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSystem(registry, codec, &tt.system)

			if tt.wantErr == nil {
				assert.NoError(t, err, "System should validate")

				return
			}

			require.Error(t, err, "System should be rejected")
			assert.ErrorIs(t, err, tt.wantErr, "Error should match the expected sentinel/code")
		})
	}
}
