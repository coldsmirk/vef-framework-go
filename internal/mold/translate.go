package mold

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/logx"
	"github.com/coldsmirk/vef-framework-go/mold"
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
)

// TranslateTransformer is a translator-based transformer that converts values to readable names
// Supports multiple translators and delegates to the appropriate one based on translation kind (from tag parameters).
type TranslateTransformer struct {
	logger      logx.Logger
	translators []mold.Translator
}

// extractStringValue extracts string value from supported field types:
// - string, *string
// - int, int8, int16, int32, int64 and their pointer forms
// - uint, uint8, uint16, uint32, uint64 and their pointer forms
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

	default:
		return "", fmt.Errorf(
			"%w: field %q has unsupported type %v (supported: string, *string, integers and their pointer forms)",
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
// Supports string and *string types.
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

	return fmt.Errorf("%w: translated field %q has unsupported type %v", ErrUnsupportedFieldType, translatedFieldName, translatedFieldType)
}

// isStringSliceSource reports whether the field is a []string source.
// Guards against invalid values to keep reflectx.IsStringSliceType from panicking on Type().
func isStringSliceSource(field reflect.Value) bool {
	return field.IsValid() && reflectx.IsStringSliceType(field.Type())
}

// extractStringSlice extracts the underlying []string from a []string source field.
// Returns (values, true, nil) when the source slice is non-nil, including empty slices.
// Returns (nil, false, nil) when the source slice is nil; the caller should leave the target untouched.
// Returns (nil, false, err) when the field type is not a []string.
func extractStringSlice(fieldName string, field reflect.Value) ([]string, bool, error) {
	if !field.IsValid() {
		return nil, false, fmt.Errorf("%w: field %q is invalid", ErrUnsupportedFieldType, fieldName)
	}

	if !reflectx.IsStringSliceType(field.Type()) {
		return nil, false, fmt.Errorf("%w: field %q has type %v (expected []string)", ErrUnsupportedFieldType, fieldName, field.Type())
	}

	values, ok := reflectx.GetStringSliceValue(field)

	return values, ok, nil
}

// setTranslatedSlice writes translated string values into a []string target field.
func setTranslatedSlice(target reflect.Value, values []string, targetFieldName string) error {
	if !reflectx.IsStringSliceType(target.Type()) {
		return fmt.Errorf("%w: translated field %q has type %v (expected []string)", ErrUnsupportedFieldType, targetFieldName, target.Type())
	}

	reflectx.SetStringSliceValue(target, values)

	return nil
}

// Tag returns the transformer tag name "translate".
func (*TranslateTransformer) Tag() string {
	return "translate"
}

// Transform executes translation transformation logic.
// Dispatches between scalar and []string/*[]string slice source paths; both share the same
// `<Field>Name` sibling convention and translator selection rules.
func (t *TranslateTransformer) Transform(ctx context.Context, fl mold.FieldLevel) error {
	name := fl.Name()
	if name == "" {
		return nil
	}

	field := fl.Field()
	if isStringSliceSource(field) {
		return t.transformStringSlice(ctx, fl, name, field)
	}

	return t.transformScalar(ctx, fl, name, field)
}

// transformScalar handles scalar source fields (string, *string, integers and their pointer forms).
// An empty extracted value skips the field entirely, matching the original behavior.
func (t *TranslateTransformer) transformScalar(ctx context.Context, fl mold.FieldLevel, name string, field reflect.Value) error {
	value, err := extractStringValue(name, field)
	if err != nil {
		return err
	}

	if value == "" {
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

// transformStringSlice handles []string source fields, translating each element via the chosen
// translator. Nil source skips writing the target; empty slice writes an empty slice; every
// element (including empty strings) is forwarded to Translator.Translate so the translator owns
// the empty-element semantics. Element-level errors fail fast and include the index.
func (t *TranslateTransformer) transformStringSlice(ctx context.Context, fl mold.FieldLevel, name string, field reflect.Value) error {
	values, ok, err := extractStringSlice(name, field)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	translatedFieldName := name + translatedFieldNameSuffix

	translatedField, found := fl.SiblingField(translatedFieldName)
	if !found {
		return fmt.Errorf("%w: failed to get field %q for field %q", ErrTranslatedFieldNotFound, translatedFieldName, name)
	}

	kind := fl.Param()
	if kind == "" {
		return fmt.Errorf("%w: field %q", ErrTranslationKindEmpty, name)
	}

	if !translatedField.CanSet() {
		return fmt.Errorf("%w: field %q for field %q", ErrTranslatedFieldNotSettable, translatedFieldName, name)
	}

	var translator mold.Translator

	for _, candidate := range t.translators {
		if candidate.Supports(kind) {
			translator = candidate

			break
		}
	}

	if translator == nil {
		if strings.HasSuffix(kind, "?") {
			return nil
		}

		return fmt.Errorf("%w: kind %q for field %q", ErrNoTranslatorSupportsKind, kind, name)
	}

	results := make([]string, len(values))

	for i, value := range values {
		translated, err := translator.Translate(ctx, kind, value)
		if err != nil {
			return fmt.Errorf("element[%d] of field %q: %w", i, name, err)
		}

		results[i] = translated
	}

	return setTranslatedSlice(translatedField, results, translatedFieldName)
}

// NewTranslateTransformer creates a translate transformer instance.
func NewTranslateTransformer(translators []mold.Translator) mold.FieldTransformer {
	return &TranslateTransformer{
		logger:      logger.Named("translate"),
		translators: translators,
	}
}
