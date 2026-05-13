package storage_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/storage"
)

// stripPrefixURLMapper is a test-only URLKeyMapper that strips a fixed
// prefix from embedded URLs (e.g. "/storage/files/") to recover the
// underlying storage key, and prepends it again on the way out. URLs
// missing the prefix are treated as not-managed (ok=false) so the
// mapper mirrors a realistic custom implementation that recognizes
// only its own URL shape.
type stripPrefixURLMapper struct{ prefix string }

func (m stripPrefixURLMapper) URLToKey(u string) (string, bool) {
	if !strings.HasPrefix(u, m.prefix) {
		return "", false
	}

	return strings.TrimPrefix(u, m.prefix), true
}

func (m stripPrefixURLMapper) KeyToURL(k string) string { return m.prefix + k }

// FileModel mirrors a typical business struct that mixes a scalar
// uploaded_file field with a richtext field carrying embedded resource
// URLs. Used by every TestFiles sub-case.
type FileModel struct {
	CoverKey string `meta:"uploaded_file"`
	Body     string `meta:"richtext"`
}

// ConsumeManyCall captures one ConsumeMany invocation.
type ConsumeManyCall struct {
	Tx   orm.DB
	Keys []string
}

// ScheduleCall captures one DeleteScheduler.Schedule invocation.
type ScheduleCall struct {
	Tx     orm.DB
	Keys   []string
	Reason storage.DeleteReason
}

// MockClaimConsumer is the minimal stub Files needs to exercise the
// claim-consumption path.
type MockClaimConsumer struct {
	consumeManyCalls []ConsumeManyCall
	consumeManyErr   error
}

func (m *MockClaimConsumer) ConsumeMany(_ context.Context, tx orm.DB, keys []string) error {
	m.consumeManyCalls = append(m.consumeManyCalls, ConsumeManyCall{
		Tx:   tx,
		Keys: append([]string(nil), keys...),
	})

	return m.consumeManyErr
}

// MockDeleteScheduler is the minimal stub Files needs to verify the
// delete-scheduling path.
type MockDeleteScheduler struct {
	scheduleCalls []ScheduleCall
	scheduleErr   error
}

func (m *MockDeleteScheduler) Schedule(_ context.Context, tx orm.DB, keys []string, reason storage.DeleteReason) error {
	m.scheduleCalls = append(m.scheduleCalls, ScheduleCall{
		Tx:     tx,
		Keys:   append([]string(nil), keys...),
		Reason: reason,
	})

	return m.scheduleErr
}

// CapturingPublisher records every published event so test cases can
// assert which FilePromotedEvent payloads escaped Files.
type CapturingPublisher struct {
	events []event.Event
}

func (p *CapturingPublisher) Publish(e event.Event) {
	p.events = append(p.events, e)
}

// promotedKeys returns the FilePromotedEvent keys captured by p, in
// publication order, ignoring any non-promoted events.
func (p *CapturingPublisher) promotedKeys() []string {
	keys := make([]string, 0, len(p.events))

	for _, e := range p.events {
		if pe, ok := e.(*storage.FilePromotedEvent); ok {
			keys = append(keys, pe.FileKey)
		}
	}

	return keys
}

// scheduledKeys picks a single Schedule call's keys by index. Tests
// that call Schedule multiple times can index by call order.
func (m *MockDeleteScheduler) scheduledKeys(callIdx int) []string {
	if callIdx < 0 || callIdx >= len(m.scheduleCalls) {
		return nil
	}

	return m.scheduleCalls[callIdx].Keys
}

func newTestFiles() (*MockClaimConsumer, *MockDeleteScheduler, *CapturingPublisher, storage.Files) {
	cs := &MockClaimConsumer{}
	ds := &MockDeleteScheduler{}
	pub := &CapturingPublisher{}

	return cs, ds, pub, storage.NewFiles(cs, ds, pub, new(storage.IdentityURLKeyMapper))
}

