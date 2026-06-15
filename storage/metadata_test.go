package storage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/storage"
)

// TestCanonicalizeMetadataKeys covers the provider-neutral metadata-key contract
// helper: keys are folded to S3/HTTP-canonical form, values pass through, empty
// inputs collapse to nil, and the result never aliases the input.
func TestCanonicalizeMetadataKeys(t *testing.T) {
	t.Run("CanonicalizesKeysAndPreservesValues", func(t *testing.T) {
		got := storage.CanonicalizeMetadataKeys(map[string]string{
			"author":          "alice",
			"x-custom":        "v1",
			"X-Already-Canon": "v2",
			"UPPER":           "v3",
		})

		assert.Equal(t, map[string]string{
			"Author":          "alice",
			"X-Custom":        "v1",
			"X-Already-Canon": "v2",
			"Upper":           "v3",
		}, got, "keys must be canonicalized via textproto.CanonicalMIMEHeaderKey, values unchanged")
	})

	t.Run("NilInputYieldsNil", func(t *testing.T) {
		assert.Nil(t, storage.CanonicalizeMetadataKeys(nil), "nil input must yield nil, not an empty map")
	})

	t.Run("EmptyInputYieldsNil", func(t *testing.T) {
		assert.Nil(t, storage.CanonicalizeMetadataKeys(map[string]string{}), "empty input must yield nil, not an empty map")
	})

	t.Run("ReturnsNewMapNotAliasingInput", func(t *testing.T) {
		input := map[string]string{"author": "alice"}
		got := storage.CanonicalizeMetadataKeys(input)

		// Mutating the input afterwards must not affect the returned map.
		input["author"] = "mutated"
		input["added"] = "later"

		assert.Equal(t, "alice", got["Author"], "result must be a snapshot, not alias the input map")
		assert.NotContains(t, got, "Added", "keys added to the input after the call must not appear")
		assert.Len(t, got, 1, "result must reflect only the input at call time")
	})

	t.Run("AlreadyCanonicalKeysAreStable", func(t *testing.T) {
		input := map[string]string{"X-Custom": "v", "Mixed-Case-Key": "w"}
		got := storage.CanonicalizeMetadataKeys(input)

		assert.Equal(t, input, got, "already-canonical keys must survive unchanged")
	})

	t.Run("CollidingKeysLastWriterWins", func(t *testing.T) {
		// "author" and "Author" both canonicalize to "Author"; exactly one
		// value survives (map iteration order decides which), and the result
		// has a single entry.
		got := storage.CanonicalizeMetadataKeys(map[string]string{
			"author": "lower",
			"Author": "title",
		})

		require.Len(t, got, 1, "colliding raw keys must collapse to a single canonical entry")
		assert.Contains(t, []string{"lower", "title"}, got["Author"],
			"the surviving value must be one of the colliding inputs (last-writer-wins)")
	})
}
