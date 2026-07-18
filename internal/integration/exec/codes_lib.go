package exec

import (
	"errors"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/definition"
	"github.com/coldsmirk/vef-framework-go/js"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// codesLibName is the global binding of the code map translation helpers.
const codesLibName = "codes"

// codesLib exposes the system's code maps to adapter scripts:
//
//	codes.toExternal('gender', input.gender)          // canonical → external
//	codes.toCanonical('gender', body.sex)             // external → canonical
//	codes.toExternal('gender', v, { fallback: 'U' })  // per-call unmapped override
//	codes.entries('gender')                           // raw mapping pairs
//
// null and undefined pass through untranslated — translating absence is not a
// lookup. Loaded maps are memoized per execution (one run sees one snapshot),
// compiled lookup indexes are shared across executions by content hash.
type codesLib struct {
	db      orm.DB
	system  *integration.System
	indexes *definition.CodeMapIndexCache
	loaded  map[string]*definition.CodeMapIndex
}

// newCodesLib builds the codes library scoped to system, sharing the
// invoker-wide compiled-index cache.
func newCodesLib(db orm.DB, system *integration.System, indexes *definition.CodeMapIndexCache) js.Lib {
	return &codesLib{db: db, system: system, indexes: indexes, loaded: map[string]*definition.CodeMapIndex{}}
}

func (*codesLib) Name() string {
	return codesLibName
}

func (l *codesLib) Install(rt *js.Runtime) error {
	return rt.Set(codesLibName, map[string]any{
		"toExternal": func(codeSet string, value any, opts ...map[string]any) (any, error) {
			return l.translate(rt, codeSet, value, opts, true)
		},
		"toCanonical": func(codeSet string, value any, opts ...map[string]any) (any, error) {
			return l.translate(rt, codeSet, value, opts, false)
		},
		"entries": func(codeSet string) (any, error) {
			idx, err := l.index(rt, codeSet)
			if err != nil {
				return nil, err
			}

			return canonicalize(idx.Entries())
		},
	})
}

// translate performs one lookup. The options argument is validated before the
// lookup so a typo surfaces on every call, not only on the first unmapped
// value.
func (l *codesLib) translate(rt *js.Runtime, codeSet string, value any, opts []map[string]any, emitExternal bool) (any, error) {
	override, err := parseUnmappedOverride(opts)
	if err != nil {
		return nil, err
	}

	if value == nil {
		return nil, nil
	}

	idx, err := l.index(rt, codeSet)
	if err != nil {
		return nil, err
	}

	key, err := definition.NormalizeCodeValue(value)
	if err != nil {
		return nil, fmt.Errorf("codes: %w", err)
	}

	var target any

	var ok bool
	if emitExternal {
		target, ok = idx.ExternalFor(key)
	} else {
		target, ok = idx.CanonicalFor(key)
	}

	if ok {
		return target, nil
	}

	return unmappedResult(idx, override, codeSet, value, emitExternal)
}

// index resolves the code set's compiled lookup index: the per-execution memo
// first, then the shared content-hash cache over a fresh row read — saves are
// live on the next invocation while one run stays consistent.
func (l *codesLib) index(rt *js.Runtime, codeSet string) (*definition.CodeMapIndex, error) {
	if idx, ok := l.loaded[codeSet]; ok {
		return idx, nil
	}

	m := new(integration.CodeMap)

	err := l.db.NewSelect().
		Model(m).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("system_id", l.system.ID).
				Equals("code_set", codeSet)
		}).
		Scan(rt.Context())
	if err != nil {
		if errors.Is(err, result.ErrRecordNotFound) {
			return nil, &codeMapError{apiErr: integration.ErrMissingCodeMap(codeSet)}
		}

		return nil, err
	}

	if !m.IsEnabled {
		return nil, &codeMapError{apiErr: integration.ErrMissingCodeMap(codeSet)}
	}

	idx, err := l.indexes.Get(m)
	if err != nil {
		return nil, &codeMapError{apiErr: integration.ErrInvalidCodeMap(err.Error())}
	}

	l.loaded[codeSet] = idx

	return idx, nil
}

// unmappedOverride is a per-call unmapped policy override parsed from the
// options argument.
type unmappedOverride struct {
	policy   integration.UnmappedPolicy
	fallback any
}

// parseUnmappedOverride validates the options argument: at most one options
// object carrying exactly one of fallback, passthrough, or reject.
func parseUnmappedOverride(opts []map[string]any) (*unmappedOverride, error) {
	if len(opts) == 0 {
		return nil, nil
	}

	if len(opts) > 1 {
		return nil, fmt.Errorf("%w: at most one options object is allowed", ErrInvalidCodesOption)
	}

	opt := opts[0]
	if len(opt) != 1 {
		return nil, fmt.Errorf(`%w: carry exactly one of "fallback", "passthrough", or "reject"`, ErrInvalidCodesOption)
	}

	for name, value := range opt {
		switch name {
		case "fallback":
			return &unmappedOverride{policy: integration.UnmappedPolicyFallback, fallback: value}, nil
		case "passthrough", "reject":
			enabled, ok := value.(bool)
			if !ok || !enabled {
				return nil, fmt.Errorf("%w: option %q must be true", ErrInvalidCodesOption, name)
			}

			policy := integration.UnmappedPolicyPassthrough
			if name == "reject" {
				policy = integration.UnmappedPolicyReject
			}

			return &unmappedOverride{policy: policy}, nil

		default:
			return nil, fmt.Errorf("%w: unknown option %q", ErrInvalidCodesOption, name)
		}
	}

	return nil, nil
}

// unmappedResult resolves a lookup no entry matched: the per-call override
// wins over the map's stored policy; reject raises a config-classified fault.
func unmappedResult(idx *definition.CodeMapIndex, override *unmappedOverride, codeSet string, value any, emitExternal bool) (any, error) {
	policy := idx.Policy()

	fallback := idx.FallbackCanonical()
	if emitExternal {
		fallback = idx.FallbackExternal()
	}

	if override != nil {
		policy = override.policy
		fallback = override.fallback
	}

	switch policy {
	case integration.UnmappedPolicyPassthrough:
		return value, nil
	case integration.UnmappedPolicyFallback:
		return fallback, nil
	default:
		return nil, &codeMapError{apiErr: integration.ErrUnmappedValue(codeSet, value)}
	}
}
