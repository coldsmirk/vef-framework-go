package i18n

import (
	"fmt"
	"os"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/samber/lo"

	vefconfig "github.com/coldsmirk/vef-framework-go/config"
)

// Translator defines the interface for message translation services.
type Translator interface {
	// T translates a message ID to a localized string with graceful error handling.
	// If translation fails, it returns the original messageID as a fallback.
	T(messageID string, templateData ...map[string]any) string

	// Te translates a message ID to a localized string and returns explicit error information.
	// Use this when you need to distinguish between successful translation and failure.
	Te(messageID string, templateData ...map[string]any) (string, error)
}

// translatorImpl is the concrete implementation of the Translator interface.
type translatorImpl struct {
	localizer *i18n.Localizer
}

// T implements the Translator interface with graceful error handling.
func (t *translatorImpl) T(messageID string, templateData ...map[string]any) string {
	message, err := t.Te(messageID, templateData...)
	if err != nil {
		logger.Warnf("Translation failed for messageID %q: %v", messageID, err)

		return messageID
	}

	return message
}

// Te implements the Translator interface with explicit error reporting.
func (t *translatorImpl) Te(messageID string, templateData ...map[string]any) (string, error) {
	if messageID == "" {
		return "", ErrMessageIDEmpty
	}

	var data map[string]any
	if len(templateData) > 0 {
		data = templateData[0]
	}

	result, err := t.localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: data,
	})
	if err != nil {
		return "", fmt.Errorf("translation failed for messageID %q: %w", messageID, err)
	}

	return result, nil
}

// New creates a new translator instance with the provided configuration.
// The language is resolved from the environment variable or the default language.
func New(config Config) (Translator, error) {
	preferredLanguage := lo.CoalesceOrEmpty(os.Getenv(vefconfig.EnvI18NLanguage), DefaultLanguage)

	st, err := newState(config.Locales, preferredLanguage)
	if err != nil {
		return nil, err
	}

	return st.translator, nil
}
