package behavior

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
)

// ItemA is a non-pointer payload type used to verify zero-value handling.
type ItemA struct {
	Tag string
}

// FakeAction implements cqrs.Action for direct collectorBehavior testing.
type FakeAction struct{ kind cqrs.ActionKind }

func (a FakeAction) Kind() cqrs.ActionKind { return a.kind }

// TryCollectorOrFail is a tiny test helper that fails the test if the
// collector wasn't installed by the behavior.
func TryCollectorOrFail(t *testing.T, ctx context.Context) *Collector[*ItemA] {
	t.Helper()

	c, ok := TryCollectorFromContext[*ItemA](ctx)
	require.True(t, ok, "Collector should be installed in ctx by behavior")

	return c
}

func TestCollectorAddDropsNil(t *testing.T) {
	t.Parallel()

	t.Run("PointerNilDropped", func(t *testing.T) {
		t.Parallel()

		var c Collector[*ItemA]

		c.Add(nil, &ItemA{Tag: "kept"}, nil)
		require.Len(t, c.Items(), 1, "Nil pointers should be skipped, non-nil kept")
		assert.Equal(t, "kept", c.Items()[0].Tag, "Collector should preserve the non-nil pointer item verbatim")
	})

	t.Run("ValueZeroKept", func(t *testing.T) {
		t.Parallel()

		var c Collector[ItemA]

		c.Add(ItemA{}, ItemA{Tag: "real"})
		require.Len(t, c.Items(), 2, "Value-type zero is a legitimate item; Add should not drop it")
	})

	t.Run("InterfaceNilDropped", func(t *testing.T) {
		t.Parallel()

		var c Collector[any]

		var typed *ItemA
		c.Add(nil, typed, "real")
		require.Len(t, c.Items(), 1, "Both untyped nil and typed-nil should be skipped")
		assert.Equal(t, "real", c.Items()[0], "Non-nil interface value should survive")
	})
}

func TestTryCollectorFromContext(t *testing.T) {
	t.Parallel()

	t.Run("MissingReturnsFalse", func(t *testing.T) {
		t.Parallel()

		_, ok := TryCollectorFromContext[*ItemA](context.Background())
		assert.False(t, ok, "Empty ctx should report missing collector")
	})

	t.Run("RoundTrip", func(t *testing.T) {
		t.Parallel()

		ctx, c, owned := installCollector[*ItemA](context.Background())
		require.True(t, owned, "First install should own the flush")

		got, ok := TryCollectorFromContext[*ItemA](ctx)
		require.True(t, ok, "Installed collector should be retrievable")
		assert.Same(t, c, got, "Retrieved collector pointer should match installed one")
	})

	t.Run("ReusesAnInstalledCollector", func(t *testing.T) {
		t.Parallel()

		outer, first, _ := installCollector[*ItemA](context.Background())

		inner, second, owned := installCollector[*ItemA](outer)
		assert.False(t, owned, "A nested install must decline ownership so the outermost one flushes")
		assert.Same(t, first, second, "A nested install must reuse the enclosing collector, not shadow it")
		assert.Same(t, outer, inner, "Reusing needs no derived context")
	})

	t.Run("TypeIsolation", func(t *testing.T) {
		t.Parallel()

		ctx, _, _ := installCollector[*ItemA](context.Background())

		_, ok := TryCollectorFromContext[string](ctx)
		assert.False(t, ok, "Different T should not retrieve another T's collector")
	})
}

