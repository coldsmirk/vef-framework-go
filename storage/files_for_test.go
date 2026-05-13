package storage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/storage"
)

// newTestFilesFor builds a FilesFor[FileModel] over fresh mocks. Mirrors
// newTestFiles so each sub-test starts from a clean baseline.
func newTestFilesFor() (*MockClaimConsumer, *MockDeleteScheduler, *CapturingPublisher, storage.FilesFor[FileModel]) {
	cs := &MockClaimConsumer{}
	ds := &MockDeleteScheduler{}
	pub := &CapturingPublisher{}

	files := storage.NewFiles(cs, ds, pub, new(storage.IdentityURLKeyMapper))

	return cs, ds, pub, storage.NewFilesFor[FileModel](files)
}

// RecordingStubFiles is a foreign Files implementation used to verify
// that NewFilesFor falls back to the public Files interface when the
// underlying value does not expose the unexported fast-path surface.
// Each method records the call so tests can assert delegation.
type RecordingStubFiles struct {
	createCalls int
	updateCalls int
	deleteCalls int
}

func (r *RecordingStubFiles) OnCreate(context.Context, orm.DB, any) error {
	r.createCalls++

	return nil
}

func (r *RecordingStubFiles) OnUpdate(context.Context, orm.DB, any, any) error {
	r.updateCalls++

	return nil
}

func (r *RecordingStubFiles) OnDelete(context.Context, orm.DB, any) error {
	r.deleteCalls++

	return nil
}

