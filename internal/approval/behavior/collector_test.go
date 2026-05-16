package behavior

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
)

// itemA is a non-pointer payload type used to verify zero-value handling.
type itemA struct {
	Tag string
}

func TestCollectorAddDropsNil(t *testing.T) {
	t.Parallel()

	t.Run("PointerNilDropped", func(t *testing.T) {
		t.Parallel()

		var c Collector[*itemA]

		c.Add(nil, &itemA{Tag: "kept"}, nil)
		require.Len(t, c.Items(), 1, "Nil pointers should be skipped, non-nil kept")
		assert.Equal(t, "kept", c.Items()[0].Tag, "Should preserve the non-nil entry verbatim")
	})

	t.Run("ValueZeroKept", func(t *testing.T) {
		t.Parallel()

		var c Collector[itemA]

		c.Add(itemA{}, itemA{Tag: "real"})
		require.Len(t, c.Items(), 2, "Value-type zero is a legitimate item; Add should not drop it")
	})

	t.Run("InterfaceNilDropped", func(t *testing.T) {
		t.Parallel()

		var c Collector[any]

		var typed *itemA
		c.Add(nil, typed, "real")
		require.Len(t, c.Items(), 1, "Both untyped nil and typed-nil should be skipped")
		assert.Equal(t, "real", c.Items()[0], "Non-nil interface value should survive")
	})
}

func TestTryCollectorFromContext(t *testing.T) {
	t.Parallel()

	t.Run("MissingReturnsFalse", func(t *testing.T) {
		t.Parallel()

		_, ok := TryCollectorFromContext[*itemA](context.Background())
		assert.False(t, ok, "Empty ctx should report missing collector")
	})

	t.Run("RoundTrip", func(t *testing.T) {
		t.Parallel()

		ctx, c := installCollector[*itemA](context.Background())

		got, ok := TryCollectorFromContext[*itemA](ctx)
		require.True(t, ok, "Installed collector should be retrievable")
		assert.Same(t, c, got, "Retrieved collector pointer should match installed one")
	})

	t.Run("TypeIsolation", func(t *testing.T) {
		t.Parallel()

		ctx, _ := installCollector[*itemA](context.Background())

		_, ok := TryCollectorFromContext[string](ctx)
		assert.False(t, ok, "Different T should not retrieve another T's collector")
	})
}

// fakeAction implements cqrs.Action for direct collectorBehavior testing.
type fakeAction struct{ kind cqrs.ActionKind }

func (a fakeAction) Kind() cqrs.ActionKind { return a.kind }

func TestCollectorBehaviorFlushOnSuccess(t *testing.T) {
	t.Parallel()

	t.Run("FlushReceivesBufferedItems", func(t *testing.T) {
		t.Parallel()

		var flushed []*itemA

		b := &collectorBehavior[*itemA]{
			order: 100,
			name:  "test",
			flush: func(_ context.Context, items []*itemA) error {
				flushed = items

				return nil
			},
		}

		_, err := b.Handle(context.Background(), fakeAction{kind: cqrs.Command}, func(ctx context.Context) (any, error) {
			TryCollectorOrFail(t, ctx).Add(&itemA{Tag: "a"}, &itemA{Tag: "b"})

			return "ok", nil
		})

		require.NoError(t, err, "Handle should succeed")
		require.Len(t, flushed, 2, "Flush should receive every buffered item")
		assert.Equal(t, "a", flushed[0].Tag, "Should preserve insertion order")
	})

	t.Run("HandlerErrorSkipsFlush", func(t *testing.T) {
		t.Parallel()

		var flushed bool

		b := &collectorBehavior[*itemA]{
			order: 100,
			name:  "test",
			flush: func(context.Context, []*itemA) error {
				flushed = true

				return nil
			},
		}

		_, err := b.Handle(context.Background(), fakeAction{kind: cqrs.Command}, func(ctx context.Context) (any, error) {
			TryCollectorOrFail(t, ctx).Add(&itemA{Tag: "a"})

			return nil, errors.New("handler boom")
		})

		assert.Error(t, err, "Handler error should propagate")
		assert.False(t, flushed, "Failed handler must not trigger flush")
	})

	t.Run("QueryBypasses", func(t *testing.T) {
		t.Parallel()

		var flushed bool

		b := &collectorBehavior[*itemA]{
			order: 100,
			name:  "test",
			flush: func(context.Context, []*itemA) error {
				flushed = true

				return nil
			},
		}

		_, err := b.Handle(context.Background(), fakeAction{kind: cqrs.Query}, func(_ context.Context) (any, error) {
			return "query-result", nil
		})

		require.NoError(t, err, "Query bypass should succeed")
		assert.False(t, flushed, "Queries should not trigger flush")
	})
}

// TryCollectorOrFail is a tiny test helper that fails the test if the
// collector wasn't installed by the behavior.
func TryCollectorOrFail(t *testing.T, ctx context.Context) *Collector[*itemA] {
	t.Helper()

	c, ok := TryCollectorFromContext[*itemA](ctx)
	require.True(t, ok, "Collector should be installed in ctx by behavior")

	return c
}
