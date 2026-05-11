package storage

import (
	"context"
	"reflect"

	collections "github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/reflectx"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// Files is the high-level facade business handlers use to keep their
// `meta`-tagged file references in sync with the storage backend across
// the standard create / update / delete lifecycle.
//
// All three methods MUST be called inside a business transaction; the
// supplied tx is the same orm.DB instance passed to orm.DB.RunInTX, so
// the claim consumption and pending-delete bookkeeping commit or roll
// back atomically with the business write.
//
// Internally Files composes ClaimStore + DeleteQueue + a per-type meta
// field cache; callers do not interact with those primitives directly.
type Files interface {
	// OnCreate adopts every file reference reachable from model by
	// deleting the corresponding upload claim rows inside tx. Returns
	// ErrClaimNotFound (wrapped) when any reference is missing, which
	// must cause the caller's tx to roll back.
	OnCreate(ctx context.Context, tx orm.DB, model any) error

	// OnUpdate reconciles file references between two snapshots of the
	// same model: newly-referenced files are adopted (ConsumeMany);
	// dereferenced files are queued for asynchronous deletion with
	// DeleteReasonReplaced. Either argument may be nil to signal the
	// absence of that side (mirrors FileRefExtractor.Diff semantics).
	OnUpdate(ctx context.Context, tx orm.DB, oldModel, newModel any) error

	// OnDelete schedules every file reference in model for asynchronous
	// deletion with DeleteReasonDeleted.
	OnDelete(ctx context.Context, tx orm.DB, model any) error
}

// NewFiles returns the default Files implementation, sharing the supplied
// ClaimStore and DeleteQueue across all model types. The returned value
// is safe for concurrent use; meta field specs are parsed once per type
// on first access and cached for the lifetime of the instance.
func NewFiles(cs ClaimStore, dq DeleteQueue) Files {
	return &defaultFiles{
		cs:    cs,
		dq:    dq,
		cache: collections.NewConcurrentHashMap[reflect.Type, *cachedExtractor](),
	}
}

type defaultFiles struct {
	cs    ClaimStore
	dq    DeleteQueue
	cache collections.ConcurrentMap[reflect.Type, *cachedExtractor]
}

// cachedExtractor holds the reflect-parsed meta field spec for a single
// model type. Stored in defaultFiles.cache keyed by the indirected type.
type cachedExtractor struct {
	fields []metaField
}

func (f *defaultFiles) OnCreate(ctx context.Context, tx orm.DB, model any) error {
	ext := f.extractorFor(model)
	if ext == nil {
		return nil
	}

	refs := ext.extract(model)
	if len(refs) == 0 {
		return nil
	}

	keys := make([]string, len(refs))
	for i, r := range refs {
		keys[i] = r.Key
	}

	return f.cs.ConsumeMany(ctx, tx, keys)
}

func (f *defaultFiles) OnUpdate(ctx context.Context, tx orm.DB, oldModel, newModel any) error {
	ext := f.extractorFor(newModel)
	if ext == nil {
		ext = f.extractorFor(oldModel)
	}
	if ext == nil {
		return nil
	}

	oldRefs := ext.extract(oldModel)
	newRefs := ext.extract(newModel)

	toConsume, toDelete := diffRefs(newRefs, oldRefs)

	if len(toConsume) > 0 {
		keys := make([]string, len(toConsume))
		for i, r := range toConsume {
			keys[i] = r.Key
		}

		if err := f.cs.ConsumeMany(ctx, tx, keys); err != nil {
			return err
		}
	}

	return f.scheduleDeletes(ctx, tx, toDelete, DeleteReasonReplaced)
}

func (f *defaultFiles) OnDelete(ctx context.Context, tx orm.DB, model any) error {
	ext := f.extractorFor(model)
	if ext == nil {
		return nil
	}

	return f.scheduleDeletes(ctx, tx, ext.extract(model), DeleteReasonDeleted)
}

// extractorFor returns the cached meta spec for model's underlying type,
// parsing it on first access. Returns nil when model is nil or a typed
// nil pointer (caller should treat that as an empty ref set).
func (f *defaultFiles) extractorFor(model any) *cachedExtractor {
	if model == nil {
		return nil
	}

	rv := reflect.ValueOf(model)
	if rv.Kind() == reflect.Pointer && rv.IsNil() {
		return nil
	}

	typ := reflectx.Indirect(reflect.TypeOf(model))

	ext, _ := f.cache.GetOrCompute(typ, func() *cachedExtractor {
		return &cachedExtractor{fields: parseMetaFields(typ)}
	})

	return ext
}

func (f *defaultFiles) scheduleDeletes(
	ctx context.Context,
	tx orm.DB,
	refs []FileRef,
	reason DeleteReason,
) error {
	if len(refs) == 0 {
		return nil
	}

	now := timex.Now()
	items := make([]PendingDelete, len(refs))

	for i, r := range refs {
		items[i] = PendingDelete{
			ID:            id.GenerateUUID(),
			Key:           r.Key,
			Reason:        reason,
			NextAttemptAt: now,
			CreatedAt:     now,
		}
	}

	return f.dq.Schedule(ctx, tx, items)
}

func (e *cachedExtractor) extract(model any) []FileRef {
	if model == nil {
		return nil
	}

	rv := reflect.ValueOf(model)
	if rv.Kind() == reflect.Pointer && rv.IsNil() {
		return nil
	}

	value := reflect.Indirect(rv)
	if value.Kind() != reflect.Struct {
		return nil
	}

	return collectFileRefs(value, e.fields)
}

// diffRefs partitions two ref sets by key:
//
//	toConsume = refs in newRefs whose key is not in oldRefs
//	toDelete  = refs in oldRefs whose key is not in newRefs
func diffRefs(newRefs, oldRefs []FileRef) (toConsume, toDelete []FileRef) {
	newSet := make(map[string]struct{}, len(newRefs))
	for _, r := range newRefs {
		newSet[r.Key] = struct{}{}
	}

	oldSet := make(map[string]struct{}, len(oldRefs))
	for _, r := range oldRefs {
		oldSet[r.Key] = struct{}{}
	}

	for _, r := range newRefs {
		if _, in := oldSet[r.Key]; !in {
			toConsume = append(toConsume, r)
		}
	}

	for _, r := range oldRefs {
		if _, in := newSet[r.Key]; !in {
			toDelete = append(toDelete, r)
		}
	}

	return toConsume, toDelete
}