func TestFiles(t *testing.T) {
	t.Run("OnCreateConsumesClaimsAndPublishesPromoted", func(t *testing.T) {
		cs, ds, pub, files := newTestFiles()
		model := &FileModel{CoverKey: "priv/cover.png"}

		require.NoError(t, files.OnCreate(context.Background(), nil, model), "OnCreate should succeed")

		require.Len(t, cs.consumeManyCalls, 1, "OnCreate must invoke ConsumeMany exactly once")
		assert.Equal(t, []string{"priv/cover.png"}, cs.consumeManyCalls[0].Keys, "Consumed keys should match the model's uploaded_file ref")

		assert.Empty(t, ds.scheduleCalls, "OnCreate must not schedule deletions")

		assert.Equal(t, []string{"priv/cover.png"}, pub.promotedKeys(), "Each consumed key must trigger one FilePromotedEvent")
	})

	t.Run("OnCreateExtractsRichTextRefs", func(t *testing.T) {
		cs, _, pub, files := newTestFiles()
		model := &FileModel{
			CoverKey: "priv/cover.png",
			Body:     `<p><img src="priv/embed-1.png"><img src="priv/embed-2.png"></p>`,
		}

		require.NoError(t, files.OnCreate(context.Background(), nil, model), "OnCreate should succeed for richtext content")

		require.Len(t, cs.consumeManyCalls, 1, "OnCreate must batch every reachable ref into one ConsumeMany call")
		assert.ElementsMatch(t,
			[]string{"priv/cover.png", "priv/embed-1.png", "priv/embed-2.png"},
			cs.consumeManyCalls[0].Keys,
			"All three refs (cover + 2 richtext URLs) should be consumed",
		)

		assert.ElementsMatch(t,
			[]string{"priv/cover.png", "priv/embed-1.png", "priv/embed-2.png"},
			pub.promotedKeys(),
			"All consumed keys should produce a FilePromotedEvent",
		)
	})

	t.Run("OnCreateNoRefsIsNoop", func(t *testing.T) {
		cs, ds, pub, files := newTestFiles()

		require.NoError(t, files.OnCreate(context.Background(), nil, &FileModel{}), "Empty model OnCreate should be a noop")

		assert.Empty(t, cs.consumeManyCalls, "ConsumeMany must not be called when the model has no refs")
		assert.Empty(t, ds.scheduleCalls, "Schedule must not be called either")
		assert.Empty(t, pub.events, "No events should be published when there is nothing to promote")
	})

	t.Run("OnCreateConsumeErrorPropagatesAndSuppressesEvents", func(t *testing.T) {
		cs, _, pub, files := newTestFiles()
		cs.consumeManyErr = errors.New("simulated transaction conflict")

		err := files.OnCreate(context.Background(), nil, &FileModel{CoverKey: "priv/cover.png"})

		require.Error(t, err, "OnCreate must surface ConsumeMany failures")
		assert.ErrorIs(t, err, cs.consumeManyErr, "Returned error must wrap the ConsumeMany error")
		assert.Empty(t, pub.events, "No FilePromotedEvent must be published when ConsumeMany fails (event-on-success contract)")
	})

	t.Run("OnUpdateConsumesNewKeysAndSchedulesReplaced", func(t *testing.T) {
		cs, ds, pub, files := newTestFiles()

		old := &FileModel{CoverKey: "priv/old-cover.png"}
		updated := &FileModel{CoverKey: "priv/new-cover.png"}

		require.NoError(t, files.OnUpdate(context.Background(), nil, old, updated), "OnUpdate should succeed")

		require.Len(t, cs.consumeManyCalls, 1, "Exactly one ConsumeMany should fire for the newly added ref")
		assert.Equal(t, []string{"priv/new-cover.png"}, cs.consumeManyCalls[0].Keys, "Only the new ref should be consumed")

		require.Len(t, ds.scheduleCalls, 1, "Exactly one Schedule should fire for the replaced ref")
		require.Len(t, ds.scheduleCalls[0].Keys, 1, "Schedule batch should carry the single replaced ref")

		assert.Equal(t, []string{"priv/old-cover.png"}, ds.scheduleCalls[0].Keys, "Replaced batch should target the old key")
		assert.Equal(t, storage.DeleteReasonReplaced, ds.scheduleCalls[0].Reason, "Replaced batch should carry the replaced reason")

		assert.Equal(t, []string{"priv/new-cover.png"}, pub.promotedKeys(), "Only newly added refs should produce FilePromotedEvent")
	})

	t.Run("OnUpdateNoChangeIsNoop", func(t *testing.T) {
		cs, ds, pub, files := newTestFiles()
		model := &FileModel{CoverKey: "priv/same.png"}

		require.NoError(t, files.OnUpdate(context.Background(), nil, model, model), "Identical snapshots should noop")

		assert.Empty(t, cs.consumeManyCalls, "Unchanged refs must not consume")
		assert.Empty(t, ds.scheduleCalls, "Unchanged refs must not schedule deletes")
		assert.Empty(t, pub.events, "Unchanged refs must not publish events")
	})

	t.Run("OnUpdateConsumeErrorSkipsScheduleAndEvent", func(t *testing.T) {
		// Failure in ConsumeMany must short-circuit before scheduling
		// the replaced delete; otherwise a rolled-back business tx
		// would still leave the queue with orphan rows.
		cs, ds, pub, files := newTestFiles()
		cs.consumeManyErr = errors.New("consume failure")

		err := files.OnUpdate(
			context.Background(),
			nil,
			&FileModel{CoverKey: "priv/old.png"},
			&FileModel{CoverKey: "priv/new.png"},
		)

		require.Error(t, err, "OnUpdate must propagate ConsumeMany failures")
		assert.Empty(t, ds.scheduleCalls, "Schedule must not run after ConsumeMany failure")
		assert.Empty(t, pub.events, "No event must be published after ConsumeMany failure")
	})

	t.Run("OnDeleteSchedulesEveryRefWithDeletedReason", func(t *testing.T) {
		cs, ds, pub, files := newTestFiles()
		model := &FileModel{
			CoverKey: "priv/cover.png",
			Body:     `<p><img src="priv/body-1.png"><img src="priv/body-2.png"></p>`,
		}

		require.NoError(t, files.OnDelete(context.Background(), nil, model), "OnDelete should succeed")

		assert.Empty(t, cs.consumeManyCalls, "OnDelete must not consume claims")

		require.Len(t, ds.scheduleCalls, 1, "OnDelete should batch every ref into one Schedule call")
		assert.Equal(t, storage.DeleteReasonDeleted, ds.scheduleCalls[0].Reason, "Schedule call should carry the deleted reason")

		assert.ElementsMatch(t,
			[]string{"priv/cover.png", "priv/body-1.png", "priv/body-2.png"},
			ds.scheduledKeys(0),
			"Scheduled keys should cover every reachable ref",
		)

		assert.Empty(t, pub.events, "OnDelete must not publish FilePromotedEvent — promotion is for adoptions only")
	})

	t.Run("OnDeleteEmptyModelIsNoop", func(t *testing.T) {
		_, ds, _, files := newTestFiles()

		require.NoError(t, files.OnDelete(context.Background(), nil, &FileModel{}), "Empty model OnDelete should be a noop")
		assert.Empty(t, ds.scheduleCalls, "No refs means no Schedule call")
	})

	t.Run("NilModelIsNoopAcrossAllHooks", func(t *testing.T) {
		cs, ds, pub, files := newTestFiles()
		ctx := context.Background()

		require.NoError(t, files.OnCreate(ctx, nil, (*FileModel)(nil)), "OnCreate(nil) must be a noop")
		require.NoError(t, files.OnDelete(ctx, nil, (*FileModel)(nil)), "OnDelete(nil) must be a noop")
		require.NoError(t, files.OnUpdate(ctx, nil, (*FileModel)(nil), (*FileModel)(nil)), "OnUpdate(nil,nil) must be a noop")

		assert.Empty(t, cs.consumeManyCalls, "Nil hooks must not consume")
		assert.Empty(t, ds.scheduleCalls, "Nil hooks must not schedule")
		assert.Empty(t, pub.events, "Nil hooks must not publish")
	})

	t.Run("NilPublisherIsTolerated", func(t *testing.T) {
		// Regression guard: storage_resource and tests sometimes wire
		// Files with a nil publisher. publishPromoted must short-circuit
		// rather than NPE.
		cs := &MockClaimConsumer{}
		ds := &MockDeleteScheduler{}
		files := storage.NewFiles(cs, ds, nil, new(storage.IdentityURLKeyMapper))

		require.NoError(t, files.OnCreate(context.Background(), nil, &FileModel{CoverKey: "priv/cover.png"}), "OnCreate with nil publisher must succeed")
		assert.Len(t, cs.consumeManyCalls, 1, "ConsumeMany must still run with a nil publisher")
	})

	// P0-1 regression guards: richtext / markdown URLs must be translated
	// through the URLKeyMapper into storage keys before ConsumeMany sees
	// them. Without the mapper, an embedded "/storage/files/foo.png"
	// would never match the "foo.png" row in sys_storage_upload_claim.

	t.Run("URLKeyMapperRewritesRichtextKeysBeforeConsume", func(t *testing.T) {
		cs := &MockClaimConsumer{}
		ds := &MockDeleteScheduler{}
		mapper := stripPrefixURLMapper{prefix: "/storage/files/"}
		files := storage.NewFiles(cs, ds, nil, mapper)

		model := &FileModel{
			CoverKey: "priv/cover.png",
			Body: `<img src="/storage/files/priv/embed-1.png">` +
				`<img src="/storage/files/priv/embed-2.png">`,
		}

		require.NoError(t, files.OnCreate(context.Background(), nil, model), "OnCreate must succeed when mapper rewrites richtext URLs")
		require.Len(t, cs.consumeManyCalls, 1, "ConsumeMany must be invoked exactly once")
		assert.ElementsMatch(t,
			[]string{"priv/cover.png", "priv/embed-1.png", "priv/embed-2.png"},
			cs.consumeManyCalls[0].Keys,
			"Richtext URLs must be stripped of the proxy prefix before reaching ConsumeMany",
		)
	})

	t.Run("URLKeyMapperLeavesUploadedFileRefsUntouched", func(t *testing.T) {
		cs := &MockClaimConsumer{}
		ds := &MockDeleteScheduler{}
		// A mapper that would rewrite EVERY input. uploaded_file values
		// already are storage keys, so they must bypass the mapper.
		mapper := stripPrefixURLMapper{prefix: "priv/"}
		files := storage.NewFiles(cs, ds, nil, mapper)

		require.NoError(t, files.OnCreate(context.Background(), nil, &FileModel{CoverKey: "priv/cover.png"}), "OnCreate must succeed")
		require.Len(t, cs.consumeManyCalls, 1, "ConsumeMany must be invoked exactly once")
		assert.Equal(t,
			[]string{"priv/cover.png"},
			cs.consumeManyCalls[0].Keys,
			"uploaded_file refs must pass through unchanged even with a non-identity mapper",
		)
	})

	t.Run("URLKeyMapperDiffsOnMappedKeysDuringOnUpdate", func(t *testing.T) {
		cs := &MockClaimConsumer{}
		ds := &MockDeleteScheduler{}
		mapper := stripPrefixURLMapper{prefix: "/storage/files/"}
		files := storage.NewFiles(cs, ds, nil, mapper)

		// Same underlying key; only the URL prefix changes (e.g. proxy
		// path swapped for a CDN URL the frontend computed differently).
		// After mapping, both sides resolve to the same storage key, so
		// nothing should be consumed and nothing should be deleted.
		old := &FileModel{Body: `<img src="/storage/files/priv/same.png">`}
		updated := &FileModel{Body: `<img src="/storage/files/priv/same.png">`}

		require.NoError(t, files.OnUpdate(context.Background(), nil, old, updated), "Identity update on mapped keys must be a noop")
		assert.Empty(t, cs.consumeManyCalls, "No new keys → no ConsumeMany call")
		assert.Empty(t, ds.scheduleCalls, "No removed keys → no Schedule call")
	})

	t.Run("IdentityMapperDropsAbsoluteURLsInRichtext", func(t *testing.T) {
		// Regression for the P0-1 contract change: the extractor now
		// surfaces http(s) URLs to the mapper, and the default
		// IdentityURLKeyMapper rejects them (ok=false). The result is
		// the same observable outcome as before — absolute URLs do
		// not reach ConsumeMany — but the decision now happens in
		// the mapper instead of the extractor, so a business module
		// can override the behavior by supplying a custom mapper.
		cs, ds, pub, files := newTestFiles()

		model := &FileModel{
			CoverKey: "priv/cover.png",
			Body: `<img src="https://cdn.example.com/abs.png">` +
				`<img src="priv/embed.png">`,
		}

		require.NoError(t, files.OnCreate(context.Background(), nil, model), "OnCreate must succeed with mixed absolute / relative URLs")
		require.Len(t, cs.consumeManyCalls, 1, "ConsumeMany must be invoked exactly once")
		assert.ElementsMatch(t,
			[]string{"priv/cover.png", "priv/embed.png"},
			cs.consumeManyCalls[0].Keys,
			"Absolute http(s) URLs must be filtered out by IdentityURLKeyMapper before reaching ConsumeMany",
		)
		assert.ElementsMatch(t,
			[]string{"priv/cover.png", "priv/embed.png"},
			pub.promotedKeys(),
			"Promotion events must mirror consumed keys; absolute URLs do not produce a FilePromotedEvent",
		)
		assert.Empty(t, ds.scheduleCalls, "OnCreate must not schedule deletes")
	})

	t.Run("CustomMapperResolvesCDNHostsToKeys", func(t *testing.T) {
		// Regression for the P0-1 contract change: business modules
		// can now supply a mapper that recognizes CDN URLs (or any
		// other http(s) URL shape) and resolves them back to storage
		// keys. The old extractor pre-filter dropped these URLs
		// before the mapper saw them; the new pipeline routes them
		// through URLToKey instead.
		cs := &MockClaimConsumer{}
		ds := &MockDeleteScheduler{}
		mapper := stripPrefixURLMapper{prefix: "https://cdn.example.com/"}
		files := storage.NewFiles(cs, ds, nil, mapper)

		model := &FileModel{
			CoverKey: "priv/cover.png",
			Body: `<img src="https://cdn.example.com/priv/cdn-1.png">` +
				`<img src="https://cdn.example.com/priv/cdn-2.png">` +
				`<img src="https://other.example.com/foreign.png">`,
		}

		require.NoError(t, files.OnCreate(context.Background(), nil, model), "OnCreate must succeed with CDN URLs")
		require.Len(t, cs.consumeManyCalls, 1, "ConsumeMany must be invoked exactly once")
		assert.ElementsMatch(t,
			[]string{"priv/cover.png", "priv/cdn-1.png", "priv/cdn-2.png"},
			cs.consumeManyCalls[0].Keys,
			"CDN URLs must be resolved to storage keys; foreign-host URLs must be rejected by the mapper",
		)
		assert.Empty(t, ds.scheduleCalls, "OnCreate must not schedule deletes")
	})
}