func TestCollectorBehaviorFlushOnSuccess(t *testing.T) {
	t.Parallel()

	t.Run("FlushReceivesBufferedItems", func(t *testing.T) {
		t.Parallel()

		var flushed []*ItemA

		b := &collectorBehavior[*ItemA]{
			order: 100,
			name:  "test",
			flush: func(_ context.Context, items []*ItemA) error {
				flushed = items

				return nil
			},
		}

		_, err := b.Handle(context.Background(), FakeAction{kind: cqrs.Command}, func(ctx context.Context) (any, error) {
			TryCollectorOrFail(t, ctx).Add(&ItemA{Tag: "a"}, &ItemA{Tag: "b"})

			return "ok", nil
		})

		require.NoError(t, err, "Handle should succeed")
		require.Len(t, flushed, 2, "Flush should receive every buffered item")
		assert.Equal(t, "a", flushed[0].Tag, "Flush should preserve insertion order")
	})

	t.Run("HandlerErrorSkipsFlush", func(t *testing.T) {
		t.Parallel()

		var flushed bool

		b := &collectorBehavior[*ItemA]{
			order: 100,
			name:  "test",
			flush: func(context.Context, []*ItemA) error {
				flushed = true

				return nil
			},
		}

		_, err := b.Handle(context.Background(), FakeAction{kind: cqrs.Command}, func(ctx context.Context) (any, error) {
			TryCollectorOrFail(t, ctx).Add(&ItemA{Tag: "a"})

			return nil, errors.New("handler boom")
		})

		assert.Error(t, err, "Handler error should propagate")
		assert.False(t, flushed, "Failed handler must not trigger flush")
	})

	t.Run("QueryBypasses", func(t *testing.T) {
		t.Parallel()

		var flushed bool

		b := &collectorBehavior[*ItemA]{
			order: 100,
			name:  "test",
			flush: func(context.Context, []*ItemA) error {
				flushed = true

				return nil
			},
		}

		_, err := b.Handle(context.Background(), FakeAction{kind: cqrs.Query}, func(_ context.Context) (any, error) {
			return "query-result", nil
		})

		require.NoError(t, err, "Query bypass should succeed")
		assert.False(t, flushed, "Queries should not trigger flush")
	})
}

// TestCollectorBehaviorReentrantDispatch pins the ordering contract for a
// command dispatched from inside another one — which approval.Service makes
// reachable from host code, since an InstanceLifecycleHook fires in-transaction
// and may itself act on another task. A fresh inner collector would flush the
// inner command's items the moment its handler returned, publishing an effect
// ahead of the cause that produced it.
func TestCollectorBehaviorReentrantDispatch(t *testing.T) {
	t.Parallel()

	newBehavior := func(flushed *[][]*ItemA) *collectorBehavior[*ItemA] {
		return &collectorBehavior[*ItemA]{
			order: 100,
			name:  "test",
			flush: func(_ context.Context, items []*ItemA) error {
				*flushed = append(*flushed, items)

				return nil
			},
		}
	}

	t.Run("InnerItemsFlushOnceInOccurrenceOrder", func(t *testing.T) {
		t.Parallel()

		var flushed [][]*ItemA

		b := newBehavior(&flushed)

		_, err := b.Handle(context.Background(), FakeAction{kind: cqrs.Command}, func(ctx context.Context) (any, error) {
			TryCollectorOrFail(t, ctx).Add(&ItemA{Tag: "outer-before"})

			// The re-entrant dispatch a lifecycle hook performs.
			if _, err := b.Handle(ctx, FakeAction{kind: cqrs.Command}, func(ctx context.Context) (any, error) {
				TryCollectorOrFail(t, ctx).Add(&ItemA{Tag: "inner"})

				return "ok", nil
			}); err != nil {
				return nil, err
			}

			TryCollectorOrFail(t, ctx).Add(&ItemA{Tag: "outer-after"})

			return "ok", nil
		})

		require.NoError(t, err, "Handle should succeed")
		require.Len(t, flushed, 1, "Only the outermost dispatch may flush, or subscribers see an effect before its cause")

		tags := make([]string, 0, len(flushed[0]))
		for _, item := range flushed[0] {
			tags = append(tags, item.Tag)
		}

		assert.Equal(t, []string{"outer-before", "inner", "outer-after"}, tags, "One buffer must preserve occurrence order across the nesting")
	})

	t.Run("FailedInnerDispatchUnwindsItsItems", func(t *testing.T) {
		t.Parallel()

		var flushed [][]*ItemA

		b := newBehavior(&flushed)

		_, err := b.Handle(context.Background(), FakeAction{kind: cqrs.Command}, func(ctx context.Context) (any, error) {
			TryCollectorOrFail(t, ctx).Add(&ItemA{Tag: "outer"})

			// A hook that swallows the inner failure and carries on.
			_, _ = b.Handle(ctx, FakeAction{kind: cqrs.Command}, func(ctx context.Context) (any, error) {
				TryCollectorOrFail(t, ctx).Add(&ItemA{Tag: "inner"})

				return nil, errors.New("inner boom")
			})

			return "ok", nil
		})

		require.NoError(t, err, "The outer handler decides the outcome")
		require.Len(t, flushed, 1, "The outer dispatch still flushes")
		require.Len(t, flushed[0], 1, "Items buffered by a failed dispatch describe writes that did not happen")
		assert.Equal(t, "outer", flushed[0][0].Tag, "The enclosing command's own items must survive the inner failure")
	})
}
