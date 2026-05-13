package storage

import (
	"context"
	"reflect"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/reflectx"
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
// Internally Files composes ClaimConsumer + DeleteScheduler + a per-type
// meta field cache; callers do not interact with those primitives
// directly.
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
// ClaimConsumer, DeleteScheduler, event Publisher, and URLKeyMapper
// across all model types. The returned value is safe for concurrent
// use; meta field specs are parsed once per type on first access and
// cached for the lifetime of the instance.
//
// The URLKeyMapper translates richtext / markdown URLs to storage keys
// during reconciliation. Pass IdentityURLKeyMapper{} (or nil, which is
// normalised to the identity mapper) when business code embeds bare
// keys directly in <img src> / ![](...).
//
// Promoted-file events are published synchronously after a successful
// ConsumeMany call but before the business transaction commits. Combined
// with an in-memory bus this gives at-least-once delivery with the
// possibility of spurious events if the business transaction later
// rolls back; subscribers MUST be idempotent.
func NewFiles(cc ClaimConsumer, ds DeleteScheduler, publisher event.Publisher, urlMapper URLKeyMapper) Files {
	if urlMapper == nil {
		urlMapper = new(IdentityURLKeyMapper)
	}

	return &defaultFiles{
		cc:        cc,
		ds:        ds,
		publisher: publisher,
		urlMapper: urlMapper,
		cache:     collections.NewConcurrentHashMap[reflect.Type, *cachedExtractor](),
	}
}

type defaultFiles struct {
	cc        ClaimConsumer
	ds        DeleteScheduler
	publisher event.Publisher
	urlMapper URLKeyMapper
	cache     collections.ConcurrentMap[reflect.Type, *cachedExtractor]
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

	refs := f.applyURLMapping(ext.extract(model))
	if len(refs) == 0 {
		return nil
	}

	keys := make([]string, len(refs))
	for i, ref := range refs {
		keys[i] = ref.Key
	}

	if err := f.cc.ConsumeMany(ctx, tx, keys); err != nil {
		return err
	}

	f.publishPromoted(keys)

	return nil
}

func (f *defaultFiles) OnUpdate(ctx context.Context, tx orm.DB, oldModel, newModel any) error {
	ext := f.extractorFor(newModel)
	if ext == nil {
		ext = f.extractorFor(oldModel)
	}

	if ext == nil {
		return nil
	}

	// URL mapping is applied before diffing so old/new ref sets compare
	// on storage keys, not raw embedded URLs. A frontend that switches
	// from "/storage/files/foo.png" to "https://cdn/foo.png" while leaving
	// the same key behind must not look like a delete + re-create.
	oldRefs := f.applyURLMapping(ext.extract(oldModel))
	newRefs := f.applyURLMapping(ext.extract(newModel))

	toConsume, toDelete := diffRefs(newRefs, oldRefs)

	var consumedKeys []string

	if len(toConsume) > 0 {
		consumedKeys = make([]string, len(toConsume))
		for i, ref := range toConsume {
			consumedKeys[i] = ref.Key
		}

		if err := f.cc.ConsumeMany(ctx, tx, consumedKeys); err != nil {
			return err
		}
	}

	if err := f.scheduleDeletes(ctx, tx, toDelete, DeleteReasonReplaced); err != nil {
		return err
	}

	f.publishPromoted(consumedKeys)

	return nil
}

// publishPromoted emits one FilePromotedEvent per adopted key. The
// publisher may be nil in tests; callers must therefore always go
// through this helper rather than touching f.publisher directly.
func (f *defaultFiles) publishPromoted(keys []string) {
	if f.publisher == nil || len(keys) == 0 {
		return
	}

	for _, key := range keys {
		f.publisher.Publish(NewFilePromotedEvent(key))
	}
}

func (f *defaultFiles) OnDelete(ctx context.Context, tx orm.DB, model any) error {
	ext := f.extractorFor(model)
	if ext == nil {
		return nil
	}

	return f.scheduleDeletes(ctx, tx, f.applyURLMapping(ext.extract(model)), DeleteReasonDeleted)
}

// applyURLMapping rewrites richtext / markdown ref keys through the
// configured URLKeyMapper so embedded URLs become storage object keys
// before they hit ClaimConsumer / DeleteScheduler.
//
// uploaded_file refs are passed through unchanged because their values
// come from upload-time storage keys directly; they are never URLs.
//
// richtext / markdown refs are URLs from user content. The mapper's
// (key, ok) result decides their fate:
//
//   - ok=true: replace the raw URL with the resolved storage key.
//   - ok=false: drop the ref entirely; the URL refers to something
//     outside this storage system (external CDN, mailto, data URI,
//     bad input). Reconciliation must not touch unrelated objects.
//
// The output slice is rebuilt rather than mutated in place so callers
// holding the input slice are not surprised by drops.
func (f *defaultFiles) applyURLMapping(refs []FileRef) []FileRef {
	if len(refs) == 0 {
		return refs
	}

	out := make([]FileRef, 0, len(refs))
	for _, r := range refs {
		if r.MetaType != MetaTypeRichText && r.MetaType != MetaTypeMarkdown {
			out = append(out, r)

			continue
		}

		key, ok := f.urlMapper.URLToKey(r.Key)
		if !ok {
			continue
		}

		r.Key = key
		out = append(out, r)
	}

	return out
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

	keys := make([]string, len(refs))
	for i, ref := range refs {
		keys[i] = ref.Key
	}

	return f.ds.Schedule(ctx, tx, keys, reason)
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
