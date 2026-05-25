package storage

import (
	"context"
	"reflect"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/reflectx"
	"github.com/coldsmirk/vef-framework-go/security"
)

// FilesFor is the type-safe counterpart of Files for handlers that
// manage a single model type. T's `meta`-tagged field spec is resolved
// once at construction (when the underlying Files is the default
// implementation), so malformed tags surface at boot and the per-call
// reflect lookup is elided on the hot path.
//
// All three methods MUST be called inside a business transaction (see
// Files for the full contract).
type FilesFor[T any] struct {
	// files is the universal Files handle. Lifecycle calls fall through
	// to it when the underlying implementation is not *defaultFiles
	// (e.g. an fx.Decorate wrapper or a test fake).
	files Files
	// fast is files type-asserted to *defaultFiles for direct access to
	// the pre-resolution helpers. Nil when files is a foreign
	// implementation; in that case calls go through the public Files
	// interface and pay the per-call cache lookup, exactly like an
	// untyped consumer would.
	fast *defaultFiles
	// ext is the pre-resolved meta spec for T. Only populated when fast
	// is non-nil; consumed by the fast path's *With methods.
	ext *cachedExtractor
}

// NewFilesFor returns a typed file lifecycle facade for T. When files
// is the value produced by NewFiles, T's meta spec is resolved and
// cached up front so each lifecycle call skips the per-call reflect
// lookup. Foreign Files implementations (e.g. fx.Decorate wrappers,
// test fakes) are accepted and simply delegated to via the public
// Files interface — the typed signatures still apply, only the
// pre-resolution optimization is skipped.
func NewFilesFor[T any](files Files) FilesFor[T] {
	f := FilesFor[T]{files: files}

	if df, ok := files.(*defaultFiles); ok {
		f.fast = df
		f.ext = df.extractorForType(reflectx.Indirect(reflect.TypeFor[T]()))
	}

	return f
}

// OnCreate adopts every file reference reachable from model by deleting
// the corresponding upload claim rows inside tx. See Files.OnCreate for
// the full contract; passing a nil pointer is a no-op.
func (f FilesFor[T]) OnCreate(ctx context.Context, tx orm.DB, principal *security.Principal, model *T) error {
	if model == nil {
		return nil
	}

	if f.fast != nil {
		return f.fast.onCreateWith(ctx, tx, principal, model, f.ext)
	}

	return f.files.OnCreate(ctx, tx, principal, model)
}

// OnUpdate reconciles file references between two snapshots of T. See
// Files.OnUpdate for the full contract; either argument may be nil.
func (f FilesFor[T]) OnUpdate(ctx context.Context, tx orm.DB, principal *security.Principal, oldModel, newModel *T) error {
	if oldModel == nil && newModel == nil {
		return nil
	}

	if f.fast != nil {
		return f.fast.onUpdateWith(ctx, tx, principal, oldModel, newModel, f.ext)
	}

	return f.files.OnUpdate(ctx, tx, principal, oldModel, newModel)
}

// OnDelete schedules every file reference in model for asynchronous
// deletion. See Files.OnDelete for the full contract; passing a nil
// pointer is a no-op.
func (f FilesFor[T]) OnDelete(ctx context.Context, tx orm.DB, model *T) error {
	if model == nil {
		return nil
	}

	if f.fast != nil {
		return f.fast.onDeleteWith(ctx, tx, model, f.ext)
	}

	return f.files.OnDelete(ctx, tx, model)
}
