package storage

import (
	"context"
	"reflect"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/event"
	ilogx "github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/reflectx"
	"github.com/coldsmirk/vef-framework-go/security"
)

var filesLogger = ilogx.Named("storage:files")

// Files is the high-level facade business handlers use to keep their
// `meta`-tagged file references in sync with the storage backend across
// the standard create / update / delete lifecycle.
//
// All three methods MUST be called inside a business transaction; the
// supplied tx is the same orm.DB instance passed to orm.DB.RunInTx, so
// the claim consumption and pending-delete bookkeeping commit or roll
// back atomically with the business write.
//
// The principal argument is the authorization subject for ownership
// checks: OnCreate / OnUpdate's `ConsumeMany` will only adopt claims
// created by this principal. Passing a nil or empty principal causes
// the call to fail with ErrAccessDenied. Background jobs / system
// paths that legitimately operate on behalf of "the system" need to
// pass a synthetic system principal; making the principal explicit at
// the call site keeps the authorization flow auditable.
//
// Internally Files composes ClaimConsumer + DeleteScheduler + a per-type
// meta field cache; callers do not interact with those primitives
// directly.
type Files interface {
	// OnCreate adopts every file reference reachable from model by
	// deleting the corresponding upload claim rows inside tx. Returns
	// ErrAccessDenied when principal does not own a referenced claim
	// (or when a claim is missing entirely — the two cases are folded
	// to avoid leaking existence across tenants).
	OnCreate(ctx context.Context, tx orm.DB, principal *security.Principal, model any) error

	// OnUpdate reconciles file references between two snapshots of the
	// same model: newly-referenced files are adopted (ConsumeMany);
	// dereferenced files are queued for asynchronous deletion with
	// DeleteReasonReplaced. Either model argument may be nil to signal
	// the absence of that side (mirrors FileRefExtractor.Diff
	// semantics).
	OnUpdate(ctx context.Context, tx orm.DB, principal *security.Principal, oldModel, newModel any) error

	// OnDelete schedules every file reference in model for asynchronous
	// deletion with DeleteReasonDeleted. The principal argument is
	// accepted for signature symmetry with OnCreate / OnUpdate but is
	// not consulted today — delete scheduling does not consume claims.
	OnDelete(ctx context.Context, tx orm.DB, model any) error
}

// NewFiles returns the default Files implementation, sharing the supplied
// ClaimConsumer, DeleteScheduler, event Publisher, and URLKeyMapper
// across all model types. The returned value is safe for concurrent
// use; meta field specs are parsed once per type on first access and
// cached for the lifetime of the instance.
//
// The URLKeyMapper translates rich_text / markdown URLs to storage keys
// during reconciliation. Pass IdentityURLKeyMapper{} (or nil, which is
// normalised to the identity mapper) when business code embeds bare
// keys directly in <img src> / ![](...).
//
// FileClaimedEvent is published through the outbox transport inside the
// caller's transaction (event.WithTx). Subscribers see the event iff
// the business transaction commits — no ghost events on rollback. The
// outbox relay forwards records to the configured sink with at-least-
// once semantics, so subscribers must still attach with event.WithGroup
// and rely on the Inbox middleware for dedupe.
func NewFiles(cc ClaimConsumer, ds DeleteScheduler, bus event.Bus, urlMapper URLKeyMapper) Files {
	if urlMapper == nil {
		urlMapper = new(IdentityURLKeyMapper)
	}

	return &defaultFiles{
		cc:        cc,
		ds:        ds,
		bus:       bus,
		urlMapper: urlMapper,
		cache:     collections.NewConcurrentHashMap[reflect.Type, *cachedExtractor](),
	}
}

type defaultFiles struct {
	cc        ClaimConsumer
	ds        DeleteScheduler
	bus       event.Bus
	urlMapper URLKeyMapper
	cache     collections.ConcurrentMap[reflect.Type, *cachedExtractor]
}

type cachedExtractor struct {
	fields []metaField
}

func (f *defaultFiles) OnCreate(ctx context.Context, tx orm.DB, principal *security.Principal, model any) error {
	ext := f.extractorFor(model)
	if ext == nil {
		return nil
	}

	return f.onCreateWith(ctx, tx, principal, model, ext)
}

func (f *defaultFiles) OnUpdate(ctx context.Context, tx orm.DB, principal *security.Principal, oldModel, newModel any) error {
	ext := f.extractorFor(newModel)
	if ext == nil {
		ext = f.extractorFor(oldModel)
	}

	if ext == nil {
		return nil
	}

	return f.onUpdateWith(ctx, tx, principal, oldModel, newModel, ext)
}

