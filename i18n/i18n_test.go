package i18n

import (
	"embed"
	"os"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/i18n/locales"
)

// TestConfig tests the Config struct and its field access.
func TestConfig(t *testing.T) {
	t.Run("ConfigFieldAccess", func(t *testing.T) {
		config := Config{
			Locales: locales.EmbedLocales,
		}

		assert.NotNil(t, config.Locales, "Locales field should be accessible")

		// Test that we can read files from the embedded FS
		files, err := config.Locales.ReadDir(".")
		require.NoError(t, err, "Should be able to read directory from embedded FS")
		assert.NotEmpty(t, files, "Should contain locale files")

		// Verify that expected language files exist
		var hasZhCN, hasEn bool
		for _, file := range files {
			if file.Name() == "zh-CN.json" {
				hasZhCN = true
			}

			if file.Name() == "en.json" {
				hasEn = true
			}
		}

		assert.True(t, hasZhCN, "Should contain zh-CN.json file")
		assert.True(t, hasEn, "Should contain en.json file")
	})

	t.Run("ConfigValidation", func(t *testing.T) {
		// Test with valid config
		validConfig := Config{
			Locales: locales.EmbedLocales,
		}

		translator, err := New(validConfig)
		require.NoError(t, err, "Should create translator with valid config")
		assert.NotNil(t, translator, "Should return non-nil translator")

		// Test translation works with the new config
		msg := translator.T("ok")
		assert.NotEmpty(t, msg, "Should return translated message")
		assert.NotEqual(t, "ok", msg, "Should translate the message")
	})

	t.Run("ConfigWithEmptyLocales", func(t *testing.T) {
		// Test with empty embed.FS
		var emptyFS embed.FS

		emptyConfig := Config{
			Locales: emptyFS,
		}

		translator, err := New(emptyConfig)
		assert.Error(t, err, "Should return error with empty locales")
		assert.Nil(t, translator, "Should return nil translator on error")
		assert.Contains(t, err.Error(), "failed to load language file", "Error should mention failed file loading")
	})
}

// TestSetLanguage tests the SetLanguage function.
func TestSetLanguage(t *testing.T) {
	originalTranslator := translator

	t.Run("SetToChinese", func(t *testing.T) {
		err := SetLanguage("zh-CN")
		require.NoError(t, err, "Should not return error")

		msg := T("validator_phone_number")
		t.Logf("Chinese message: %s", msg)
		assert.NotEqual(t, "validator_phone_number", msg, "Translation should succeed")
		assert.Contains(t, msg, "格式", "Should contain Chinese characters")
	})

	t.Run("SetToEnglish", func(t *testing.T) {
		err := SetLanguage("en")
		require.NoError(t, err, "Should not return error")

		msg := T("validator_phone_number")
		t.Logf("English message: %s", msg)
		assert.NotEqual(t, "validator_phone_number", msg, "Translation should succeed")
		assert.Contains(t, msg, "format", "Should contain English text")
	})

	t.Run("SetToEmptyStringUsesDefault", func(t *testing.T) {
		originalEnv := os.Getenv("VEF_I18N_LANGUAGE")

		os.Unsetenv("VEF_I18N_LANGUAGE")

		defer func() {
			if originalEnv != "" {
				os.Setenv("VEF_I18N_LANGUAGE", originalEnv)
			}
		}()

		err := SetLanguage("")
		require.NoError(t, err, "Should not return error")

		msg := T("ok")
		t.Logf("Default language message: %s", msg)
		assert.NotEqual(t, "ok", msg, "Translation should succeed")
		assert.Contains(t, msg, "成功", "Should use zh-CN as default")
	})

	t.Run("SetToUnsupportedLanguage", func(t *testing.T) {
		err := SetLanguage("fr")
		assert.Error(t, err, "Should return error for unsupported language")
		assert.Contains(t, err.Error(), "unsupported language code", "Error should mention unsupported language")
	})

	translator = originalTranslator
}

