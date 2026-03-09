package mold

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/log"
	"github.com/coldsmirk/vef-framework-go/mold"
	"github.com/coldsmirk/vef-framework-go/null"
	"github.com/coldsmirk/vef-framework-go/reflectx"
)

const (
	translatedFieldNameSuffix = "Name"
)

var (
	// ErrTranslatedFieldNotFound is returned when the target translated field (e.g., StatusName) is not found.
	ErrTranslatedFieldNotFound = errors.New("target translated field not found")
	// ErrTranslationKindEmpty is returned when the translation kind parameter is missing.
	ErrTranslationKindEmpty = errors.New("translation kind parameter is empty")
	// ErrTranslatedFieldNotSettable is returned when the target translated field cannot be set.
	ErrTranslatedFieldNotSettable = errors.New("target translated field is not settable")
	// ErrNoTranslatorSupportsKind is returned when no translator supports the given kind.
	ErrNoTranslatorSupportsKind = errors.New("no translator supports the given kind")
	// ErrUnsupportedFieldType is returned when the field type is not supported for translation.
	ErrUnsupportedFieldType = errors.New("unsupported field type for translation")

	nullStringType = reflect.TypeFor[null.String]()
	nullIntType    = reflect.TypeFor[null.Int]()
	nullInt16Type  = reflect.TypeFor[null.Int16]()
	nullInt32Type  = reflect.TypeFor[null.Int32]()
)

// TranslateTransformer is a translator-based transformer that converts values to readable names
// Supports multiple translators and delegates to the appropriate one based on translation kind (from tag parameters).
type TranslateTransformer struct {
	logger      log.Logger
	translators []mold.Translator
}

// extractStringValue extracts string value from supported field types:
// - string, *string, null.String
// - int, int8, int16, int32, int64 and their pointer forms
// - uint, uint8, uint16, uint32, uint64 and their pointer forms
// - null.Int, null.Int16, null.Int32
// Returns empty string and an error for unsupported types.
func extractStringValue(fieldName string, field reflect.Value) (string, error) {
	if !field.IsValid() {
		return "", fmt.Errorf("%w: field %q is invalid", ErrUnsupportedFieldType, fieldName)
	}

	fieldType := field.Type()
	fieldKind := fieldType.Kind()

	switch {
	case fieldKind == reflect.String:
		return field.String(), nil

	case isSignedInt(fieldKind):
		return cast.ToStringE(field.Int())

	case isUnsignedInt(fieldKind):
		return cast.ToStringE(field.Uint())

	case fieldKind == reflect.Pointer:
		return extractPointerStringValue(fieldName, field)

	case fieldType == nullStringType:
		return field.Interface().(null.String).ValueOrZero(), nil

	case fieldType == nullIntType:
		nullInt := field.Interface().(null.Int)
		if !nullInt.Valid {
			return "", nil
		}

		return cast.ToStringE(nullInt.Int64)

	case fieldType == nullInt16Type:
		nullInt16 := field.Interface().(null.Int16)
		if !nullInt16.Valid {
			return "", nil
		}

		return cast.ToStringE(nullInt16.Int16)

	case fieldType == nullInt32Type:
		nullInt32 := field.Interface().(null.Int32)
		if !nullInt32.Valid {
			return "", nil
		}

		return cast.ToStringE(nullInt32.Int32)

	default:
		return "", fmt.Errorf(
			"%w: field %q has unsupported type %v (supported: string, *string, null.String, null.Int, null.Int16, null.Int32, integers and their pointer forms)",
			ErrUnsupportedFieldType,
			fieldName,
			fieldType,
		)
	}
}

