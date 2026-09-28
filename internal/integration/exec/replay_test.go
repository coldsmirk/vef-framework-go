package exec

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/definition"
)

// newSealingInvoker builds an invoker carrying only what sealing a replay
// payload reads: the log configuration and the secret codec.
func newSealingInvoker(t *testing.T, cfg *config.IntegrationConfig) *Invoker {
	t.Helper()

	codec, err := definition.NewSecretCodec(cfg)
	require.NoError(t, err, "The secret codec should build")

	return &Invoker{cfg: cfg, codec: codec}
}

func TestSealReplay(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32))
	payload := &ReplayPayload{Input: map[string]any{"password": "s3cret"}}

	t.Run("OffKeepsNothing", func(t *testing.T) {
		inv := newSealingInvoker(t, &config.IntegrationConfig{SecretKey: key})

		assert.Empty(t, inv.sealReplay(&outcome{replay: payload}), "Replay off should keep no payload")
	})

	t.Run("SealedWithTheSecretKey", func(t *testing.T) {
		inv := newSealingInvoker(t, &config.IntegrationConfig{SecretKey: key, Log: config.IntegrationLogConfig{Replay: true}})

		sealed := inv.sealReplay(&outcome{replay: payload})
		require.True(t, strings.HasPrefix(sealed, "enc:"), "The payload should be sealed")
		assert.NotContains(t, sealed, "s3cret", "The sealed payload should not expose the input")

		plain, err := inv.codec.DecryptValue(sealed)
		require.NoError(t, err, "The sealed payload should open")
		assert.JSONEq(t, `{"input":{"password":"s3cret"}}`, plain, "The payload should round-trip losslessly")
	})

	t.Run("PlaintextWithoutKey", func(t *testing.T) {
		inv := newSealingInvoker(t, &config.IntegrationConfig{Log: config.IntegrationLogConfig{Replay: true}})

		assert.JSONEq(t, `{"input":{"password":"s3cret"}}`, inv.sealReplay(&outcome{replay: payload}),
			"Without a key the payload should be kept in plaintext")
	})

	t.Run("OverLimitKeepsNothing", func(t *testing.T) {
		inv := newSealingInvoker(t, &config.IntegrationConfig{Log: config.IntegrationLogConfig{Replay: true, ReplayLimit: 8}})

		assert.Empty(t, inv.sealReplay(&outcome{replay: payload}), "A payload over the limit should not be kept")
	})

	t.Run("NoPayloadKeepsNothing", func(t *testing.T) {
		inv := newSealingInvoker(t, &config.IntegrationConfig{Log: config.IntegrationLogConfig{Replay: true}})

		assert.Empty(t, inv.sealReplay(new(outcome)), "An outcome without replay material should keep nothing")
	})
}

func TestReplayRequest(t *testing.T) {
	original := &integration.InboundRequest{
		Method: "POST",
		Path:   "/integration/inbound/lis/lab",
		Headers: map[string]string{
			"authorization": "Bearer caller-token",
			"cookie":        "session=abc",
			"x-api-key":     "key-1",
			"x-trace":       "t-1",
		},
		Query:      map[string]string{"token": "key-1", "batch": "7"},
		Body:       []byte(`{"key":"key-1","rid":"R-1"}`),
		ClientAddr: "203.0.113.7",
	}

	kept := replayRequest(original, []string{"key-1"})

	assert.NotContains(t, kept.Headers, "authorization", "Credential headers should be dropped")
	assert.NotContains(t, kept.Headers, "cookie", "Cookies should be dropped")
	assert.Equal(t, integration.MaskedSecret, kept.Headers["x-api-key"], "A verified credential value should be scrubbed from headers")
	assert.Equal(t, "t-1", kept.Headers["x-trace"], "Other headers should be kept")
	assert.Equal(t, integration.MaskedSecret, kept.Query["token"], "A verified credential value should be scrubbed from the query")
	assert.Equal(t, "7", kept.Query["batch"], "Other query parameters should be kept")
	assert.JSONEq(t, `{"key":"******","rid":"R-1"}`, string(kept.Body), "A verified credential value should be scrubbed from the body")
	assert.Equal(t, "203.0.113.7", kept.ClientAddr, "The caller address should be kept")
	assert.Equal(t, "Bearer caller-token", original.Headers["authorization"], "The delivered request should be left untouched")
	assert.Equal(t, "key-1", original.Query["token"], "The delivered query should be left untouched")
}

func TestReplayHandler(t *testing.T) {
	handler := &replayHandler{results: []DispatchResult{
		{Output: map[string]any{"accepted": true}},
		{Error: "laboratory rejected the report"},
	}}

	output, err := handler.Handle(context.Background(), nil)
	require.NoError(t, err, "The first dispatch should be answered with the recorded output")
	assert.Equal(t, map[string]any{"accepted": true}, output, "The recorded output should be returned")

	_, err = handler.Handle(context.Background(), nil)
	assert.EqualError(t, err, "laboratory rejected the report", "The recorded failure should be reproduced")

	_, err = handler.Handle(context.Background(), nil)
	assert.ErrorIs(t, err, ErrReplayDispatchUnrecorded, "A dispatch beyond the recording has no result")
}

func TestReplayPayloadEncoding(t *testing.T) {
	data, err := json.Marshal(&ReplayPayload{Input: map[string]any{"id": "1"}})
	require.NoError(t, err, "The payload should encode")

	assert.JSONEq(t, `{"input":{"id":"1"}}`, string(data), "An outbound payload should carry only its input")
}
