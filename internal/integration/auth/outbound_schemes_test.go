package auth

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/hashx"
	"github.com/coldsmirk/vef-framework-go/httpx"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/js"
	"github.com/coldsmirk/vef-framework-go/js/jscrypto"
	"github.com/coldsmirk/vef-framework-go/security"
)

// newTestEngine builds a bare engine carrying the crypto base lib, mirroring
// the production baseline signing scripts rely on.
func newTestEngine(t *testing.T) *js.Engine {
	t.Helper()

	engine, err := js.NewEngine(js.WithoutStdLibs(), js.WithBaseLibs(jscrypto.New()))
	require.NoError(t, err, "Engine construction should succeed")

	return engine
}

func newTestRegistry(t *testing.T) *OutboundRegistry {
	t.Helper()

	return NewOutboundRegistry(newTestEngine(t), new(config.IntegrationConfig), nil)
}

// applyScheme runs one request through a client configured by the scheme and
// returns what the server observed.
func applyScheme(t *testing.T, scheme integration.OutboundAuthScheme, cfg *integration.OutboundAuthConfig) *http.Request {
	t.Helper()

	var observed *http.Request

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clone := *r
		clone.URL = r.URL
		observed = &clone

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	opts, err := scheme.Apply(cfg)
	require.NoError(t, err, "Scheme should accept its config")

	client, err := httpx.New(append(opts, httpx.WithBaseURL(srv.URL))...)
	require.NoError(t, err, "Client construction should succeed")

	_, err = client.NewRequest().Get(t.Context(), "/probe")
	require.NoError(t, err, "Probe request should succeed")
	require.NotNil(t, observed, "Server should observe the request")

	return observed
}

func TestBuiltinSchemes(t *testing.T) {
	registry := newTestRegistry(t)

	t.Run("None", func(t *testing.T) {
		scheme, ok := registry.Get(OutboundSchemeNone)
		require.True(t, ok, "none scheme should be registered")

		req := applyScheme(t, scheme, new(integration.OutboundAuthConfig))
		assert.Empty(t, req.Header.Get("Authorization"), "none scheme should send no credentials")
		assert.Empty(t, scheme.SensitiveParams(), "none scheme should declare no sensitive params")
	})

	t.Run("HTTPBasic", func(t *testing.T) {
		scheme, ok := registry.Get(OutboundSchemeHTTPBasic)
		require.True(t, ok, "http_basic scheme should be registered")

		req := applyScheme(t, scheme, &integration.OutboundAuthConfig{
			Params: map[string]string{"username": "u", "password": "p"},
		})
		user, pass, hasAuth := req.BasicAuth()
		require.True(t, hasAuth, "http_basic scheme should send Basic credentials")
		assert.Equal(t, "u", user, "Username should reach the server")
		assert.Equal(t, "p", pass, "Password should reach the server")
		assert.Equal(t, []string{"password"}, scheme.SensitiveParams(), "Only the password should be sensitive")
	})

	t.Run("Bearer", func(t *testing.T) {
		scheme, ok := registry.Get(OutboundSchemeBearer)
		require.True(t, ok, "bearer scheme should be registered")

		req := applyScheme(t, scheme, &integration.OutboundAuthConfig{Params: map[string]string{"token": "tok"}})
		assert.Equal(t, "Bearer tok", req.Header.Get("Authorization"), "Bearer token should reach the server")
		assert.Equal(t, []string{"token"}, scheme.SensitiveParams(), "The token should be sensitive")
	})

	t.Run("HeaderSendsEveryPair", func(t *testing.T) {
		scheme, ok := registry.Get(OutboundSchemeHeader)
		require.True(t, ok, "header scheme should be registered")

		req := applyScheme(t, scheme, &integration.OutboundAuthConfig{
			Params: map[string]string{"X-App-Id": "app-1", "X-App-Secret": "s3"},
		})
		assert.Equal(t, "app-1", req.Header.Get("X-App-Id"), "First credential header should reach the server")
		assert.Equal(t, "s3", req.Header.Get("X-App-Secret"), "Second credential header should reach the server")
		assert.Equal(t, []string{integration.SensitiveAll}, scheme.SensitiveParams(),
			"User-defined header names force the sensitive-all wildcard")
	})

	t.Run("QuerySendsEveryPair", func(t *testing.T) {
		scheme, ok := registry.Get(OutboundSchemeQuery)
		require.True(t, ok, "query scheme should be registered")

		req := applyScheme(t, scheme, &integration.OutboundAuthConfig{
			Params: map[string]string{"appid": "app-1", "token": "t0k"},
		})
		assert.Equal(t, "app-1", req.URL.Query().Get("appid"), "First credential parameter should reach the server")
		assert.Equal(t, "t0k", req.URL.Query().Get("token"), "Second credential parameter should reach the server")
		assert.Equal(t, []string{integration.SensitiveAll}, scheme.SensitiveParams(),
			"User-defined parameter names force the sensitive-all wildcard")
	})

	t.Run("SignatureSignsMethodAndPath", func(t *testing.T) {
		scheme, ok := registry.Get(OutboundSchemeSignature)
		require.True(t, ok, "signature scheme should be registered")

		const secret = "aabbccdd"

		req := applyScheme(t, scheme, &integration.OutboundAuthConfig{
			Params: map[string]string{"appId": "his-01", "secret": secret},
		})

		timestamp, err := strconv.ParseInt(req.Header.Get("X-Timestamp"), 10, 64)
		require.NoError(t, err, "The signed request should carry a numeric timestamp")

		verifier, err := security.NewSignature("00")
		require.NoError(t, err, "Verifier construction should succeed")

		err = verifier.VerifyWithSecret(t.Context(), secret,
			security.SignatureRequest{AppID: "his-01", Method: http.MethodGet, Path: "/probe"},
			security.SignatureCredentials{
				Timestamp: timestamp,
				Nonce:     req.Header.Get("X-Nonce"),
				Signature: req.Header.Get("X-Signature"),
			})
		assert.NoError(t, err, "The inbound-side verifier should accept the outbound-signed request")
		assert.Equal(t, []string{"secret"}, scheme.SensitiveParams(), "The secret should be sensitive")
	})

	t.Run("ScriptSignsFromRequestAndParams", func(t *testing.T) {
		scheme, ok := registry.Get(OutboundSchemeScript)
		require.True(t, ok, "script scheme should be registered")

		req := applyScheme(t, scheme, &integration.OutboundAuthConfig{
			Params: map[string]string{"secret": "s3cr3t"},
			Script: `return { 'X-Sign': crypto.md5(request.method + request.path + params.secret) }`,
		})

		assert.Equal(t, hashx.MD5(http.MethodGet+"/probe"+"s3cr3t"), req.Header.Get("X-Sign"),
			"The signing script should see the built request and the decrypted params")
		assert.Equal(t, []string{integration.SensitiveAll}, scheme.SensitiveParams(),
			"User-defined parameter names force the sensitive-all wildcard")
	})
}

