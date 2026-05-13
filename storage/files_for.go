package storage

import (
	"context"
	"reflect"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/reflectx"
)

// FilesFor is the type-safe counterpart of Files for handlers that
// manage a single model type. T's `meta`-tagged field spec is resolved
// once at construction (when the underlying Files exposes its cache),
// so malformed tags surface at boot and the per-call reflect lookup is
// elided on the hot path.
//
// All three methods MUST be called inside a business transaction (see
// Files for the full contract).
type FilesFor[T any] struct {
	// files is the universal Files handle. Lifecycle calls fall through
	// to it when the underlying implementation does not expose the
	// pre-resolution fast path (e.g. a custom decorator).
	files Files
	// cached is the same value as files, type-asserted to the unexported
	// fast-path surface. Nil when files is a foreign implementation; in
	// that case calls go through the public Files interface and pay the
	// per-call cache lookup, exactly like an untyped consumer would.
	cached cachedFiles
	// ext is the pre-resolved meta spec for T. Only populated when
	// cached is non-nil; consumed by the fast path's *With methods.
	ext *cachedExtractor
}

// cachedFiles is the unexported surface FilesFor[T] needs from a Files
// implementation to skip the per-call reflect lookup. Only *defaultFiles
// satisfies it today; the indirection lets us probe for the capability
// rather than name the concrete type.
type cachedFiles interface {
	extractorForType(reflect.Type) *cachedExtractor
	onCreateWith(ctx context.Context, tx orm.DB, model any, ext *cachedExtractor) error
	onUpdateWith(ctx context.Context, tx orm.DB, oldModel, newModel any, ext *cachedExtractor) error
	onDeleteWith(ctx context.Context, tx orm.DB, model any, ext *cachedExtractor) error
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

	if cached, ok := files.(cachedFiles); ok {
		f.cached = cached
		f.ext = cached.extractorForType(reflectx.Indirect(reflect.TypeFor[T]()))
	}

	return f
}

// OnCreate adopts every file reference reachable from model by deleting
// the corresponding upload claim rows inside tx. See Files.OnCreate for
// the full contract; passing a nil pointer is a no-op.
func (f FilesFor[T]) OnCreate(ctx context.Context, tx orm.DB, model *T) error {
	if model == nil {
		return nil
	}

	if f.cached != nil {
		return f.cached.onCreateWith(ctx, tx, model, f.ext)
	}

	return f.files.OnCreate(ctx, tx, model)
}

// OnUpdate reconciles file references between two snapshots of T. See
// Files.OnUpdate for the full contract; either argument may be nil.
func (f FilesFor[T]) OnUpdate(ctx context.Context, tx orm.DB, oldModel, newModel *T) error {
	if oldModel == nil && newModel == nil {
		return nil
	}

	if f.cached != nil {
		return f.cached.onUpdateWith(ctx, tx, oldModel, newModel, f.ext)
	}

	return f.files.OnUpdate(ctx, tx, oldModel, newModel)
}

// OnDelete schedules every file reference in model for asynchronous
// deletion. See Files.OnDelete for the full contract; passing a nil
// pointer is a no-op.
func (f FilesFor[T]) OnDelete(ctx context.Context, tx orm.DB, model *T) error {
	if model == nil {
		return nil
	}

	if f.cached != nil {
		return f.cached.onDeleteWith(ctx, tx, model, f.ext)
	}

	return f.files.OnDelete(ctx, tx, model)
}
