package command

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTruncateRunes pins the rune-aware truncation that keeps a rendered
// instance title within the VARCHAR(256) column. A byte-slice regression would
// either overflow the column (ASCII) or corrupt multi-byte text (CJK).
func TestTruncateRunes(t *testing.T) {
	t.Run("ShortStringUnchanged", func(t *testing.T) {
		assert.Equal(t, "hello", truncateRunes("hello", 10), "sub-limit ASCII string must be returned unchanged")
	})

	t.Run("ExactLimitUnchanged", func(t *testing.T) {
		assert.Equal(t, "abcde", truncateRunes("abcde", 5), "string at exactly the limit must be unchanged")
	})

	t.Run("OverLimitCutToMaxRunes", func(t *testing.T) {
		assert.Equal(t, "abc", truncateRunes("abcdef", 3), "over-limit string must be cut to exactly maxRunes runes")
	})

	t.Run("MultiByteNeverSplitMidRune", func(t *testing.T) {
		// CJK runes are 3 bytes each; a byte-slice cut would corrupt them.
		got := truncateRunes("中文测试内容", 3) // 6 runes -> 3
		assert.True(t, utf8.ValidString(got), "truncated multi-byte string must remain valid UTF-8")
		assert.Equal(t, "中文测", got, "multi-byte string must be cut to exactly maxRunes runes without splitting")
	})
}

// TestRenderInstanceTitleTruncation verifies both return paths of
// renderInstanceTitle (empty-template fallback and rendered template) clamp an
// oversize result to instanceTitleMaxRunes, while a sub-limit title is kept
// verbatim.
func TestRenderInstanceTitleTruncation(t *testing.T) {
	t.Run("EmptyTemplateFallbackTruncated", func(t *testing.T) {
		data := map[string]any{
			"flowName":   strings.Repeat("f", instanceTitleMaxRunes),
			"instanceNo": "NO-001",
		}
		got, err := renderInstanceTitle("", data)
		require.NoError(t, err, "empty-template fallback must not error")
		assert.Equal(t, instanceTitleMaxRunes, utf8.RuneCountInString(got), "empty-template fallback output must be truncated to maxRunes")
	})

	t.Run("RenderedTemplateTruncated", func(t *testing.T) {
		data := map[string]any{
			"formData": map[string]any{"note": strings.Repeat("x", instanceTitleMaxRunes*2)},
		}
		got, err := renderInstanceTitle("{{.formData.note}}", data)
		require.NoError(t, err, "rendered template must not error")
		assert.True(t, utf8.ValidString(got), "rendered output must be valid UTF-8")
		assert.Equal(t, instanceTitleMaxRunes, utf8.RuneCountInString(got), "rendered template output must be truncated to maxRunes")
	})

	t.Run("ShortTitlePreserved", func(t *testing.T) {
		data := map[string]any{"formData": map[string]any{"note": "Travel"}}
		got, err := renderInstanceTitle("{{.formData.note}}", data)
		require.NoError(t, err, "short template must not error")
		assert.Equal(t, "Travel", got, "sub-limit rendered title must be preserved verbatim")
	})
}
