package api_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/api"
)

// TestParamsUnmarshalJSON covers the number-preserving JSON parsing of Params:
// numeric values must arrive as json.Number at every nesting level, survive
// digit-exact into json.RawMessage captures, and keep encoding/json's map
// semantics (merge into an existing map, null resets it, invalid input fails).
func TestParamsUnmarshalJSON(t *testing.T) {
	t.Run("NumbersArriveAsJSONNumber", func(t *testing.T) {
		var params api.Params

		payload := []byte(`{"count":9007199254740993,"nested":{"ratio":1.5},"list":[2,{"n":3}]}`)
		require.NoError(t, json.Unmarshal(payload, &params), "Unmarshal should succeed")

		assert.Equal(t, json.Number("9007199254740993"), params["count"], "top-level number should be json.Number")

		nested, ok := params["nested"].(map[string]any)
		require.True(t, ok, "nested object should decode as map[string]any")
		assert.Equal(t, json.Number("1.5"), nested["ratio"], "nested number should be json.Number")

		list, ok := params["list"].([]any)
		require.True(t, ok, "array should decode as []any")
		assert.Equal(t, json.Number("2"), list[0], "array scalar should be json.Number")

		elem, ok := list[1].(map[string]any)
		require.True(t, ok, "array object element should decode as map[string]any")
		assert.Equal(t, json.Number("3"), elem["n"], "number inside array object should be json.Number")
	})

	t.Run("RequestLevelNesting", func(t *testing.T) {
		// Unmarshaling a whole Request (the same mechanism Fiber's default
		// JSON binder uses) must route Params and Meta through their
		// number-preserving UnmarshalJSON.
		var req api.Request

		payload := []byte(`{"resource":"demo","action":"echo","version":"v1",` +
			`"params":{"id":9007199254740993},"meta":{"page":2}}`)
		require.NoError(t, json.Unmarshal(payload, &req), "Unmarshal should succeed")

		assert.Equal(t, json.Number("9007199254740993"), req.Params["id"], "params number should be json.Number")
		assert.Equal(t, json.Number("2"), req.Meta["page"], "meta number should be json.Number")
	})

	t.Run("BigIntegerSurvivesToRawMessage", func(t *testing.T) {
		var params api.Params

		payload := []byte(`{"payload":{"n":9007199254740993},"count":9007199254740993}`)
		require.NoError(t, json.Unmarshal(payload, &params), "Unmarshal should succeed")

		var out struct {
			Payload json.RawMessage `json:"payload"`
			Count   int64           `json:"count"`
		}

		require.NoError(t, params.Decode(&out), "Decode should succeed")
		assert.Contains(t, string(out.Payload), "9007199254740993", "RawMessage capture must keep exact digits")
		assert.Equal(t, int64(9007199254740993), out.Count, "int64 field must keep exact digits")
	})

	t.Run("TypedDecode", func(t *testing.T) {
		var params api.Params

		payload := []byte(`{"count":42,"ratio":0.5,"name":"demo","values":{"a":1.5}}`)
		require.NoError(t, json.Unmarshal(payload, &params), "Unmarshal should succeed")

		var out struct {
			Count  int            `json:"count"`
			Ratio  float64        `json:"ratio"`
			Name   string         `json:"name"`
			Values map[string]any `json:"values"`
		}

		require.NoError(t, params.Decode(&out), "Decode should succeed")
		assert.Equal(t, 42, out.Count, "int field should bind as before")
		assert.Equal(t, 0.5, out.Ratio, "float field should bind as before")
		assert.Equal(t, "demo", out.Name, "string field should bind as before")
		assert.Equal(t, 1.5, out.Values["a"], "untyped map values must surface as float64, not json.Number")
	})

	t.Run("MergesIntoPreinitializedMap", func(t *testing.T) {
		params := api.Params{"pre": "kept"}

		require.NoError(t, json.Unmarshal([]byte(`{"a":1}`), &params), "Unmarshal should succeed")
		assert.Equal(t, "kept", params["pre"], "existing entries should be preserved")
		assert.Equal(t, json.Number("1"), params["a"], "new entries should be added")
	})

	t.Run("NullResetsMap", func(t *testing.T) {
		params := api.Params{"pre": "x"}

		require.NoError(t, json.Unmarshal([]byte(`null`), &params), "Unmarshal of null should succeed")
		assert.Nil(t, params, "JSON null should reset the map to nil, matching encoding/json semantics")
	})

	t.Run("InvalidJSONFails", func(t *testing.T) {
		var params api.Params

		assert.Error(t, json.Unmarshal([]byte(`{"a":`), &params), "truncated JSON should fail")
		assert.Error(t, json.Unmarshal([]byte(`"text"`), &params), "non-object JSON should fail")
	})
}

