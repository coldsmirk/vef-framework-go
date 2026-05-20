package mold

import (
	"context"
	"errors"
	"strings"

	"github.com/coldsmirk/vef-framework-go/logx"
	"github.com/coldsmirk/vef-framework-go/mold"
)

const (
	dictKeyPrefix = "dict:"
)

// ErrDictionaryResolverNotConfigured is returned when DictionaryResolver is not provided.
var ErrDictionaryResolverNotConfigured = errors.New("dictionary resolver is not configured, please provide one in the container")

// DictionaryTranslator is a data dictionary translator that converts code values to readable names.
type DictionaryTranslator struct {
	logger   logx.Logger
	resolver mold.DictionaryResolver
}

func (*DictionaryTranslator) Supports(kind string) bool {
	return strings.HasPrefix(kind, dictKeyPrefix)
}

func (t *DictionaryTranslator) Translate(ctx context.Context, kind, value string) (string, error) {
	if t.resolver == nil {
		return "", ErrDictionaryResolverNotConfigured
	}

	dictKey := kind[len(dictKeyPrefix):]

	result, err := t.resolver.Resolve(ctx, dictKey, value)
	if err != nil {
		t.logger.Errorf("Failed to resolve dictionary %q for code %q: %v", dictKey, value, err)

		return "", err
	}

	return result, nil
}

// NewDictionaryTranslator creates a data dictionary translator instance.
func NewDictionaryTranslator(resolver mold.DictionaryResolver) mold.Translator {
	return &DictionaryTranslator{
		logger:   logger.Named("dictionary"),
		resolver: resolver,
	}
}
