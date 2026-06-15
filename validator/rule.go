package validator

import (
	"fmt"
	"strings"

	ut "github.com/go-playground/universal-translator"
	v "github.com/go-playground/validator/v10"

	"github.com/coldsmirk/vef-framework-go/i18n"
)

var presetValidationRules = []ValidationRule{
	newPhoneNumberRule(),
	newDecimalMinRule(),
	newDecimalMaxRule(),
	newAlphanumUsRule(),
	newAlphanumUsSlashRule(),
	newAlphanumUsDotRule(),
}

// ValidationRule describes a custom validation rule registered with the
// underlying go-playground validator. It is the extension point consumed by
// RegisterValidationRules: framework users populate one per custom constraint.
type ValidationRule struct {
	// RuleTag is the struct-tag keyword that activates the rule (e.g. "dec_min"
	// for `validate:"dec_min=10"`). It must be unique across registered rules.
	RuleTag string
	// ErrMessageTemplate is the fallback message used when no i18n translation
	// resolves for ErrMessageI18nKey. It may contain {N} placeholders filled
	// from ParseParam in order ({0} is the first element, {1} the second).
	ErrMessageTemplate string
	// ErrMessageI18nKey is the i18n key looked up first to render the error
	// message; when it resolves, the translated text receives the same {N}
	// placeholder substitution as ErrMessageTemplate. Leave empty to always use
	// ErrMessageTemplate.
	ErrMessageI18nKey string
	// Validate reports whether the field value satisfies the rule. It returns
	// true when valid; the rule parameter (right of '=') is available via
	// fl.Param().
	Validate func(fl v.FieldLevel) bool
	// ParseParam returns the ordered substitution values for the message
	// placeholders ({0}, {1}, ...) for a failing field, typically the field
	// name and the rule parameter.
	ParseParam func(fe v.FieldError) []string
	// CallValidationEvenIfNull maps to go-playground's callEvenIfNull: when
	// true, Validate is invoked even for nil/zero fields. Leave false unless the
	// rule must inspect absent values.
	CallValidationEvenIfNull bool
}

func (vr ValidationRule) register(validator *v.Validate, translators map[string]ut.Translator) error {
	if err := validator.RegisterValidation(vr.RuleTag, vr.Validate, vr.CallValidationEvenIfNull); err != nil {
		return fmt.Errorf("failed to register %q validation rule: %w", vr.RuleTag, err)
	}

	for _, translator := range translators {
		if err := validator.RegisterTranslation(
			vr.RuleTag,
			translator,
			func(t ut.Translator) error {
				return t.Add(vr.RuleTag, vr.ErrMessageTemplate, false)
			},
			func(t ut.Translator, fe v.FieldError) string {
				if vr.ErrMessageI18nKey != "" {
					msg := i18n.T(vr.ErrMessageI18nKey)
					if msg != vr.ErrMessageI18nKey {
						return vr.replacePlaceholders(msg, vr.ParseParam(fe))
					}
				}

				msg, err := t.T(vr.RuleTag, vr.ParseParam(fe)...)
				if err != nil {
					logger.Errorf("Failed to translate %s: %v", vr.RuleTag, err)

					return vr.ErrMessageTemplate
				}

				return msg
			},
		); err != nil {
			return fmt.Errorf("failed to register %q validation rule: %w", vr.RuleTag, err)
		}
	}

	return nil
}

func (ValidationRule) replacePlaceholders(message string, params []string) string {
	result := message
	for i, param := range params {
		placeholder := fmt.Sprintf("{%d}", i)
		result = strings.ReplaceAll(result, placeholder, param)
	}

	return result
}
