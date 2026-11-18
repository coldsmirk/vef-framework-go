package business

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

var kindPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)

type Registry struct {
	handlers map[approval.BusinessKind]approval.BusinessHandler
	ordered  []approval.BusinessKind
}

func NewRegistry(handlers []approval.BusinessHandler, validate func([]approval.FormFieldDefinition) error, builtins ...approval.BusinessHandler) (*Registry, error) {
	byKind := make(map[approval.BusinessKind]approval.BusinessHandler, len(handlers))
	for i, h := range slices.Concat(handlers, builtins) {
		d := h.Describe()
		if len(d.Kind) > 64 || !kindPattern.MatchString(string(d.Kind)) || strings.HasPrefix(string(d.Kind), "approval.") != (i >= len(handlers)) || strings.TrimSpace(d.Label) == "" {
			return nil, fmt.Errorf("%w: %q", ErrInvalidDescriptor, d.Kind)
		}

		if _, ok := byKind[d.Kind]; ok {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateKind, d.Kind)
		}

		if err := validate(d.FormFields); err != nil {
			return nil, fmt.Errorf("business kind %q: %w", d.Kind, err)
		}

		byKind[d.Kind] = h
	}

	return &Registry{handlers: byKind, ordered: slices.Sorted(maps.Keys(byKind))}, nil
}

func (r *Registry) Lookup(kind approval.BusinessKind) (approval.BusinessHandler, bool) {
	if r == nil {
		return nil, false
	}

	h, ok := r.handlers[kind]

	return h, ok
}

func (r *Registry) Require(kind approval.BusinessKind) (approval.BusinessHandler, error) {
	if h, ok := r.Lookup(kind); ok {
		return h, nil
	}

	return nil, fmt.Errorf("%w: %s", approval.ErrBusinessKindNotRegistered, kind)
}

func (r *Registry) Descriptors() []approval.BusinessKindDescriptor {
	ds := make([]approval.BusinessKindDescriptor, 0, len(r.ordered))
	for _, kind := range r.ordered {
		d := r.handlers[kind].Describe()
		if d.FormFields == nil {
			d.FormFields = []approval.FormFieldDefinition{}
		}

		ds = append(ds, d)
	}

	return ds
}

func (r *Registry) View(ctx context.Context, db orm.DB, vc *approval.ViewContext) (any, error) {
	if vc.Instance.BusinessKind == nil {
		return nil, nil
	}

	h, err := r.Require(*vc.Instance.BusinessKind)
	if err != nil {
		return nil, err
	}

	return h.View(ctx, db, vc)
}
