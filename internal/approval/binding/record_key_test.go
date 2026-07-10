package binding

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecordKeysEqual(t *testing.T) {
	t.Run("IgnoresJSONRepresentationDifferences", func(t *testing.T) {
		left := json.RawMessage(`[{"column":"tenant_id","kind":"string","value":"tenant-1"}]`)
		right := json.RawMessage(`[
			{"value":"tenant-1", "kind":"string", "column":"tenant_id"}
		]`)

		equal, err := recordKeysEqual(left, right)
		require.NoError(t, err, "Equivalent record keys should decode")
		require.True(t, equal, "Equivalent record keys should compare structurally")
	})

	t.Run("DetectsDifferentTarget", func(t *testing.T) {
		left := json.RawMessage(`[{"column":"id","kind":"string","value":"order-1"}]`)
		right := json.RawMessage(`[{"column":"id","kind":"string","value":"order-2"}]`)

		equal, err := recordKeysEqual(left, right)
		require.NoError(t, err, "Valid record keys should decode")
		require.False(t, equal, "Different record keys should not compare equal")
	})

	t.Run("RejectsInvalidStoredKey", func(t *testing.T) {
		_, err := recordKeysEqual(json.RawMessage(`{`), json.RawMessage(`[]`))
		require.Error(t, err, "Invalid stored record key JSON should fail")
	})
}
