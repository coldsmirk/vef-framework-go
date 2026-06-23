package crud_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/i18n"
)

// TestMutationSuccessMessageKeys verifies that every CRUD mutation
// success-message constant resolves to a real translation in all supported
// languages instead of falling back to the raw key name. It guards against
// typos in the constants and missing entries in the locale files — failures
// the handler-level assertions cannot catch, since those resolve the same key
// through i18n.T on both sides.
func TestMutationSuccessMessageKeys(t *testing.T) {
	cases := []struct {
		name string
		key  string
	}{
		{"MessageCreated", crud.MessageCreated},
		{"MessageUpdated", crud.MessageUpdated},
		{"MessageDeleted", crud.MessageDeleted},
		{"MessageImported", crud.MessageImported},
	}

	original := i18n.CurrentLanguage()
	t.Cleanup(func() {
		require.NoError(t, i18n.SetLanguage(original), "Should restore the original language after the test")
	})

	for _, lang := range i18n.GetSupportedLanguages() {
		t.Run(lang, func(t *testing.T) {
			require.NoError(t, i18n.SetLanguage(lang), "Should switch to the target language")

			for _, tc := range cases {
				translated := i18n.T(tc.key)
				assert.NotEmpty(t, translated, "%s (%q) should resolve to a non-empty message", tc.name, tc.key)
				assert.NotEqual(t, tc.key, translated, "%s (%q) should resolve to a translation, not fall back to the key name", tc.name, tc.key)
			}
		})
	}
}