func TestScriptSchemeFailure(t *testing.T) {
	registry := newTestRegistry(t)

	scheme, ok := registry.Get(OutboundSchemeScript)
	require.True(t, ok, "script scheme should be registered")

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()

	newClient := func(script string) *httpx.Client {
		opts, err := scheme.Apply(&integration.OutboundAuthConfig{Script: script})
		require.NoError(t, err, "Scheme should accept a compilable script")

		client, err := httpx.New(append(opts, httpx.WithBaseURL(srv.URL))...)
		require.NoError(t, err, "Client construction should succeed")

		return client
	}

	t.Run("ThrowCarriesTheAuthMarker", func(t *testing.T) {
		_, err := newClient(`throw new Error('vault unreachable')`).NewRequest().Get(t.Context(), "/probe")
		require.Error(t, err, "A throwing signing script should fail the request")

		var authErr *OutboundAuthError
		require.ErrorAs(t, err, &authErr, "The failure should carry the outbound-auth marker")
		assert.Contains(t, authErr.Error(), "vault unreachable", "The script's own message should surface")
	})

	t.Run("NonObjectReturnIsRejected", func(t *testing.T) {
		_, err := newClient(`return 42`).NewRequest().Get(t.Context(), "/probe")
		require.Error(t, err, "A non-object return should fail the request")

		var authErr *OutboundAuthError
		require.ErrorAs(t, err, &authErr, "The failure should carry the outbound-auth marker")
		assert.Contains(t, authErr.Error(), "header object", "The error should state the return contract")
	})

	t.Run("BrokenScriptFailsAtApply", func(t *testing.T) {
		_, err := scheme.Apply(&integration.OutboundAuthConfig{Script: "return {"})
		require.Error(t, err, "A script that does not compile should fail Apply")
	})
}