// TestGetSupportedLanguages tests the GetSupportedLanguages function.
func TestGetSupportedLanguages(t *testing.T) {
	langs := GetSupportedLanguages()

	assert.NotEmpty(t, langs, "Should return non-empty list")
	assert.Contains(t, langs, "zh-CN", "Should contain zh-CN")
	assert.Contains(t, langs, "en", "Should contain en")

	langs[0] = "modified"
	newLangs := GetSupportedLanguages()
	assert.NotEqual(t, "modified", newLangs[0], "Should return a copy, not the original slice")
}

// TestIsLanguageSupported tests the IsLanguageSupported function.
func TestIsLanguageSupported(t *testing.T) {
	tests := []struct {
		name     string
		langCode string
		want     bool
	}{
		{"Chinese", "zh-CN", true},
		{"English", "en", true},
		{"French", "fr", false},
		{"German", "de", false},
		{"Empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsLanguageSupported(tt.langCode)
			assert.Equal(t, tt.want, got, "IsLanguageSupported(%q) = %v, want %v", tt.langCode, got, tt.want)
		})
	}
}

// TestTranslator tests the global T and TE functions.
func TestTranslator(t *testing.T) {
	err := SetLanguage("zh-CN")
	require.NoError(t, err, "Should not return error")

	t.Run("TFunctionWithValidMessageID", func(t *testing.T) {
		msg := T("ok")
		assert.NotEmpty(t, msg, "Should return non-empty message")
		assert.NotEqual(t, "ok", msg, "Should translate the message")
		assert.Contains(t, msg, "成功", "Should contain Chinese translation")
	})

	t.Run("TFunctionWithInvalidMessageID", func(t *testing.T) {
		msg := T("nonexistent.message.key")
		assert.Equal(t, "nonexistent.message.key", msg, "Should return message ID as fallback")
	})

	t.Run("TEFunctionWithValidMessageID", func(t *testing.T) {
		msg, err := Te("ok")
		assert.NoError(t, err, "Should not return error for valid message")
		assert.NotEmpty(t, msg, "Should return non-empty message")
		assert.Contains(t, msg, "成功", "Should contain Chinese translation")
	})

	t.Run("TEFunctionWithInvalidMessageID", func(t *testing.T) {
		msg, err := Te("nonexistent.message.key")
		assert.Error(t, err, "Should return error for nonexistent message")
		assert.Empty(t, msg, "Should return empty message on error")
	})

	t.Run("TEFunctionWithEmptyMessageID", func(t *testing.T) {
		msg, err := Te("")
		assert.Error(t, err, "Should return error for empty message ID")
		assert.Empty(t, msg, "Should return empty message on error")
	})

	t.Cleanup(func() {
		_ = SetLanguage("")
	})
}

// TestTranslatorInterfaceConsistency tests the consistency between T and Te methods.
// Feature: i18n-naming-standardization, Property 1: Interface implementation consistency.
func TestTranslatorInterfaceConsistency(t *testing.T) {
	// Create a translator instance for testing
	config := Config{
		Locales: locales.EmbedLocales,
	}

	translator, err := New(config)
	require.NoError(t, err, "Should create translator successfully")

	// Property test: For any message ID, when Te returns an error, T should return the messageID as fallback
	property := func(messageID string) bool {
		// Skip empty strings as they have special handling
		if messageID == "" {
			return true
		}

		// Call Te method
		translatedMsg, err := translator.Te(messageID)

		// Call T method
		fallbackMsg := translator.T(messageID)

		// If Te returns an error, T should return the messageID as fallback
		if err != nil {
			return fallbackMsg == messageID
		}

		// If Te succeeds, T should return the same translated message
		return fallbackMsg == translatedMsg
	}

	// Run property test with 100 iterations
	quickConfig := &quick.Config{MaxCount: 100}
	err = quick.Check(property, quickConfig)
	assert.NoError(t, err, "Property should hold for all generated message IDs")

	// Test with some specific known cases
	testCases := []string{
		"ok",                      // Valid message ID
		"nonexistent.message.key", // Invalid message ID
		"validator_phone_number",  // Another valid message ID
		"completely.unknown.key",  // Another invalid message ID
	}

	for _, messageID := range testCases {
		t.Run("MessageID_"+messageID, func(t *testing.T) {
			translatedMsg, err := translator.Te(messageID)
			fallbackMsg := translator.T(messageID)

			if err != nil {
				assert.Equal(t, messageID, fallbackMsg,
					"When Te returns error, T should return messageID as fallback")
			} else {
				assert.Equal(t, translatedMsg, fallbackMsg,
					"When Te succeeds, T should return the same translated message")
			}
		})
	}
}