// extractPointerStringValue extracts string value from pointer types.
func extractPointerStringValue(fieldName string, field reflect.Value) (string, error) {
	if field.IsNil() {
		return "", nil
	}

	elemType := reflectx.Indirect(field.Type())
	elemKind := elemType.Kind()
	elemValue := field.Elem()

	switch {
	case elemKind == reflect.String:
		return elemValue.String(), nil
	case isSignedInt(elemKind):
		return cast.ToStringE(elemValue.Int())
	case isUnsignedInt(elemKind):
		return cast.ToStringE(elemValue.Uint())
	default:
		return "", fmt.Errorf("%w: field %q has unsupported pointer element type %v", ErrUnsupportedFieldType, fieldName, elemType)
	}
}

// isSignedInt checks if the kind is a signed integer type.
func isSignedInt(kind reflect.Kind) bool {
	return kind >= reflect.Int && kind <= reflect.Int64
}

// isUnsignedInt checks if the kind is an unsigned integer type.
func isUnsignedInt(kind reflect.Kind) bool {
	return kind >= reflect.Uint && kind <= reflect.Uint64
}

// setTranslatedValue sets the translated string value to the target field.
// Supports string, *string, and null.String types.
func setTranslatedValue(translatedField reflect.Value, translated, translatedFieldName string) error {
	translatedFieldType := translatedField.Type()
	fieldKind := translatedFieldType.Kind()

	if fieldKind == reflect.String {
		translatedField.SetString(translated)

		return nil
	}

	if fieldKind == reflect.Pointer {
		elemType := translatedFieldType.Elem()
		if elemType.Kind() != reflect.String {
			return fmt.Errorf("%w: translated field %q has unsupported pointer type %v", ErrUnsupportedFieldType, translatedFieldName, translatedFieldType)
		}

		if translatedField.IsNil() {
			translatedField.Set(reflect.New(elemType))
		}

		translatedField.Elem().SetString(translated)

		return nil
	}

	if translatedFieldType == nullStringType {
		translatedField.Set(reflect.ValueOf(null.StringFrom(translated)))

		return nil
	}

	return fmt.Errorf("%w: translated field %q has unsupported type %v", ErrUnsupportedFieldType, translatedFieldName, translatedFieldType)
}

// Tag returns the transformer tag name "translate".
func (*TranslateTransformer) Tag() string {
	return "translate"
}

// Transform executes translation transformation logic
// Gets translation kind from tag parameter and field value, then converts through appropriate translator.
func (t *TranslateTransformer) Transform(ctx context.Context, fl mold.FieldLevel) error {
	name := fl.Name()
	field := fl.Field()

	// Extract string value from supported field types
	value, err := extractStringValue(name, field)
	if err != nil {
		return err
	}

	// Skip empty value or name processing
	if name == "" || value == "" {
		return nil
	}

	translatedFieldName := name + translatedFieldNameSuffix

	translatedField, ok := fl.SiblingField(translatedFieldName)
	if !ok {
		return fmt.Errorf("%w: failed to get field %q for field %q with value %q", ErrTranslatedFieldNotFound, translatedFieldName, name, value)
	}

	kind := fl.Param()
	if kind == "" {
		return fmt.Errorf("%w: field %q with value %q", ErrTranslationKindEmpty, name, value)
	}

	// Find the translator that supports the translation kind
	for _, translator := range t.translators {
		if translator.Supports(kind) {
			translated, err := translator.Translate(ctx, kind, value)
			if err != nil {
				return err
			}

			if !translatedField.CanSet() {
				return fmt.Errorf("%w: field %q for field %q with value %q", ErrTranslatedFieldNotSettable, translatedFieldName, name, value)
			}

			return setTranslatedValue(translatedField, translated, translatedFieldName)
		}
	}

	if strings.HasSuffix(kind, "?") {
		return nil
	}

	return fmt.Errorf("%w: kind %q for field %q with value %q", ErrNoTranslatorSupportsKind, kind, name, value)
}

// NewTranslateTransformer creates a translate transformer instance.
func NewTranslateTransformer(translators []mold.Translator) mold.FieldTransformer {
	return &TranslateTransformer{
		logger:      logger.Named("translate"),
		translators: translators,
	}
}
