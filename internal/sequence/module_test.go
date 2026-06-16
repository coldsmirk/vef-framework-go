package sequence

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/sequence"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// StubStore is a minimal sequence.Store with no Init hook.
type StubStore struct{}

func (*StubStore) Reserve(context.Context, string, int, timex.DateTime) (*sequence.Rule, int, error) {
	return nil, 0, nil
}

// InitStubStore implements both sequence.Store and contract.Initializer (structurally),
// and reserves a deterministic rule so tests can prove the generator delegates to it.
type InitStubStore struct {
	initErr error
	inited  bool
}

func (s *InitStubStore) Init(context.Context) error {
	s.inited = true

	return s.initErr
}

func (*InitStubStore) Reserve(context.Context, string, int, timex.DateTime) (*sequence.Rule, int, error) {
	return &sequence.Rule{Prefix: "STUB-", SeqStep: 1, SeqLength: 2}, 7, nil
}

// FakeLifecycle captures appended hooks so a test can trigger OnStart directly.
type FakeLifecycle struct {
	hooks []fx.Hook
}

func (l *FakeLifecycle) Append(h fx.Hook) {
	l.hooks = append(l.hooks, h)
}

func TestInitStore(t *testing.T) {
	t.Run("CallsInitWhenInitializer", func(t *testing.T) {
		store := new(InitStubStore)
		lc := new(FakeLifecycle)

		initStore(lc, store)
		require.Len(t, lc.hooks, 1, "initStore should append exactly one lifecycle hook")

		require.NoError(t, lc.hooks[0].OnStart(context.Background()), "OnStart should succeed")
		assert.True(t, store.inited, "OnStart should call Init on a store implementing contract.Initializer")
	})

	t.Run("PropagatesInitError", func(t *testing.T) {
		wantErr := errors.New("boom")
		store := &InitStubStore{initErr: wantErr}
		lc := new(FakeLifecycle)

		initStore(lc, store)
		require.Len(t, lc.hooks, 1, "initStore should append exactly one lifecycle hook")

		err := lc.hooks[0].OnStart(context.Background())
		assert.ErrorIs(t, err, wantErr, "OnStart should propagate the store's Init error")
	})

	t.Run("NoopWhenNotInitializer", func(t *testing.T) {
		lc := new(FakeLifecycle)

		initStore(lc, new(StubStore))
		require.Len(t, lc.hooks, 1, "initStore should append a lifecycle hook even for non-initializer stores")

		assert.NoError(t, lc.hooks[0].OnStart(context.Background()), "OnStart should be a no-op for stores without Init")
	})
}

func TestModuleGraph(t *testing.T) {
	t.Run("DefaultBacksGeneratorWithMemoryStore", func(t *testing.T) {
		var (
			gen sequence.Generator
			mem *sequence.MemoryStore
		)

		app := fx.New(fx.NopLogger, Module, fx.Populate(&gen, &mem))
		require.NoError(t, app.Err(), "default sequence graph should build")

		ctx := context.Background()

		require.NoError(t, app.Start(ctx), "app should start")
		defer func() { require.NoError(t, app.Stop(ctx), "app should stop") }()

		require.NotNil(t, mem, "concrete *MemoryStore must remain injectable for rule seeding")
		mem.Register(&sequence.Rule{Key: "order", Prefix: "O", SeqStep: 1, SeqLength: 3, ResetCycle: sequence.ResetNone, IsActive: true})

		got, err := gen.GenerateN(ctx, "order", 2)
		require.NoError(t, err, "generation via the default in-memory store should succeed")
		assert.Equal(t, []string{"O001", "O002"}, got, "default generator should use the seeded in-memory rule")
	})

	t.Run("SupplySequenceStoreReplacesDefaultAndInitsOnStart", func(t *testing.T) {
		user := new(InitStubStore)

		var gen sequence.Generator

		app := fx.New(
			fx.NopLogger,
			Module,
			// An application override: decorate sequence.Store to replace the default.
			fx.Decorate(func() sequence.Store { return user }),
			fx.Populate(&gen),
		)
		require.NoError(t, app.Err(), "graph with a replacement store should build")

		ctx := context.Background()

		require.NoError(t, app.Start(ctx), "app should start")
		defer func() { require.NoError(t, app.Stop(ctx), "app should stop") }()

		assert.True(t, user.inited, "Init should run on start for the replacement store")

		got, err := gen.GenerateN(ctx, "anything", 1)
		require.NoError(t, err, "generation via the replacement store should succeed")
		assert.Equal(t, []string{"STUB-07"}, got, "generator should delegate to the replacement store")
	})
}
