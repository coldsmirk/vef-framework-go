package sequence

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/internal/contract"
	"github.com/coldsmirk/vef-framework-go/sequence"
)

// Module provides the sequence generation functionality for the VEF framework.
//
// The in-memory store backs the Generator by default and is also exposed as its
// concrete *sequence.MemoryStore, so applications can inject it and seed rules via
// MemoryStore.Register during their own start-up hook.
//
// Applications can replace the backing store with a persistent or distributed one
// (e.g. sequence.NewDBStore or sequence.NewRedisStore) by decorating sequence.Store
// with vef.Decorate. When the active store implements contract.Initializer
// (as *sequence.DBStore does), initStore runs Init at start-up.
var Module = fx.Module(
	"vef:sequence",
	fx.Provide(
		fx.Annotate(
			sequence.NewMemoryStore,
			fx.As(fx.Self()),
			fx.As(new(sequence.Store)),
		),
		NewGenerator,
	),
	fx.Invoke(initStore),
)

// initStore initializes the active store during application start-up when it
// implements contract.Initializer (e.g. the DB-backed store creating its table).
func initStore(lc fx.Lifecycle, store sequence.Store) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			initializer, ok := store.(contract.Initializer)
			if !ok {
				return nil
			}

			if err := initializer.Init(ctx); err != nil {
				return fmt.Errorf("failed to initialize sequence store: %w", err)
			}

			return nil
		},
	})
}