// TestAPIBehaviorConsistency tests the consistency of API behavior across different scenarios.
// Feature: i18n-naming-standardization, Property 2: API behavior consistency.
func TestAPIBehaviorConsistency(t *testing.T) {
	// Property test: Translation behavior should be consistent regardless of how translator is created
	property := func(messageID string) bool {
		// Skip empty strings as they have special handling
		if messageID == "" {
			return true
		}

		// Create translator instance directly
		config := Config{
			Locales: locales.EmbedLocales,
		}

		directTranslator, err := New(config)
		if err != nil {
			return false // Skip if translator creation fails
		}

		// Use global translator (initialized in init())
		globalResult := T(messageID)
		directResult := directTranslator.T(messageID)

		// Both should return the same result
		return globalResult == directResult
	}

	// Run property test with 50 iterations
	quickConfig := &quick.Config{MaxCount: 50}
	err := quick.Check(property, quickConfig)
	assert.NoError(t, err, "API behavior should be consistent between global and direct translator usage")

	// Test specific scenarios for API consistency
	testCases := []struct {
		name      string
		messageID string
	}{
		{"ValidMessage", "ok"},
		{"InvalidMessage", "nonexistent.key"},
		{"ValidatorMessage", "validator_phone_number"},
		{"AnotherInvalidMessage", "unknown.message"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create fresh translator instance
			config := Config{
				Locales: locales.EmbedLocales,
			}
			directTranslator, err := New(config)
			require.NoError(t, err, "Should create translator successfully")

			// Compare global vs direct translator results
			globalResult := T(tc.messageID)
			directResult := directTranslator.T(tc.messageID)

			assert.Equal(t, globalResult, directResult,
				"Global and direct translator should return same result for messageID: %s", tc.messageID)

			// Also test Te method consistency
			globalTeResult, globalTeErr := Te(tc.messageID)
			directTeResult, directTeErr := directTranslator.Te(tc.messageID)

			assert.Equal(t, globalTeResult, directTeResult,
				"Global and direct translator Te should return same result for messageID: %s", tc.messageID)

			// Error states should also be consistent
			if globalTeErr != nil && directTeErr != nil {
				// Both should have errors - check error types are similar
				assert.Contains(t, directTeErr.Error(), "translation failed",
					"Both errors should be translation failures")
			} else {
				assert.Equal(t, globalTeErr, directTeErr,
					"Error states should be consistent between global and direct translator")
			}
		})
	}

	// Test language switching consistency
	t.Run("LanguageSwitchingConsistency", func(t *testing.T) {
		originalLang := "zh-CN"
		testLang := "en"

		// Set to test language
		err := SetLanguage(testLang)
		require.NoError(t, err, "Should set language successfully")

		// Create new translator with same language
		config := Config{
			Locales: locales.EmbedLocales,
		}
		directTranslator, err := New(config)
		require.NoError(t, err, "Should create translator successfully")

		// Test a known message
		globalResult := T("ok")
		directResult := directTranslator.T("ok")

		// Note: Direct translator uses environment/default language, not the globally set language
		// So we test that both produce valid translations, not necessarily identical ones
		assert.NotEqual(t, "ok", globalResult, "Global translator should translate")
		assert.NotEqual(t, "ok", directResult, "Direct translator should translate")

		// Restore original language
		err = SetLanguage(originalLang)
		require.NoError(t, err, "Should restore language successfully")
	})
}
