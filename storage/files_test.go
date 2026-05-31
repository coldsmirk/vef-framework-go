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
	"github.com/coldsmirk/vef-framework-go/security"
	"github.com/coldsmirk/vef-framework-go/storage"
)

// testPrincipal is the default authorization subject for Files lifecycle
// tests. Tests that exercise the ownership boundary build their own.
var testPrincipal = &security.Principal{ID: "tester"}

// StripPrefixURLMapper is a test-only URLKeyMapper that strips a fixed
// prefix from embedded URLs (e.g. "/storage/files/") to recover the
// underlying storage key, and prepends it again on the way out. URLs
// missing the prefix are treated as not-managed (ok=false) so the
// mapper mirrors a realistic custom implementation that recognizes
// only its own URL shape.
type StripPrefixURLMapper struct{ prefix string }

func (m StripPrefixURLMapper) URLToKey(u string) (string, bool) {
	if !strings.HasPrefix(u, m.prefix) {
		return "", false
	}

	return strings.TrimPrefix(u, m.prefix), true
}

func (m StripPrefixURLMapper) KeyToURL(k string) string { return m.prefix + k }

// FileModel mirrors a typical business struct that mixes a scalar
// uploaded_file field with a richtext field carrying embedded resource
// URLs. Used by every TestFiles sub-case.
type FileModel struct {
	CoverKey string `meta:"uploaded_file"`
	Body     string `meta:"rich_text"`
}

// ConsumeCall captures one ClaimConsumer.Consume invocation.
type ConsumeCall struct {
	Tx        orm.DB
	Principal *security.Principal
	Keys      []string
}

// EnqueueCall captures one DeleteEnqueuer.Enqueue invocation.
type EnqueueCall struct {
	Tx     orm.DB
	Keys   []string
	Reason storage.DeleteReason
}

// MockClaimConsumer is the minimal stub Files needs to exercise the
// claim-consumption path.
type MockClaimConsumer struct {
	consumeCalls []ConsumeCall
	consumeErr   error
}

func (m *MockClaimConsumer) Consume(_ context.Context, tx orm.DB, principal *security.Principal, keys []string) error {
	m.consumeCalls = append(m.consumeCalls, ConsumeCall{
		Tx:        tx,
		Principal: principal,
		Keys:      append([]string(nil), keys...),
	})

	return m.consumeErr
}

// MockDeleteEnqueuer is the minimal stub Files needs to verify the
// delete-enqueue path.
type MockDeleteEnqueuer struct {
	enqueueCalls []EnqueueCall
	enqueueErr   error
}

func (m *MockDeleteEnqueuer) Enqueue(_ context.Context, tx orm.DB, keys []string, reason storage.DeleteReason) error {
	m.enqueueCalls = append(m.enqueueCalls, EnqueueCall{
		Tx:     tx,
		Keys:   append([]string(nil), keys...),
		Reason: reason,
	})

	return m.enqueueErr
}

// CapturedPublish records one Publish invocation: the event payload and
// the per-call PublishOption count, so tests can assert that the
// caller forwarded transactional options (typically WithTx) rather
// than firing a bare Publish.
//
// Note: PublishConfig.Tx defaults to a nil interface, so a stripped-
// WithTx call and a present-WithTx(nil) call resolve to the same
// post-fold value. Counting raw options is the only way the captured
// trace can distinguish the two cases in a unit test.
type CapturedPublish struct {
	Event   event.Event
	OptsLen int
}

// CapturingPublisher records every published event so test cases can
// assert which FileClaimedEvent payloads escaped Files. Implements
// event.Bus so storage.NewFiles accepts it directly in tests.
type CapturingPublisher struct {
	events []event.Event
	calls  []CapturedPublish
}

// Publish implements event.Bus.
func (p *CapturingPublisher) Publish(_ context.Context, e event.Event, opts ...event.PublishOption) error {
	p.events = append(p.events, e)
	p.calls = append(p.calls, CapturedPublish{Event: e, OptsLen: len(opts)})

	return nil
}

// PublishBatch implements event.Bus.
func (p *CapturingPublisher) PublishBatch(_ context.Context, evts []event.Event, opts ...event.PublishOption) error {
	for _, e := range evts {
		p.events = append(p.events, e)
		p.calls = append(p.calls, CapturedPublish{Event: e, OptsLen: len(opts)})
	}

	return nil
}

