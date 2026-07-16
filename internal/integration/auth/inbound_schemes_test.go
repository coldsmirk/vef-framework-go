package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/integration"
)

// inboundRequest builds a minimal envelope carrying the given lowercased
// headers and query parameters.
func inboundRequest(headers, query map[string]string) *integration.InboundRequest {
	return &integration.InboundRequest{
		SystemCode:   "sys",
		ContractCode: "op",
		Protocol:     "http",
		Method:       "POST",
		Path:         "/integration/inbound/sys/op",
		Headers:      headers,
		Query:        query,
	}
}

// TestEmptyCredentialValueFailsClosed guards the fail-closed contract: a blank
// configured credential value must never authenticate an absent request
// header, even though ConstantTimeCompare("", "") reports a match.
func TestEmptyCredentialValueFailsClosed(t *testing.T) {
	scheme := new(headerInboundScheme)
	cfg := &integration.InboundAuthConfig{Params: map[string]string{"X-A": "", "X-B": "b"}}

	// The caller presents the correct X-B and nothing for X-A (absent -> "").
	req := inboundRequest(map[string]string{"x-b": "b"}, nil)

	err := scheme.Verify(t.Context(), req, cfg)
	require.Error(t, err, "An empty configured credential must not match an absent header")
	assert.ErrorIs(t, err, ErrVerificationFailed, "The rejection is the uniform verification failure")
}

// TestHeaderInboundScheme tests the multi-pair credential header verification.
func TestHeaderInboundScheme(t *testing.T) {
	scheme := new(headerInboundScheme)
	cfg := &integration.InboundAuthConfig{Params: map[string]string{"X-App-Id": "app-1", "X-App-Secret": "s3"}}

	t.Run("AllPairsMatchingPasses", func(t *testing.T) {
		req := inboundRequest(map[string]string{"x-app-id": "app-1", "x-app-secret": "s3"}, nil)
		assert.NoError(t, scheme.Verify(t.Context(), req, cfg), "Matching every configured header should pass")
	})

	t.Run("OneMismatchedPairFails", func(t *testing.T) {
		req := inboundRequest(map[string]string{"x-app-id": "app-1", "x-app-secret": "wrong"}, nil)
		err := scheme.Verify(t.Context(), req, cfg)
		require.Error(t, err, "A single mismatched header should fail the whole set")
		assert.ErrorIs(t, err, ErrVerificationFailed, "Rejection should be the uniform verification failure")
	})

	t.Run("MissingHeaderFails", func(t *testing.T) {
		req := inboundRequest(map[string]string{"x-app-id": "app-1"}, nil)
		err := scheme.Verify(t.Context(), req, cfg)
		require.Error(t, err, "An absent configured header should fail")
		assert.ErrorIs(t, err, ErrVerificationFailed, "Rejection should be the uniform verification failure")
	})

	t.Run("EmptyPairSetIsConfigFault", func(t *testing.T) {
		req := inboundRequest(map[string]string{"x-app-id": "app-1"}, nil)
		err := scheme.Verify(t.Context(), req, new(integration.InboundAuthConfig))
		require.Error(t, err, "A header scheme without pairs must not allow anything")
		assert.ErrorIs(t, err, ErrMissingParam, "An empty pair set should classify as a config fault")
	})

	t.Run("SensitivityIsWildcard", func(t *testing.T) {
		assert.Equal(t, []string{integration.SensitiveAll}, scheme.SensitiveParams(),
			"User-defined header names force the sensitive-all wildcard")
	})
}

// TestQueryInboundScheme tests the multi-pair credential query verification.
func TestQueryInboundScheme(t *testing.T) {
	scheme := new(queryInboundScheme)
	cfg := &integration.InboundAuthConfig{Params: map[string]string{"appid": "app-1", "token": "t0k"}}

	t.Run("AllPairsMatchingPasses", func(t *testing.T) {
		req := inboundRequest(nil, map[string]string{"appid": "app-1", "token": "t0k"})
		assert.NoError(t, scheme.Verify(t.Context(), req, cfg), "Matching every configured parameter should pass")
	})

	t.Run("OneMismatchedPairFails", func(t *testing.T) {
		req := inboundRequest(nil, map[string]string{"appid": "app-1", "token": "wrong"})
		err := scheme.Verify(t.Context(), req, cfg)
		require.Error(t, err, "A single mismatched parameter should fail the whole set")
		assert.ErrorIs(t, err, ErrVerificationFailed, "Rejection should be the uniform verification failure")
	})

	t.Run("EmptyPairSetIsConfigFault", func(t *testing.T) {
		req := inboundRequest(nil, map[string]string{"appid": "app-1"})
		err := scheme.Verify(t.Context(), req, new(integration.InboundAuthConfig))
		require.Error(t, err, "A query scheme without pairs must not allow anything")
		assert.ErrorIs(t, err, ErrMissingParam, "An empty pair set should classify as a config fault")
	})
}

// TestBearerInboundScheme tests the static bearer token verification.
func TestBearerInboundScheme(t *testing.T) {
	scheme := new(bearerInboundScheme)
	cfg := &integration.InboundAuthConfig{Params: map[string]string{"token": "tok-1"}}

	t.Run("MatchingTokenPasses", func(t *testing.T) {
		req := inboundRequest(map[string]string{"authorization": "Bearer tok-1"}, nil)
		assert.NoError(t, scheme.Verify(t.Context(), req, cfg), "A matching bearer token should pass")
	})

	t.Run("SchemePrefixIsCaseInsensitive", func(t *testing.T) {
		req := inboundRequest(map[string]string{"authorization": "bearer tok-1"}, nil)
		assert.NoError(t, scheme.Verify(t.Context(), req, cfg), "The Bearer prefix should match case-insensitively")
	})

	t.Run("WrongTokenFails", func(t *testing.T) {
		req := inboundRequest(map[string]string{"authorization": "Bearer nope"}, nil)
		err := scheme.Verify(t.Context(), req, cfg)
		require.Error(t, err, "A wrong token should fail")
		assert.ErrorIs(t, err, ErrVerificationFailed, "Rejection should be the uniform verification failure")
	})

	t.Run("MissingHeaderFails", func(t *testing.T) {
		req := inboundRequest(nil, nil)
		err := scheme.Verify(t.Context(), req, cfg)
		require.Error(t, err, "An absent Authorization header should fail")
		assert.ErrorIs(t, err, ErrVerificationFailed, "Rejection should be the uniform verification failure")
	})

	t.Run("MissingTokenParamIsConfigFault", func(t *testing.T) {
		req := inboundRequest(map[string]string{"authorization": "Bearer tok-1"}, nil)
		err := scheme.Verify(t.Context(), req, new(integration.InboundAuthConfig))
		require.Error(t, err, "A bearer scheme without a token must not allow anything")
		assert.ErrorIs(t, err, ErrMissingParam, "A missing token param should classify as a config fault")
	})
}
