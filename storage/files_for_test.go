package storage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/security"
	"github.com/coldsmirk/vef-framework-go/storage"
)

// newTestFilesFor builds a FilesFor[FileModel] over fresh mocks. Mirrors
// newTestFiles so each sub-test starts from a clean baseline.
func newTestFilesFor() (*MockClaimConsumer, *MockDeleteEnqueuer, *CapturingPublisher, storage.FilesFor[FileModel]) {
	cs := &MockClaimConsumer{}
	de := &MockDeleteEnqueuer{}
	pub := &CapturingPublisher{}

	files := storage.NewFiles(cs, de, pub, new(storage.IdentityURLKeyMapper))

	return cs, de, pub, storage.NewFilesFor[FileModel](files)
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

func (r *RecordingStubFiles) OnCreate(context.Context, orm.DB, *security.Principal, any) error {
	r.createCalls++

	return nil
}

func (r *RecordingStubFiles) OnUpdate(context.Context, orm.DB, *security.Principal, any, any) error {
	r.updateCalls++

	return nil
}

func (r *RecordingStubFiles) OnDelete(context.Context, orm.DB, any) error {
	r.deleteCalls++

	return nil
}

func TestFilesFor(t *testing.T) {
	t.Run("OnCreateBehavesLikeUntypedFilesForSameModel", func(t *testing.T) {
		cs, de, pub, files := newTestFilesFor()
		model := &FileModel{
			CoverKey: "priv/cover.png",
			Body:     `<p><img src="priv/embed.png"></p>`,
		}

		require.NoError(t, files.OnCreate(context.Background(), nil, testPrincipal, model), "Typed OnCreate must succeed for the same input as untyped Files")

		require.Len(t, cs.consumeCalls, 1, "Typed OnCreate must batch every ref into one Consume call, matching untyped semantics")
		assert.ElementsMatch(t,
			[]string{"priv/cover.png", "priv/embed.png"},
			cs.consumeCalls[0].Keys,
			"Typed OnCreate must surface the same refs (cover + richtext) the untyped path would",
		)
		assert.Empty(t, de.enqueueCalls, "Typed OnCreate must not enqueue deletions, matching untyped semantics")
		assert.ElementsMatch(t,
			[]string{"priv/cover.png", "priv/embed.png"},
			pub.claimedKeys(),
			"Each consumed key must produce one FileClaimedEvent, matching untyped semantics",
		)
	})

	t.Run("OnUpdateConsumesNewAndEnqueuesReplaced", func(t *testing.T) {
		cs, de, pub, files := newTestFilesFor()
		old := &FileModel{CoverKey: "priv/old.png"}
		updated := &FileModel{CoverKey: "priv/new.png"}

		require.NoError(t, files.OnUpdate(context.Background(), nil, testPrincipal, old, updated), "Typed OnUpdate must succeed")

		require.Len(t, cs.consumeCalls, 1, "Typed OnUpdate must consume exactly the newly added ref")
		assert.Equal(t, []string{"priv/new.png"}, cs.consumeCalls[0].Keys, "Only the new ref should be consumed")

		require.Len(t, de.enqueueCalls, 1, "Typed OnUpdate must enqueue exactly the replaced ref")
		assert.Equal(t, []string{"priv/old.png"}, de.enqueueCalls[0].Keys, "Replaced batch should target the old key")
		assert.Equal(t, storage.DeleteReasonReplaced, de.enqueueCalls[0].Reason, "Replaced batch should carry the replaced reason")

		assert.Equal(t, []string{"priv/new.png"}, pub.claimedKeys(), "Only newly added refs should produce FileClaimedEvent")
	})

	t.Run("OnDeleteEnqueuesEveryRefWithDeletedReason", func(t *testing.T) {
		cs, de, pub, files := newTestFilesFor()
		model := &FileModel{
			CoverKey: "priv/cover.png",
			Body:     `<p><img src="priv/body.png"></p>`,
		}

		require.NoError(t, files.OnDelete(context.Background(), nil, model), "Typed OnDelete must succeed")

		assert.Empty(t, cs.consumeCalls, "Typed OnDelete must not consume claims")
		require.Len(t, de.enqueueCalls, 1, "Typed OnDelete must batch every ref into one Enqueue call")
		assert.Equal(t, storage.DeleteReasonDeleted, de.enqueueCalls[0].Reason, "Enqueue call should carry the deleted reason")
		assert.ElementsMatch(t,
			[]string{"priv/cover.png", "priv/body.png"},
			de.enqueueCalls[0].Keys,
			"Enqueued keys should cover every reachable ref",
		)
		assert.Empty(t, pub.events, "Typed OnDelete must not publish claim events")
	})

	t.Run("NilModelIsNoopAcrossAllHooks", func(t *testing.T) {
		cs, de, pub, files := newTestFilesFor()
		ctx := context.Background()

		require.NoError(t, files.OnCreate(ctx, nil, testPrincipal, nil), "Typed OnCreate(nil) must be a noop")
		require.NoError(t, files.OnDelete(ctx, nil, nil), "Typed OnDelete(nil) must be a noop")
		require.NoError(t, files.OnUpdate(ctx, nil, testPrincipal, nil, nil), "Typed OnUpdate(nil,nil) must be a noop")

		assert.Empty(t, cs.consumeCalls, "Nil hooks must not consume")
		assert.Empty(t, de.enqueueCalls, "Nil hooks must not enqueue")
		assert.Empty(t, pub.events, "Nil hooks must not publish")
	})

	t.Run("OnUpdateWithOneSideNilTreatedAsCreateOrDelete", func(t *testing.T) {
		// Mirrors the untyped Files contract: OnUpdate(nil, new) behaves
		// like a pure create (consume only); OnUpdate(old, nil) behaves
		// like a pure delete (enqueue only). Pre-resolution must not
		// change this.
		t.Run("NilOldIsPureCreate", func(t *testing.T) {
			cs, de, pub, files := newTestFilesFor()
			updated := &FileModel{CoverKey: "priv/added.png"}

			require.NoError(t, files.OnUpdate(context.Background(), nil, testPrincipal, nil, updated), "Typed OnUpdate(nil, new) must succeed")

			require.Len(t, cs.consumeCalls, 1, "Missing old side must consume new refs")
			assert.Equal(t, []string{"priv/added.png"}, cs.consumeCalls[0].Keys, "Only the new ref should be consumed")
			assert.Empty(t, de.enqueueCalls, "Missing old side must not enqueue deletions")
			assert.Equal(t, []string{"priv/added.png"}, pub.claimedKeys(), "New ref should produce a FileClaimedEvent")
		})

		t.Run("NilNewIsPureDelete", func(t *testing.T) {
			cs, de, pub, files := newTestFilesFor()
			old := &FileModel{CoverKey: "priv/removed.png"}

			require.NoError(t, files.OnUpdate(context.Background(), nil, testPrincipal, old, nil), "Typed OnUpdate(old, nil) must succeed")

			assert.Empty(t, cs.consumeCalls, "Missing new side must not consume")
			require.Len(t, de.enqueueCalls, 1, "Missing new side must enqueue the removed ref")
			assert.Equal(t, []string{"priv/removed.png"}, de.enqueueCalls[0].Keys, "Removed batch should target the old key")
			assert.Equal(t, storage.DeleteReasonReplaced, de.enqueueCalls[0].Reason, "Removed batch should carry the replaced reason (mirrors untyped OnUpdate)")
			assert.Empty(t, pub.events, "Pure delete must not publish claim events")
		})
	})

	t.Run("ConsumeErrorPropagatesAndSuppressesEvents", func(t *testing.T) {
		cs, _, pub, files := newTestFilesFor()
		cs.consumeErr = errors.New("simulated conflict")

		err := files.OnCreate(context.Background(), nil, testPrincipal, &FileModel{CoverKey: "priv/cover.png"})

		require.Error(t, err, "Typed OnCreate must surface Consume failures")
		assert.ErrorIs(t, err, cs.consumeErr, "Returned error must wrap the Consume error")
		assert.Empty(t, pub.events, "No FileClaimedEvent must be published when Consume fails (event-on-success contract)")
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
		require.NoError(t, typed.OnCreate(ctx, nil, testPrincipal, &FileModel{}), "OnCreate must delegate to the foreign Files")
		require.NoError(t, typed.OnUpdate(ctx, nil, testPrincipal, &FileModel{}, &FileModel{}), "OnUpdate must delegate to the foreign Files")
		require.NoError(t, typed.OnDelete(ctx, nil, &FileModel{}), "OnDelete must delegate to the foreign Files")

		assert.Equal(t, 1, stub.createCalls, "Foreign OnCreate must be invoked exactly once")
		assert.Equal(t, 1, stub.updateCalls, "Foreign OnUpdate must be invoked exactly once")
		assert.Equal(t, 1, stub.deleteCalls, "Foreign OnDelete must be invoked exactly once")
	})

	t.Run("BothFacadesExtractSameRefsForSameModel", func(t *testing.T) {
		// FilesFor[T] and the untyped Files reach the same per-type
		// extractor via the shared cache on *defaultFiles. This test
		// can only observe that contract indirectly: both facades must
		// agree on the extracted refs for an identical input. (A cache
		// split would also pass this assertion if both halves parsed
		// identically — for stronger guarantees, a future test could
		// expose the per-type entry pointer.)
		cs := &MockClaimConsumer{}
		de := &MockDeleteEnqueuer{}
		pub := &CapturingPublisher{}
		files := storage.NewFiles(cs, de, pub, new(storage.IdentityURLKeyMapper))

		typed := storage.NewFilesFor[FileModel](files)

		model := &FileModel{CoverKey: "priv/shared.png"}

		require.NoError(t, typed.OnCreate(context.Background(), nil, testPrincipal, model), "Typed OnCreate must succeed")
		require.NoError(t, files.OnCreate(context.Background(), nil, testPrincipal, model), "Untyped OnCreate must succeed after typed has populated the cache")

		require.Len(t, cs.consumeCalls, 2, "Each path must produce its own Consume call")
		assert.Equal(t, cs.consumeCalls[0].Keys, cs.consumeCalls[1].Keys, "Both facades must extract the same refs from the same model")
	})
}
