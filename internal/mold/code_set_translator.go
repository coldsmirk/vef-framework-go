package mold

import (
	"context"
	"errors"
	"strings"

	"github.com/coldsmirk/vef-framework-go/logx"
	"github.com/coldsmirk/vef-framework-go/mold"
)

const (
	codeSetKeyPrefix = "codes:"
)

// ErrCodeSetResolverNotConfigured is returned when CodeSetResolver is not provided.
var ErrCodeSetResolverNotConfigured = errors.New("code set resolver is not configured, please provide one in the container")

// CodeSetTranslator is a code set translator that converts code values to readable names.
type CodeSetTranslator struct {
	logger   logx.Logger
	resolver mold.CodeSetResolver
}

func (*CodeSetTranslator) Supports(kind string) bool {
	return strings.HasPrefix(kind, codeSetKeyPrefix)
}

func (t *CodeSetTranslator) Translate(ctx context.Context, kind, value string) (string, error) {
	if t.resolver == nil {
		return "", ErrCodeSetResolverNotConfigured
	}

	codeSet := kind[len(codeSetKeyPrefix):]

	result, err := t.resolver.Resolve(ctx, codeSet, value)
	if err != nil {
		t.logger.Errorf("Failed to resolve code set %q for code %q: %v", codeSet, value, err)

		return "", err
	}

	return result, nil
}

// NewCodeSetTranslator creates a code set translator instance.
func NewCodeSetTranslator(resolver mold.CodeSetResolver) mold.Translator {
	return &CodeSetTranslator{
		logger:   logger.Named("code_set"),
		resolver: resolver,
	}
}
