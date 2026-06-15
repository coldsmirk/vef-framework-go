package outbox

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRedactError covers the redaction + truncation contract: credential
// fragments are scrubbed, the result is capped at maxLastErrorBytes, and
// truncation never emits a partial multi-byte rune.
func TestRedactError(t *testing.T) {
	t.Run("ScrubsCredentialFragments", func(t *testing.T) {
		out := redactError("dial failed password=hunter2 for host")
		assert.Contains(t, out, "[redacted]", "password fragment must be redacted")
		assert.NotContains(t, out, "hunter2", "the secret value must not survive redaction")
	})

	t.Run("TruncatesByteLengthForAscii", func(t *testing.T) {
		long := strings.Repeat("a", maxLastErrorBytes+50)
		out := redactError(long)
		assert.LessOrEqual(t, len(out), maxLastErrorBytes, "ASCII output must be capped at the byte limit")
		assert.True(t, utf8.ValidString(out), "ASCII output must be valid UTF-8")
	})

	t.Run("TruncationNeverEmitsPartialRune", func(t *testing.T) {
		// 多字节 runes are 3 bytes each in UTF-8. Build a string whose byte
		// length crosses maxLastErrorBytes mid-rune so a naive byte slice
		// would leave a dangling fragment.
		multi := strings.Repeat("中", maxLastErrorBytes) // 3 * maxLastErrorBytes bytes
		out := redactError(multi)

		require.True(t, utf8.ValidString(out),
			"truncating across a multi-byte rune boundary must not emit invalid UTF-8")
		assert.LessOrEqual(t, len(out), maxLastErrorBytes,
			"the byte cap must remain an upper bound even for multi-byte input")
		assert.NotEmpty(t, out, "a long multi-byte error must still yield some redacted prefix")
	})

	t.Run("ShortMessagePassesThroughTrimmed", func(t *testing.T) {
		out := redactError("  boom  ")
		assert.Equal(t, "boom", out, "short messages are returned trimmed and unchanged")
	})
}
