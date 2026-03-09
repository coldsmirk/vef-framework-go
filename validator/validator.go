package validator

import (
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/coldsmirk/go-streams"
	"github.com/gofiber/fiber/v3"
	"github.com/samber/lo"

	enlocale "github.com/go-playground/locales/en"
	zhlocale "github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	v "github.com/go-playground/validator/v10"
	entranslation "github.com/go-playground/validator/v10/translations/en"
	zhtranslation "github.com/go-playground/validator/v10/translations/zh"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/log"
	"github.com/coldsmirk/vef-framework-go/null"
	"github.com/coldsmirk/vef-framework-go/result"
)

const (
	tagLabel     = "label"
	tagLabelI18n = "label_i18n"
)

var (
	logger     = log.Named("validator")
	translator ut.Translator
	validator  *v.Validate
)

func init() {
	preferredLanguage := lo.CoalesceOrEmpty(os.Getenv(config.EnvI18NLanguage), i18n.DefaultLanguage)
	localeTranslator := lo.TernaryF(
		preferredLanguage == i18n.DefaultLanguage,
		zhlocale.New,
		enlocale.New,
	)
	universalTranslator := ut.New(localeTranslator, localeTranslator)

	translator, _ = universalTranslator.GetTranslator(
		lo.Ternary(
			preferredLanguage == i18n.DefaultLanguage,
			"zh",
			"en",
		),
	)
	validator = v.New(v.WithRequiredStructEnabled())

	if err := lo.TernaryF(
		preferredLanguage == i18n.DefaultLanguage,
		func() error {
			return zhtranslation.RegisterDefaultTranslations(validator, translator)
		},
		func() error {
			return entranslation.RegisterDefaultTranslations(validator, translator)
		},
	); err != nil {
		panic(
			fmt.Errorf("failed to register default translations: %w", err),
		)
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

func RegisterValidationRules(rules ...ValidationRule) error {
	return streams.FromSlice(rules).ForEachErr(func(rule ValidationRule) error {
		return rule.register(validator)
	})
}

type CustomTypeFunc = func(field reflect.Value) any

func RegisterTypeFunc(fn CustomTypeFunc, types ...any) {
	validator.RegisterCustomTypeFunc(fn, types...)
}

func RegisterNullValueTypeFunc[T any]() {
	validator.RegisterCustomTypeFunc(
		func(field reflect.Value) any {
			if nv, ok := field.Interface().(null.Value[T]); ok && nv.Valid {
				return nv.V
			}

			return nil
		},
		null.Value[T]{},
	)
}

func Validate(value any) error {
	err := validator.Struct(value)
	if err == nil {
		return nil
	}

	var validationErrors v.ValidationErrors
	if !errors.As(err, &validationErrors) || len(validationErrors) == 0 {
		return result.Err(
			err.Error(),
			result.WithCode(result.ErrCodeBadRequest),
			result.WithStatus(fiber.StatusBadRequest),
		)
	}

	return result.Err(
		validationErrors[0].Translate(translator),
		result.WithCode(result.ErrCodeBadRequest),
		result.WithStatus(fiber.StatusBadRequest),
	)
}