// TestMetaUnmarshalJSON confirms Meta shares the number-preserving parsing.
func TestMetaUnmarshalJSON(t *testing.T) {
	var meta api.Meta

	require.NoError(t, json.Unmarshal([]byte(`{"page":9007199254740993}`), &meta), "Unmarshal should succeed")
	assert.Equal(t, json.Number("9007199254740993"), meta["page"], "meta number should be json.Number")

	var out struct {
		Page int64 `json:"page"`
	}

	require.NoError(t, meta.Decode(&out), "Decode should succeed")
	assert.Equal(t, int64(9007199254740993), out.Page, "meta int64 field must keep exact digits")
}

func TestParamsDecodeReportingUnmapped(t *testing.T) {
	t.Run("ReportsKeysTheTargetDeclaresNoFieldFor", func(t *testing.T) {
		params := api.Params{"known": "value", "retired": true}

		var out struct {
			Known string `json:"known"`
		}

		unmapped, err := params.DecodeReportingUnmapped(&out)
		require.NoError(t, err, "An undeclared key must not fail decoding")
		assert.Equal(t, []string{"retired"}, unmapped, "The undeclared key must be reported")
		assert.Equal(t, "value", out.Known, "The declared key must still decode")
	})

	t.Run("ReportsNestedKeysByPath", func(t *testing.T) {
		params := api.Params{"outer": map[string]any{"known": "value", "retired": 1}}

		var out struct {
			Outer struct {
				Known string `json:"known"`
			} `json:"outer"`
		}

		unmapped, err := params.DecodeReportingUnmapped(&out)
		require.NoError(t, err, "A nested undeclared key must not fail decoding")
		assert.Equal(t, []string{"outer.retired"}, unmapped,
			"A nested undeclared key must be reported by its path so the drift is locatable")
	})

	t.Run("SortsReportedKeys", func(t *testing.T) {
		params := api.Params{"zeta": 1, "alpha": 2, "mid": 3}

		var out struct{}

		unmapped, err := params.DecodeReportingUnmapped(&out)
		require.NoError(t, err, "Decoding into a target with no fields must not fail")
		assert.Equal(t, []string{"alpha", "mid", "zeta"}, unmapped,
			"Reported keys must be sorted so a caller can dedupe on a stable key")
	})

	t.Run("ReportsNothingWhenEveryKeyMaps", func(t *testing.T) {
		params := api.Params{"known": "value"}

		var out struct {
			Known string `json:"known"`
		}

		unmapped, err := params.DecodeReportingUnmapped(&out)
		require.NoError(t, err, "Decoding must succeed")
		assert.Empty(t, unmapped, "A fully mapped payload must report nothing")
	})

	t.Run("DecodeIgnoresUndeclaredKeys", func(t *testing.T) {
		params := api.Params{"known": "value", "retired": true}

		var out struct {
			Known string `json:"known"`
		}

		require.NoError(t, params.Decode(&out),
			"Decode must ignore undeclared keys so a client sending a retired field keeps working")
		assert.Equal(t, "value", out.Known, "The declared key must still decode")
	})
}