// Subscribe implements event.Bus with a no-op unsubscribe.
func (*CapturingPublisher) Subscribe(string, event.Handler, ...event.SubscribeOption) (event.Unsubscribe, error) {
	return func() {}, nil
}

// claimedKeys returns the FileClaimedEvent keys captured by p, in
// publication order, ignoring any non-claimed events.
func (p *CapturingPublisher) claimedKeys() []string {
	keys := make([]string, 0, len(p.events))

	for _, e := range p.events {
		if pe, ok := e.(*storage.FileClaimedEvent); ok {
			keys = append(keys, pe.FileKey)
		}
	}

	return keys
}

func newTestFiles() (*MockClaimConsumer, *MockDeleteEnqueuer, *CapturingPublisher, storage.Files) {
	cs := &MockClaimConsumer{}
	de := &MockDeleteEnqueuer{}
	pub := &CapturingPublisher{}

	return cs, de, pub, storage.NewFiles(cs, de, pub, new(storage.IdentityURLKeyMapper))
}

func TestFiles(t *testing.T) {
	t.Run("OnCreateConsumesClaimsAndPublishesClaimed", func(t *testing.T) {
		cs, de, pub, files := newTestFiles()
		model := &FileModel{CoverKey: "priv/cover.png"}

		require.NoError(t, files.OnCreate(context.Background(), nil, testPrincipal, model), "OnCreate should succeed")

		require.Len(t, cs.consumeCalls, 1, "OnCreate must invoke Consume exactly once")
		assert.Equal(t, []string{"priv/cover.png"}, cs.consumeCalls[0].Keys, "Consumed keys should match the model's uploaded_file ref")

		assert.Empty(t, de.enqueueCalls, "OnCreate must not enqueue deletions")

		assert.Equal(t, []string{"priv/cover.png"}, pub.claimedKeys(), "Each consumed key must trigger one FileClaimedEvent")
	})

	t.Run("OnCreateExtractsRichTextRefs", func(t *testing.T) {
		cs, _, pub, files := newTestFiles()
		model := &FileModel{
			CoverKey: "priv/cover.png",
			Body:     `<p><img src="priv/embed-1.png"><img src="priv/embed-2.png"></p>`,
		}

		require.NoError(t, files.OnCreate(context.Background(), nil, testPrincipal, model), "OnCreate should succeed for richtext content")

		require.Len(t, cs.consumeCalls, 1, "OnCreate must batch every reachable ref into one Consume call")
		assert.ElementsMatch(t,
			[]string{"priv/cover.png", "priv/embed-1.png", "priv/embed-2.png"},
			cs.consumeCalls[0].Keys,
			"All three refs (cover + 2 richtext URLs) should be consumed",
		)

		assert.ElementsMatch(t,
			[]string{"priv/cover.png", "priv/embed-1.png", "priv/embed-2.png"},
			pub.claimedKeys(),
			"All consumed keys should produce a FileClaimedEvent",
		)
	})

	t.Run("OnCreateNoRefsIsNoop", func(t *testing.T) {
		cs, de, pub, files := newTestFiles()

		require.NoError(t, files.OnCreate(context.Background(), nil, testPrincipal, &FileModel{}), "Empty model OnCreate should be a noop")

		assert.Empty(t, cs.consumeCalls, "Consume must not be called when the model has no refs")
		assert.Empty(t, de.enqueueCalls, "Enqueue must not be called either")
		assert.Empty(t, pub.events, "No events should be published when there is nothing to claim")
	})

	t.Run("OnCreateConsumeErrorPropagatesAndSuppressesEvents", func(t *testing.T) {
		cs, _, pub, files := newTestFiles()
		cs.consumeErr = errors.New("simulated transaction conflict")

		err := files.OnCreate(context.Background(), nil, testPrincipal, &FileModel{CoverKey: "priv/cover.png"})

		require.Error(t, err, "OnCreate must surface Consume failures")
		assert.ErrorIs(t, err, cs.consumeErr, "Returned error must wrap the Consume error")
		assert.Empty(t, pub.events, "No FileClaimedEvent must be published when Consume fails (event-on-success contract)")
	})

	t.Run("OnUpdateConsumesNewKeysAndEnqueuesReplaced", func(t *testing.T) {
		cs, de, pub, files := newTestFiles()

		old := &FileModel{CoverKey: "priv/old-cover.png"}
		updated := &FileModel{CoverKey: "priv/new-cover.png"}

		require.NoError(t, files.OnUpdate(context.Background(), nil, testPrincipal, old, updated), "OnUpdate should succeed")

		require.Len(t, cs.consumeCalls, 1, "Exactly one Consume should fire for the newly added ref")
		assert.Equal(t, []string{"priv/new-cover.png"}, cs.consumeCalls[0].Keys, "Only the new ref should be consumed")

		require.Len(t, de.enqueueCalls, 1, "Exactly one Enqueue should fire for the replaced ref")
		require.Len(t, de.enqueueCalls[0].Keys, 1, "Enqueue batch should carry the single replaced ref")

		assert.Equal(t, []string{"priv/old-cover.png"}, de.enqueueCalls[0].Keys, "Replaced batch should target the old key")
		assert.Equal(t, storage.DeleteReasonReplaced, de.enqueueCalls[0].Reason, "Replaced batch should carry the replaced reason")

		assert.Equal(t, []string{"priv/new-cover.png"}, pub.claimedKeys(), "Only newly added refs should produce FileClaimedEvent")
	})

	t.Run("OnUpdateNoChangeIsNoop", func(t *testing.T) {
		cs, de, pub, files := newTestFiles()
		model := &FileModel{CoverKey: "priv/same.png"}

		require.NoError(t, files.OnUpdate(context.Background(), nil, testPrincipal, model, model), "Identical snapshots should noop")

		assert.Empty(t, cs.consumeCalls, "Unchanged refs must not consume")
		assert.Empty(t, de.enqueueCalls, "Unchanged refs must not enqueue deletes")
		assert.Empty(t, pub.events, "Unchanged refs must not publish events")
	})

	t.Run("OnUpdateConsumeErrorSkipsEnqueueAndEvent", func(t *testing.T) {
		// Failure in Consume must short-circuit before enqueuing the
		// replaced delete; otherwise a rolled-back business tx would
		// still leave the queue with orphan rows.
		cs, de, pub, files := newTestFiles()
		cs.consumeErr = errors.New("consume failure")

		err := files.OnUpdate(
			context.Background(),
			nil,
			testPrincipal,
			&FileModel{CoverKey: "priv/old.png"},
			&FileModel{CoverKey: "priv/new.png"},
		)

		require.Error(t, err, "OnUpdate must propagate Consume failures")
		assert.Empty(t, de.enqueueCalls, "Enqueue must not run after Consume failure")
		assert.Empty(t, pub.events, "No event must be published after Consume failure")
	})

	t.Run("OnDeleteEnqueuesEveryRefWithDeletedReason", func(t *testing.T) {
		cs, de, pub, files := newTestFiles()
		model := &FileModel{
			CoverKey: "priv/cover.png",
			Body:     `<p><img src="priv/body-1.png"><img src="priv/body-2.png"></p>`,
		}

		require.NoError(t, files.OnDelete(context.Background(), nil, model), "OnDelete should succeed")

		assert.Empty(t, cs.consumeCalls, "OnDelete must not consume claims")

		require.Len(t, de.enqueueCalls, 1, "OnDelete should batch every ref into one Enqueue call")
		assert.Equal(t, storage.DeleteReasonDeleted, de.enqueueCalls[0].Reason, "Enqueue call should carry the deleted reason")

		assert.ElementsMatch(t,
			[]string{"priv/cover.png", "priv/body-1.png", "priv/body-2.png"},
			de.enqueueCalls[0].Keys,
			"Enqueued keys should cover every reachable ref",
		)

		assert.Empty(t, pub.events, "OnDelete must not publish FileClaimedEvent — claim events are for adoptions only")
	})

	t.Run("OnDeleteEmptyModelIsNoop", func(t *testing.T) {
		_, de, _, files := newTestFiles()

		require.NoError(t, files.OnDelete(context.Background(), nil, &FileModel{}), "Empty model OnDelete should be a noop")
		assert.Empty(t, de.enqueueCalls, "No refs means no Enqueue call")
	})

	t.Run("NilModelIsNoopAcrossAllHooks", func(t *testing.T) {
		cs, de, pub, files := newTestFiles()
		ctx := context.Background()

		require.NoError(t, files.OnCreate(ctx, nil, testPrincipal, (*FileModel)(nil)), "OnCreate(nil) must be a noop")
		require.NoError(t, files.OnDelete(ctx, nil, (*FileModel)(nil)), "OnDelete(nil) must be a noop")
		require.NoError(t, files.OnUpdate(ctx, nil, testPrincipal, (*FileModel)(nil), (*FileModel)(nil)), "OnUpdate(nil,nil) must be a noop")

		assert.Empty(t, cs.consumeCalls, "Nil hooks must not consume")
		assert.Empty(t, de.enqueueCalls, "Nil hooks must not enqueue")
		assert.Empty(t, pub.events, "Nil hooks must not publish")
	})

	t.Run("NilPublisherIsTolerated", func(t *testing.T) {
		// Regression guard: storage_resource and tests sometimes wire
		// Files with a nil publisher. publishClaimed must short-circuit
		// rather than NPE.
		cs := &MockClaimConsumer{}
		de := &MockDeleteEnqueuer{}
		files := storage.NewFiles(cs, de, nil, new(storage.IdentityURLKeyMapper))

		require.NoError(t, files.OnCreate(context.Background(), nil, testPrincipal, &FileModel{CoverKey: "priv/cover.png"}), "OnCreate with nil publisher must succeed")
		assert.Len(t, cs.consumeCalls, 1, "Consume must still run with a nil publisher")
	})

	t.Run("PublishCarriesTransactionalOption", func(t *testing.T) {
		// Regression guard: claimed events must flow through the outbox
		// transport in the caller's transaction. Files achieves this by
		// always passing event.WithTx(tx) to Publish. The CapturingPublisher
		// records the raw PublishOption count so we can fail loudly if
		// a future refactor drops the option.
		_, _, pub, files := newTestFiles()

		require.NoError(t,
			files.OnCreate(context.Background(), nil, testPrincipal,
				&FileModel{CoverKey: "priv/cover.png"}),
			"OnCreate should succeed")

		require.Len(t, pub.calls, 1, "Exactly one Publish call expected")
		assert.GreaterOrEqual(t, pub.calls[0].OptsLen, 1,
			"PublishClaimed must forward at least one PublishOption (event.WithTx)")
	})

	// P0-1 regression guards: rich_text / markdown URLs must be translated
	// through the URLKeyMapper into storage keys before Consume sees
	// them. Without the mapper, an embedded "/storage/files/foo.png"
	// would never match the "foo.png" row in sys_storage_upload_claim.

	t.Run("URLKeyMapperRewritesRichtextKeysBeforeConsume", func(t *testing.T) {
		cs := &MockClaimConsumer{}
		de := &MockDeleteEnqueuer{}
		mapper := StripPrefixURLMapper{prefix: "/storage/files/"}
		files := storage.NewFiles(cs, de, nil, mapper)

		model := &FileModel{
			CoverKey: "priv/cover.png",
			Body: `<img src="/storage/files/priv/embed-1.png">` +
				`<img src="/storage/files/priv/embed-2.png">`,
		}

		require.NoError(t, files.OnCreate(context.Background(), nil, testPrincipal, model), "OnCreate must succeed when mapper rewrites richtext URLs")
		require.Len(t, cs.consumeCalls, 1, "Consume must be invoked exactly once")
		assert.ElementsMatch(t,
			[]string{"priv/cover.png", "priv/embed-1.png", "priv/embed-2.png"},
			cs.consumeCalls[0].Keys,
			"Richtext URLs must be stripped of the proxy prefix before reaching Consume",
		)
	})

	t.Run("URLKeyMapperLeavesUploadedFileRefsUntouched", func(t *testing.T) {
		cs := &MockClaimConsumer{}
		de := &MockDeleteEnqueuer{}
		// A mapper that would rewrite EVERY input. uploaded_file values
		// already are storage keys, so they must bypass the mapper.
		mapper := StripPrefixURLMapper{prefix: "priv/"}
		files := storage.NewFiles(cs, de, nil, mapper)

		require.NoError(t, files.OnCreate(context.Background(), nil, testPrincipal, &FileModel{CoverKey: "priv/cover.png"}), "OnCreate must succeed")
		require.Len(t, cs.consumeCalls, 1, "Consume must be invoked exactly once")
		assert.Equal(t,
			[]string{"priv/cover.png"},
			cs.consumeCalls[0].Keys,
			"Uploaded_file refs must pass through unchanged even with a non-identity mapper",
		)
	})

	t.Run("URLKeyMapperDiffsOnMappedKeysDuringOnUpdate", func(t *testing.T) {
		cs := &MockClaimConsumer{}
		de := &MockDeleteEnqueuer{}
		mapper := StripPrefixURLMapper{prefix: "/storage/files/"}
		files := storage.NewFiles(cs, de, nil, mapper)

		// Same underlying key; only the URL prefix changes (e.g. proxy
		// path swapped for a CDN URL the frontend computed differently).
		// After mapping, both sides resolve to the same storage key, so
		// nothing should be consumed and nothing should be deleted.
		old := &FileModel{Body: `<img src="/storage/files/priv/same.png">`}
		updated := &FileModel{Body: `<img src="/storage/files/priv/same.png">`}

		require.NoError(t, files.OnUpdate(context.Background(), nil, testPrincipal, old, updated), "Identity update on mapped keys must be a noop")
		assert.Empty(t, cs.consumeCalls, "No new keys → no Consume call")
		assert.Empty(t, de.enqueueCalls, "No removed keys → no Enqueue call")
	})

	t.Run("IdentityMapperDropsAbsoluteURLsInRichtext", func(t *testing.T) {
		// Regression for the P0-1 contract change: the extractor now
		// surfaces http(s) URLs to the mapper, and the default
		// IdentityURLKeyMapper rejects them (ok=false). The result is
		// the same observable outcome as before — absolute URLs do
		// not reach Consume — but the decision now happens in
		// the mapper instead of the extractor, so a business module
		// can override the behavior by supplying a custom mapper.
		cs, de, pub, files := newTestFiles()

		model := &FileModel{
			CoverKey: "priv/cover.png",
			Body: `<img src="https://cdn.example.com/abs.png">` +
				`<img src="priv/embed.png">`,
		}

		require.NoError(t, files.OnCreate(context.Background(), nil, testPrincipal, model), "OnCreate must succeed with mixed absolute / relative URLs")
		require.Len(t, cs.consumeCalls, 1, "Consume must be invoked exactly once")
		assert.ElementsMatch(t,
			[]string{"priv/cover.png", "priv/embed.png"},
			cs.consumeCalls[0].Keys,
			"Absolute http(s) URLs must be filtered out by IdentityURLKeyMapper before reaching Consume",
		)
		assert.ElementsMatch(t,
			[]string{"priv/cover.png", "priv/embed.png"},
			pub.claimedKeys(),
			"Claim events must mirror consumed keys; absolute URLs do not produce a FileClaimedEvent",
		)
		assert.Empty(t, de.enqueueCalls, "OnCreate must not enqueue deletes")
	})

	t.Run("CustomMapperResolvesCDNHostsToKeys", func(t *testing.T) {
		// Regression for the P0-1 contract change: business modules
		// can now supply a mapper that recognizes CDN URLs (or any
		// other http(s) URL shape) and resolves them back to storage
		// keys. The old extractor pre-filter dropped these URLs
		// before the mapper saw them; the new pipeline routes them
		// through URLToKey instead.
		cs := &MockClaimConsumer{}
		de := &MockDeleteEnqueuer{}
		mapper := StripPrefixURLMapper{prefix: "https://cdn.example.com/"}
		files := storage.NewFiles(cs, de, nil, mapper)

		model := &FileModel{
			CoverKey: "priv/cover.png",
			Body: `<img src="https://cdn.example.com/priv/cdn-1.png">` +
				`<img src="https://cdn.example.com/priv/cdn-2.png">` +
				`<img src="https://other.example.com/foreign.png">`,
		}

		require.NoError(t, files.OnCreate(context.Background(), nil, testPrincipal, model), "OnCreate must succeed with CDN URLs")
		require.Len(t, cs.consumeCalls, 1, "Consume must be invoked exactly once")
		assert.ElementsMatch(t,
			[]string{"priv/cover.png", "priv/cdn-1.png", "priv/cdn-2.png"},
			cs.consumeCalls[0].Keys,
			"CDN URLs must be resolved to storage keys; foreign-host URLs must be rejected by the mapper",
		)
		assert.Empty(t, de.enqueueCalls, "OnCreate must not enqueue deletes")
	})
}
