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

type ValidationRule struct {
	RuleTag                  string
	ErrMessageTemplate       string
	ErrMessageI18nKey        string
	Validate                 func(fl v.FieldLevel) bool
	ParseParam               func(fe v.FieldError) []string
	CallValidationEvenIfNull bool
}

func (vr ValidationRule) register(validator *v.Validate) error {
	if err := validator.RegisterValidation(vr.RuleTag, vr.Validate, vr.CallValidationEvenIfNull); err != nil {
		return fmt.Errorf("failed to register %q validation rule: %w", vr.RuleTag, err)
	}

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
