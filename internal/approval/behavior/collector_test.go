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

		ctx, c := installCollector[*ItemA](context.Background())

		got, ok := TryCollectorFromContext[*ItemA](ctx)
		require.True(t, ok, "Installed collector should be retrievable")
		assert.Same(t, c, got, "Retrieved collector pointer should match installed one")
	})

	t.Run("TypeIsolation", func(t *testing.T) {
		t.Parallel()

		ctx, _ := installCollector[*ItemA](context.Background())

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
