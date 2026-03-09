package mold

import (
	"context"
	"errors"
	"strings"

	"github.com/coldsmirk/vef-framework-go/log"
	"github.com/coldsmirk/vef-framework-go/mold"
)

const (
	dictKeyPrefix = "dict:"
)

// ErrDataDictResolverNotConfigured is returned when DataDictResolver is not provided.
var ErrDataDictResolverNotConfigured = errors.New("data dictionary resolver is not configured, please provide one in the container")

// DataDictTranslator is a data dictionary translator that converts code values to readable names.
type DataDictTranslator struct {
	logger   log.Logger
	resolver mold.DataDictResolver
}

func (*DataDictTranslator) Supports(kind string) bool {
	return strings.HasPrefix(kind, dictKeyPrefix)
}

func (t *DataDictTranslator) Translate(ctx context.Context, kind, value string) (string, error) {
	if t.resolver == nil {
		return "", ErrDataDictResolverNotConfigured
	}

	dictKey := kind[len(dictKeyPrefix):]

	result, err := t.resolver.Resolve(ctx, dictKey, value)
	if err != nil {
		t.logger.Errorf("Failed to resolve dictionary %q for code %q: %v", dictKey, value, err)

		return "", err
	}

	return result, nil
}

// NewDataDictTranslator creates a data dictionary translator instance.
func NewDataDictTranslator(resolver mold.DataDictResolver) mold.Translator {
	return &DataDictTranslator{
		logger:   logger.Named("data_dict"),
		resolver: resolver,
	}
}
