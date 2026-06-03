package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sync/atomic"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/samber/lo"
	"golang.org/x/text/language"

	vefconfig "github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/i18n/locales"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
)

// DefaultLanguage is the default language for the i18n system.
const DefaultLanguage = "zh-CN"

var (
	logger             = logx.Named("i18n")
	supportedLanguages = []string{"zh-CN", "en"}
	// current holds the active global translation state. It is read by T/Te on
	// every translation (including from concurrent request handlers) and swapped
	// atomically by SetLanguage, so the pointer load/store keeps those accesses
	// race-free.
	current atomic.Pointer[state]
)

// state is an immutable snapshot of the global translation configuration.
// SetLanguage publishes a fresh snapshot rather than mutating fields in place.
type state struct {
	translator *translatorImpl
	language   string
	locales    embed.FS
}

func init() {
	preferredLanguage := lo.CoalesceOrEmpty(os.Getenv(vefconfig.EnvI18NLanguage), DefaultLanguage)

	st, err := newState(locales.EmbedLocales, preferredLanguage)
	if err != nil {
		panic(err)
	}

	logger.Infof("Using language: %s", st.language)
	current.Store(st)
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

// newState builds a translation snapshot for the given locale set and language.
func newState(localesFS embed.FS, languageCode string) (*state, error) {
	bundle, err := newBundle(localesFS)
	if err != nil {
		return nil, err
	}

	return &state{
		translator: &translatorImpl{localizer: i18n.NewLocalizer(bundle, languageCode)},
		language:   languageCode,
		locales:    localesFS,
	}, nil
}

// T translates a message ID using the global translator.
// Returns the messageID as fallback if translation fails.
func T(messageID string, templateData ...map[string]any) string {
	return current.Load().translator.T(messageID, templateData...)
}

// Te translates a message ID with explicit error handling.
// Use this when you need to handle translation errors programmatically.
func Te(messageID string, templateData ...map[string]any) (string, error) {
	return current.Load().translator.Te(messageID, templateData...)
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

// CurrentLanguage returns the language code the global translator is currently using.
func CurrentLanguage() string {
	return current.Load().language
}

// SetLanguage changes the global translator to use the specified language.
// This is primarily intended for testing scenarios where you need to verify translations
// in different languages without restarting the process.
// If languageCode is empty, uses the environment variable or default language.
// The active locale set is preserved, so a translator built from a custom Config.Locales
// keeps those locales after switching language.
func SetLanguage(languageCode string) error {
	if languageCode == "" {
		languageCode = lo.CoalesceOrEmpty(os.Getenv(vefconfig.EnvI18NLanguage), DefaultLanguage)
	}

	if !IsLanguageSupported(languageCode) {
		return fmt.Errorf("%w: %s (supported: %v)", ErrUnsupportedLanguage, languageCode, supportedLanguages)
	}

	st, err := newState(current.Load().locales, languageCode)
	if err != nil {
		return err
	}

	current.Store(st)

	logger.Infof("Language set to: %s", languageCode)

	return nil
}
