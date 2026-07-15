// The validation tests exercise the interplay with the real auth schemes, so
// they live in the external test package to keep definition free of an auth
// import cycle.
package definition_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/auth"
	"github.com/coldsmirk/vef-framework-go/internal/integration/definition"
)

// plainCodec builds a key-less codec for validation tests.
func plainCodec(t *testing.T) *definition.SecretCodec {
	t.Helper()

	codec, err := definition.NewSecretCodec(new(config.IntegrationConfig))
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
			err := definition.ValidateContract(&tt.contract)

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
		assert.NoError(t, definition.ValidateAdapterScript("return { ok: true }"), "Valid script should compile")
	})

	t.Run("TopLevelReturnIsSupported", func(t *testing.T) {
		assert.NoError(t, definition.ValidateAdapterScript("if (input) { return input } return null"),
			"The function wrapper should make top-level return legal")
	})

	t.Run("SyntaxErrorFails", func(t *testing.T) {
		err := definition.ValidateAdapterScript("return {")
		require.Error(t, err, "Broken script should be rejected")
		assert.ErrorIs(t, err, integration.ErrInvalidScript(""), "Error should carry the invalid-script code")
	})
}

func TestValidateSystem(t *testing.T) {
	registry := auth.NewOutboundRegistry(nil)
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
				BaseURL:      "https://his.example.com",
				OutboundAuth: &integration.OutboundAuthConfig{Scheme: auth.OutboundSchemeBearer, Params: map[string]string{"token": "t"}},
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
				BaseURL:      "https://his.example.com",
				OutboundAuth: &integration.OutboundAuthConfig{Scheme: "kerberos"},
			},
			wantErr: integration.ErrUnknownAuthScheme("kerberos"),
		},
		{
			name: "MissingSchemeParamFails",
			system: integration.System{
				BaseURL:      "https://his.example.com",
				OutboundAuth: &integration.OutboundAuthConfig{Scheme: auth.OutboundSchemeBasic, Params: map[string]string{"username": "u"}},
			},
			wantErr: integration.ErrInvalidAuthParams(""),
		},
		{
			name: "ReadWriteDataSourcePasses",
			system: integration.System{
				DataSource: &integration.DataSourceConfig{Kind: config.SQLite, Mode: integration.DataSourceModeReadWrite},
			},
		},
		{
			name: "UnknownDataSourceModeFails",
			system: integration.System{
				DataSource: &integration.DataSourceConfig{Kind: config.SQLite, Mode: "admin"},
			},
			wantErr: integration.ErrInvalidDataSource(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := definition.ValidateSystem(registry, codec, &tt.system)

			if tt.wantErr == nil {
				assert.NoError(t, err, "System should validate")

				return
			}

			require.Error(t, err, "System should be rejected")
			assert.ErrorIs(t, err, tt.wantErr, "Error should match the expected sentinel/code")
		})
	}
}
