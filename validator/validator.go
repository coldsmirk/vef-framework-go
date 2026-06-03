package validator

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/coldsmirk/go-streams"
	"github.com/go-playground/locales"
	"github.com/gofiber/fiber/v3"
	"github.com/samber/lo"

	enlocale "github.com/go-playground/locales/en"
	zhlocale "github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	v "github.com/go-playground/validator/v10"
	entranslation "github.com/go-playground/validator/v10/translations/en"
	zhtranslation "github.com/go-playground/validator/v10/translations/zh"

	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/result"
)

const (
	tagLabel     = "label"
	tagLabelI18n = "label_i18n"
)

// Built-in (go-playground) rule messages are translated through a dedicated
// translator per language so that, like custom rules, they follow the language
// selected at validation time via i18n.CurrentLanguage rather than being frozen
// to the language present when the package was initialized.
const (
	langZh = "zh"
	langEn = "en"
)

var (
	logger      = logx.Named("validator")
	translators map[string]ut.Translator
	validator   *v.Validate
)

func init() {
	enTranslator := newLocaleTranslator(enlocale.New())
	zhTranslator := newLocaleTranslator(zhlocale.New())
	translators = map[string]ut.Translator{
		langZh: zhTranslator,
		langEn: enTranslator,
	}

	validator = v.New(v.WithRequiredStructEnabled())

	if err := zhtranslation.RegisterDefaultTranslations(validator, zhTranslator); err != nil {
		panic(fmt.Errorf("failed to register zh default translations: %w", err))
	}

	if err := entranslation.RegisterDefaultTranslations(validator, enTranslator); err != nil {
		panic(fmt.Errorf("failed to register en default translations: %w", err))
	}

	validator.RegisterTagNameFunc(func(field reflect.StructField) string {
		label := field.Tag.Get(tagLabel)
		if label != "" {
			return label
		}

		label = field.Tag.Get(tagLabelI18n)
		if label != "" {
			return lo.CoalesceOrEmpty(i18n.T(label), field.Name)
		}

		return field.Name
	})

	setup()
}

// newLocaleTranslator builds a go-playground translator for a single locale.
func newLocaleTranslator(locale locales.Translator) ut.Translator {
	translator, _ := ut.New(locale, locale).GetTranslator(locale.Locale())

	return translator
}

// activeTranslator returns the go-playground translator matching the current
// i18n language. Chinese maps to the zh translator; every other supported
// language falls back to en, mirroring the framework's locale set.
func activeTranslator() ut.Translator {
	if i18n.CurrentLanguage() == i18n.DefaultLanguage {
		return translators[langZh]
	}

	return translators[langEn]
}

func RegisterValidationRules(rules ...ValidationRule) error {
	return streams.FromSlice(rules).ForEachErr(func(rule ValidationRule) error {
		return rule.register(validator, translators)
	})
}

type CustomTypeFunc = func(field reflect.Value) any

func RegisterTypeFunc(fn CustomTypeFunc, types ...any) {
	validator.RegisterCustomTypeFunc(fn, types...)
}

func Validate(value any) error {
	err := validator.Struct(value)
	if err == nil {
		return nil
	}

	var validationErrors v.ValidationErrors
	if !errors.As(err, &validationErrors) || len(validationErrors) == 0 {
		return badRequest(err.Error())
	}

	return badRequest(validationErrors[0].Translate(activeTranslator()))
}

// badRequest wraps a validation message as an outward-facing bad-request error.
func badRequest(message string) result.Error {
	return result.Err(
		message,
		result.WithCode(result.ErrCodeBadRequest),
		result.WithStatus(fiber.StatusBadRequest),
	)
}