func TestSchemeParamValidation(t *testing.T) {
	registry := newTestRegistry(t)

	tests := []struct {
		name   string
		scheme string
		cfg    *integration.OutboundAuthConfig
	}{
		{name: "HTTPBasicMissingUsername", scheme: OutboundSchemeHTTPBasic, cfg: &integration.OutboundAuthConfig{Params: map[string]string{"password": "p"}}},
		{name: "HTTPBasicMissingPassword", scheme: OutboundSchemeHTTPBasic, cfg: &integration.OutboundAuthConfig{Params: map[string]string{"username": "u"}}},
		{name: "BearerMissingToken", scheme: OutboundSchemeBearer, cfg: new(integration.OutboundAuthConfig)},
		{name: "HeaderWithoutPairs", scheme: OutboundSchemeHeader, cfg: new(integration.OutboundAuthConfig)},
		{name: "QueryWithoutPairs", scheme: OutboundSchemeQuery, cfg: new(integration.OutboundAuthConfig)},
		{name: "SignatureMissingAppID", scheme: OutboundSchemeSignature, cfg: &integration.OutboundAuthConfig{Params: map[string]string{"secret": "aabb"}}},
		{name: "SignatureMissingSecret", scheme: OutboundSchemeSignature, cfg: &integration.OutboundAuthConfig{Params: map[string]string{"appId": "a"}}},
		{name: "ScriptMissingBody", scheme: OutboundSchemeScript, cfg: new(integration.OutboundAuthConfig)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme, ok := registry.Get(tt.scheme)
			require.True(t, ok, "Scheme should be registered")

			_, err := scheme.Apply(tt.cfg)
			require.Error(t, err, "Missing required configuration should be rejected")
			assert.ErrorIs(t, err, ErrMissingParam, "Error should be the missing-param sentinel")
		})
	}
}

// OverrideScheme replaces the built-in bearer scheme in registry tests.
type OverrideScheme struct{}

func (*OverrideScheme) Name() string { return OutboundSchemeBearer }

func (*OverrideScheme) Apply(*integration.OutboundAuthConfig) ([]httpx.Option, error) {
	return nil, nil
}

func (*OverrideScheme) SensitiveParams() []string { return nil }

func TestRegistry(t *testing.T) {
	engine := newTestEngine(t)

	t.Run("ResolveNilAuthYieldsNone", func(t *testing.T) {
		scheme, ok := NewOutboundRegistry(engine, new(config.IntegrationConfig), nil).Resolve(nil)
		require.True(t, ok, "Nil auth should resolve")
		assert.Equal(t, OutboundSchemeNone, scheme.Name(), "Nil auth should resolve to the none scheme")
	})

	t.Run("ResolveEmptySchemeYieldsNone", func(t *testing.T) {
		scheme, ok := NewOutboundRegistry(engine, new(config.IntegrationConfig), nil).Resolve(&integration.OutboundAuthConfig{})
		require.True(t, ok, "Empty scheme should resolve")
		assert.Equal(t, OutboundSchemeNone, scheme.Name(), "Empty scheme should resolve to the none scheme")
	})

	t.Run("UnknownSchemeReportsNotOK", func(t *testing.T) {
		_, ok := NewOutboundRegistry(engine, new(config.IntegrationConfig), nil).Resolve(&integration.OutboundAuthConfig{Scheme: "kerberos"})
		assert.False(t, ok, "Unknown scheme should not resolve")
	})

	t.Run("ApplicationSchemeOverridesBuiltin", func(t *testing.T) {
		registry := NewOutboundRegistry(engine, new(config.IntegrationConfig), []integration.OutboundAuthScheme{new(OverrideScheme)})

		scheme, ok := registry.Get(OutboundSchemeBearer)
		require.True(t, ok, "Overridden scheme should stay registered")
		assert.IsType(t, new(OverrideScheme), scheme, "Application scheme should replace the built-in by name")
	})
}

// TestValidateOutboundAuth tests the structural script/scheme match check.
func TestValidateOutboundAuth(t *testing.T) {
	t.Run("NilConfigPasses", func(t *testing.T) {
		assert.NoError(t, ValidateOutboundAuth(nil), "Nil auth should validate")
	})

	t.Run("ScriptOnScriptSchemePasses", func(t *testing.T) {
		cfg := &integration.OutboundAuthConfig{Scheme: OutboundSchemeScript, Script: "return {}"}
		assert.NoError(t, ValidateOutboundAuth(cfg), "A script on the script scheme should validate")
	})

	t.Run("ScriptOnOtherSchemeFails", func(t *testing.T) {
		cfg := &integration.OutboundAuthConfig{Scheme: OutboundSchemeBearer, Script: "return {}"}
		err := ValidateOutboundAuth(cfg)
		require.Error(t, err, "A script on a non-script scheme should be rejected")
		assert.ErrorIs(t, err, integration.ErrInvalidAuthParams(""), "Error should carry the invalid-auth-params code")
	})
}