func (f *defaultFiles) onCreateWith(ctx context.Context, tx orm.DB, principal *security.Principal, model any, ext *cachedExtractor) error {
	refs := f.applyURLMapping(ext.extract(model))
	if len(refs) == 0 {
		return nil
	}

	keys := refKeys(refs)

	if err := f.cc.ConsumeMany(ctx, tx, principal, keys); err != nil {
		return err
	}

	f.publishClaimed(ctx, tx, keys)

	return nil
}

// onUpdateWith applies URL mapping before diffing so old/new ref sets
// compare on storage keys, not raw embedded URLs — a frontend that
// switches from "/storage/files/foo.png" to "https://cdn/foo.png" while
// the underlying key is unchanged must not look like a delete + re-create.
func (f *defaultFiles) onUpdateWith(ctx context.Context, tx orm.DB, principal *security.Principal, oldModel, newModel any, ext *cachedExtractor) error {
	oldRefs := f.applyURLMapping(ext.extract(oldModel))
	newRefs := f.applyURLMapping(ext.extract(newModel))

	toConsume, toDelete := diffRefs(newRefs, oldRefs)

	var consumedKeys []string

	if len(toConsume) > 0 {
		consumedKeys = refKeys(toConsume)

		if err := f.cc.ConsumeMany(ctx, tx, principal, consumedKeys); err != nil {
			return err
		}
	}

	if err := f.scheduleDeletes(ctx, tx, toDelete, DeleteReasonReplaced); err != nil {
		return err
	}

	f.publishClaimed(ctx, tx, consumedKeys)

	return nil
}

func refKeys(refs []FileRef) []string {
	keys := make([]string, len(refs))
	for i, ref := range refs {
		keys[i] = ref.Key
	}

	return keys
}

// publishClaimed routes every claimed key through the outbox transport
// inside the caller's business transaction. Tolerates a nil bus (tests
// wire defaultFiles without an event bus); callers must therefore always
// go through this helper rather than touching f.bus directly.
//
// A publish failure here is logged but not propagated: the outbox table
// participates in tx, so a tx commit later will fail and trigger a
// retry of the whole flow anyway. Surfacing the per-key error would
// only mask the eventual commit error with less useful context.
func (f *defaultFiles) publishClaimed(ctx context.Context, tx orm.DB, keys []string) {
	if f.bus == nil || len(keys) == 0 {
		return
	}

	for _, key := range keys {
		if err := f.bus.Publish(ctx, NewFileClaimedEvent(key), event.WithTx(tx)); err != nil {
			filesLogger.Warnf("publish file-claimed event for %s failed: %v", key, err)
		}
	}
}

func (f *defaultFiles) OnDelete(ctx context.Context, tx orm.DB, model any) error {
	ext := f.extractorFor(model)
	if ext == nil {
		return nil
	}

	return f.onDeleteWith(ctx, tx, model, ext)
}

func (f *defaultFiles) onDeleteWith(ctx context.Context, tx orm.DB, model any, ext *cachedExtractor) error {
	return f.scheduleDeletes(ctx, tx, f.applyURLMapping(ext.extract(model)), DeleteReasonDeleted)
}

// applyURLMapping rewrites rich_text / markdown ref keys through the
// configured URLKeyMapper so embedded URLs become storage object keys
// before they hit ClaimConsumer / DeleteScheduler. uploaded_file refs
// are passed through unchanged (they already hold storage keys, not URLs).
//
// A (key, ok=false) result drops the ref entirely: the URL refers to
// something outside this storage system (external CDN, mailto, data URI,
// bad input). Reconciliation must not touch unrelated objects.
//
// Always returns a freshly allocated slice — never aliases the input.
// The allocation is negligible (ref lists are short) and the explicit
// ownership boundary keeps future callers safe from a subtle bug where
// a cached input slice gets mutated by a downstream pipeline.
func (f *defaultFiles) applyURLMapping(refs []FileRef) []FileRef {
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

// extractorFor returns nil when model is nil or a typed nil pointer; the
// caller treats that as an empty ref set.
func (f *defaultFiles) extractorFor(model any) *cachedExtractor {
	if model == nil {
		return nil
	}

	rv := reflect.ValueOf(model)
	if rv.Kind() == reflect.Pointer && rv.IsNil() {
		return nil
	}

	return f.extractorForType(reflectx.Indirect(reflect.TypeOf(model)))
}

// extractorForType lets callers that know T statically (e.g. FilesFor[T])
// pre-resolve the spec once at construction instead of paying the cache
// lookup on every call.
func (f *defaultFiles) extractorForType(typ reflect.Type) *cachedExtractor {
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

	return f.ds.Schedule(ctx, tx, refKeys(refs), reason)
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