func TestFilesFor(t *testing.T) {
	t.Run("OnCreateBehavesLikeUntypedFilesForSameModel", func(t *testing.T) {
		cs, ds, pub, files := newTestFilesFor()
		model := &FileModel{
			CoverKey: "priv/cover.png",
			Body:     `<p><img src="priv/embed.png"></p>`,
		}

		require.NoError(t, files.OnCreate(context.Background(), nil, model), "Typed OnCreate must succeed for the same input as untyped Files")

		require.Len(t, cs.consumeManyCalls, 1, "Typed OnCreate must batch every ref into one ConsumeMany call, matching untyped semantics")
		assert.ElementsMatch(t,
			[]string{"priv/cover.png", "priv/embed.png"},
			cs.consumeManyCalls[0].Keys,
			"Typed OnCreate must surface the same refs (cover + richtext) the untyped path would",
		)
		assert.Empty(t, ds.scheduleCalls, "Typed OnCreate must not schedule deletions, matching untyped semantics")
		assert.ElementsMatch(t,
			[]string{"priv/cover.png", "priv/embed.png"},
			pub.promotedKeys(),
			"Each consumed key must produce one FilePromotedEvent, matching untyped semantics",
		)
	})

	t.Run("OnUpdateConsumesNewAndSchedulesReplaced", func(t *testing.T) {
		cs, ds, pub, files := newTestFilesFor()
		old := &FileModel{CoverKey: "priv/old.png"}
		updated := &FileModel{CoverKey: "priv/new.png"}

		require.NoError(t, files.OnUpdate(context.Background(), nil, old, updated), "Typed OnUpdate must succeed")

		require.Len(t, cs.consumeManyCalls, 1, "Typed OnUpdate must consume exactly the newly added ref")
		assert.Equal(t, []string{"priv/new.png"}, cs.consumeManyCalls[0].Keys, "Only the new ref should be consumed")

		require.Len(t, ds.scheduleCalls, 1, "Typed OnUpdate must schedule exactly the replaced ref")
		assert.Equal(t, []string{"priv/old.png"}, ds.scheduleCalls[0].Keys, "Replaced batch should target the old key")
		assert.Equal(t, storage.DeleteReasonReplaced, ds.scheduleCalls[0].Reason, "Replaced batch should carry the replaced reason")

		assert.Equal(t, []string{"priv/new.png"}, pub.promotedKeys(), "Only newly added refs should produce FilePromotedEvent")
	})

	t.Run("OnDeleteSchedulesEveryRefWithDeletedReason", func(t *testing.T) {
		cs, ds, pub, files := newTestFilesFor()
		model := &FileModel{
			CoverKey: "priv/cover.png",
			Body:     `<p><img src="priv/body.png"></p>`,
		}

		require.NoError(t, files.OnDelete(context.Background(), nil, model), "Typed OnDelete must succeed")

		assert.Empty(t, cs.consumeManyCalls, "Typed OnDelete must not consume claims")
		require.Len(t, ds.scheduleCalls, 1, "Typed OnDelete must batch every ref into one Schedule call")
		assert.Equal(t, storage.DeleteReasonDeleted, ds.scheduleCalls[0].Reason, "Schedule call should carry the deleted reason")
		assert.ElementsMatch(t,
			[]string{"priv/cover.png", "priv/body.png"},
			ds.scheduledKeys(0),
			"Scheduled keys should cover every reachable ref",
		)
		assert.Empty(t, pub.events, "Typed OnDelete must not publish promotion events")
	})

	t.Run("NilModelIsNoopAcrossAllHooks", func(t *testing.T) {
		cs, ds, pub, files := newTestFilesFor()
		ctx := context.Background()

		require.NoError(t, files.OnCreate(ctx, nil, nil), "Typed OnCreate(nil) must be a noop")
		require.NoError(t, files.OnDelete(ctx, nil, nil), "Typed OnDelete(nil) must be a noop")
		require.NoError(t, files.OnUpdate(ctx, nil, nil, nil), "Typed OnUpdate(nil,nil) must be a noop")

		assert.Empty(t, cs.consumeManyCalls, "Nil hooks must not consume")
		assert.Empty(t, ds.scheduleCalls, "Nil hooks must not schedule")
		assert.Empty(t, pub.events, "Nil hooks must not publish")
	})

	t.Run("OnUpdateWithOneSideNilTreatedAsCreateOrDelete", func(t *testing.T) {
		// Mirrors the untyped Files contract: OnUpdate(nil, new) behaves
		// like a pure create (consume only); OnUpdate(old, nil) behaves
		// like a pure delete (schedule only). Pre-resolution must not
		// change this.
		t.Run("NilOldIsPureCreate", func(t *testing.T) {
			cs, ds, pub, files := newTestFilesFor()
			updated := &FileModel{CoverKey: "priv/added.png"}

			require.NoError(t, files.OnUpdate(context.Background(), nil, nil, updated), "Typed OnUpdate(nil, new) must succeed")

			require.Len(t, cs.consumeManyCalls, 1, "Missing old side must consume new refs")
			assert.Equal(t, []string{"priv/added.png"}, cs.consumeManyCalls[0].Keys, "Only the new ref should be consumed")
			assert.Empty(t, ds.scheduleCalls, "Missing old side must not schedule deletions")
			assert.Equal(t, []string{"priv/added.png"}, pub.promotedKeys(), "New ref should produce a FilePromotedEvent")
		})

		t.Run("NilNewIsPureDelete", func(t *testing.T) {
			cs, ds, pub, files := newTestFilesFor()
			old := &FileModel{CoverKey: "priv/removed.png"}

			require.NoError(t, files.OnUpdate(context.Background(), nil, old, nil), "Typed OnUpdate(old, nil) must succeed")

			assert.Empty(t, cs.consumeManyCalls, "Missing new side must not consume")
			require.Len(t, ds.scheduleCalls, 1, "Missing new side must schedule the removed ref")
			assert.Equal(t, []string{"priv/removed.png"}, ds.scheduleCalls[0].Keys, "Removed batch should target the old key")
			assert.Equal(t, storage.DeleteReasonReplaced, ds.scheduleCalls[0].Reason, "Removed batch should carry the replaced reason (mirrors untyped OnUpdate)")
			assert.Empty(t, pub.events, "Pure delete must not publish promotion events")
		})
	})

	t.Run("ConsumeErrorPropagatesAndSuppressesEvents", func(t *testing.T) {
		cs, _, pub, files := newTestFilesFor()
		cs.consumeManyErr = errors.New("simulated conflict")

		err := files.OnCreate(context.Background(), nil, &FileModel{CoverKey: "priv/cover.png"})

		require.Error(t, err, "Typed OnCreate must surface ConsumeMany failures")
		assert.ErrorIs(t, err, cs.consumeManyErr, "Returned error must wrap the ConsumeMany error")
		assert.Empty(t, pub.events, "No FilePromotedEvent must be published when ConsumeMany fails (event-on-success contract)")
	})

	t.Run("NewFilesForFallsBackToForeignFilesImplementation", func(t *testing.T) {
		// FilesFor only short-circuits the per-call cache lookup when it
		// can reach defaultFiles' unexported surface. Foreign Files
		// values (custom decorators, test fakes) are still accepted —
		// each lifecycle call is delegated to the public interface so
		// callers retain the typed signatures without losing the ability
		// to wrap Files via fx.Decorate.
		stub := &RecordingStubFiles{}
		typed := storage.NewFilesFor[FileModel](stub)

		ctx := context.Background()
		require.NoError(t, typed.OnCreate(ctx, nil, &FileModel{}), "OnCreate must delegate to the foreign Files")
		require.NoError(t, typed.OnUpdate(ctx, nil, &FileModel{}, &FileModel{}), "OnUpdate must delegate to the foreign Files")
		require.NoError(t, typed.OnDelete(ctx, nil, &FileModel{}), "OnDelete must delegate to the foreign Files")

		assert.Equal(t, 1, stub.createCalls, "Foreign OnCreate must be invoked exactly once")
		assert.Equal(t, 1, stub.updateCalls, "Foreign OnUpdate must be invoked exactly once")
		assert.Equal(t, 1, stub.deleteCalls, "Foreign OnDelete must be invoked exactly once")
	})

	t.Run("SharedCacheWithUntypedFiles", func(t *testing.T) {
		// FilesFor[T] must populate the same cache the untyped Files
		// reads from, so a typed facade constructed first does not force
		// the untyped path to re-parse the spec. We can observe this
		// indirectly: both facades must agree on the extracted refs for
		// an identical input.
		cs := &MockClaimConsumer{}
		ds := &MockDeleteScheduler{}
		pub := &CapturingPublisher{}
		files := storage.NewFiles(cs, ds, pub, new(storage.IdentityURLKeyMapper))

		typed := storage.NewFilesFor[FileModel](files)

		model := &FileModel{CoverKey: "priv/shared.png"}

		require.NoError(t, typed.OnCreate(context.Background(), nil, model), "Typed OnCreate must succeed")
		require.NoError(t, files.OnCreate(context.Background(), nil, model), "Untyped OnCreate must succeed after typed has populated the cache")

		require.Len(t, cs.consumeManyCalls, 2, "Each path must produce its own ConsumeMany call")
		assert.Equal(t, cs.consumeManyCalls[0].Keys, cs.consumeManyCalls[1].Keys, "Both facades must extract the same refs from the same model")
	})
}
