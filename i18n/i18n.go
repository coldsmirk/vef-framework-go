package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/samber/lo"
	"golang.org/x/text/language"

	vefconfig "github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/i18n/locales"
	"github.com/coldsmirk/vef-framework-go/internal/log"
)

// DefaultLanguage is the default language for the i18n system.
const DefaultLanguage = "zh-CN"

var (
	logger             = log.Named("i18n")
	supportedLanguages = []string{"zh-CN", "en"}
	translator         Translator
)

func init() {
	var err error
	if translator, err = New(Config{
		Locales: locales.EmbedLocales,
	}); err != nil {
		panic(err)
	}
}

// Config defines the configuration for the i18n system.
type Config struct {
	// Locales contains the embedded locale files (JSON format).
	Locales embed.FS
}

// newBundle creates a new i18n bundle with all supported languages loaded.
func newBundle(localesFS embed.FS) (*i18n.Bundle, error) {
	bundle := i18n.NewBundle(language.SimplifiedChinese)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	for _, lang := range supportedLanguages {
		filename := fmt.Sprintf("%s.json", lang)
		if _, err := bundle.LoadMessageFileFS(localesFS, filename); err != nil {
			logger.Errorf("Failed to load language file %s: %v", filename, err)

			return nil, fmt.Errorf("failed to load language file %s: %w", filename, err)
		}

		logger.Debugf("Successfully loaded language file: %s", filename)
	}

	return bundle, nil
}

// newLocalizer creates a new i18n localizer with all supported languages.
func newLocalizer(config Config) (*i18n.Localizer, error) {
	bundle, err := newBundle(config.Locales)
	if err != nil {
		return nil, err
	}

	preferredLanguage := lo.CoalesceOrEmpty(os.Getenv(vefconfig.EnvI18NLanguage), DefaultLanguage)
	logger.Infof("Using language: %s", preferredLanguage)

	return i18n.NewLocalizer(bundle, preferredLanguage), nil
}

// T translates a message ID using the global translator.
// Returns the messageID as fallback if translation fails.
func T(messageID string, templateData ...map[string]any) string {
	return translator.T(messageID, templateData...)
}

// Te translates a message ID with explicit error handling.
// Use this when you need to handle translation errors programmatically.
func Te(messageID string, templateData ...map[string]any) (string, error) {
	return translator.Te(messageID, templateData...)
}

// GetSupportedLanguages returns a copy of all supported language codes.
func GetSupportedLanguages() []string {
	result := make([]string, len(supportedLanguages))
	copy(result, supportedLanguages)

	return result
}

// IsLanguageSupported checks if the given language code is supported.
func IsLanguageSupported(languageCode string) bool {
	return slices.Contains(supportedLanguages, languageCode)
}

// SetLanguage changes the global translator to use the specified language.
// This is primarily intended for testing scenarios where you need to verify translations
// in different languages without restarting the process.
// If languageCode is empty, uses the environment variable or default language.
func SetLanguage(languageCode string) error {
	if languageCode == "" {
		languageCode = lo.CoalesceOrEmpty(os.Getenv(vefconfig.EnvI18NLanguage), DefaultLanguage)
	}

	if !IsLanguageSupported(languageCode) {
		return fmt.Errorf("%w: %s (supported: %v)", ErrUnsupportedLanguage, languageCode, supportedLanguages)
	}

	bundle, err := newBundle(locales.EmbedLocales)
	if err != nil {
		return err
	}

	localizer := i18n.NewLocalizer(bundle, languageCode)
	translator = &translatorImpl{localizer: localizer}

	logger.Infof("Language set to: %s", languageCode)

	return nil
}
