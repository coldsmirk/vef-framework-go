package crud

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/copier"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/storage"
)

// CreateMany provides a fluent interface for building batch create endpoints.
// Creates multiple models atomically in a single transaction with pre/post hooks.
type CreateMany[TModel, TParams any] interface {
	api.OperationsProvider
	Builder[CreateMany[TModel, TParams]]

	// WithPreCreateMany registers a processor that is called before the models are saved to the database.
	WithPreCreateMany(processor PreCreateManyProcessor[TModel, TParams]) CreateMany[TModel, TParams]
	// WithPostCreateMany registers a processor that is called after the models are saved within the same transaction.
	WithPostCreateMany(processor PostCreateManyProcessor[TModel, TParams]) CreateMany[TModel, TParams]
}

type createManyOperation[TModel, TParams any] struct {
	Builder[CreateMany[TModel, TParams]]

	preCreateMany  PreCreateManyProcessor[TModel, TParams]
	postCreateMany PostCreateManyProcessor[TModel, TParams]
}

func (c *createManyOperation[TModel, TParams]) Provide() []api.OperationSpec {
	return []api.OperationSpec{c.Build(c.createMany)}
}

func (c *createManyOperation[TModel, TParams]) WithPreCreateMany(processor PreCreateManyProcessor[TModel, TParams]) CreateMany[TModel, TParams] {
	c.preCreateMany = processor

	return c
}

func (c *createManyOperation[TModel, TParams]) WithPostCreateMany(processor PostCreateManyProcessor[TModel, TParams]) CreateMany[TModel, TParams] {
	c.postCreateMany = processor

	return c
}

func (c *createManyOperation[TModel, TParams]) createMany(sc storage.Service, publisher event.Publisher) (func(ctx fiber.Ctx, db orm.DB, params CreateManyParams[TParams]) error, error) {
	promoter := storage.NewPromoter[TModel](sc, publisher)

	return func(ctx fiber.Ctx, db orm.DB, params CreateManyParams[TParams]) error {
		if len(params.List) == 0 {
			return result.Ok([]map[string]any{}).Response(ctx)
		}

		models := make([]TModel, len(params.List))
		for i := range params.List {
			if err := copier.Copy(&params.List[i], &models[i]); err != nil {
				return err
			}
		}

		return db.RunInTX(ctx.Context(), func(txCtx context.Context, tx orm.DB) error {
			cleanup := func() error { return batchCleanup(txCtx, promoter, models) }

			query := tx.NewInsert().Model(&models)
			if c.preCreateMany != nil {
				if err := c.preCreateMany(models, params.List, query, ctx, tx); err != nil {
					return err
				}
			}

			for i := range models {
				if err := promoter.Promote(txCtx, &models[i], nil); err != nil {
					err = fmt.Errorf("promote files for model %d failed: %w", i, err)

					return withCleanup(err, func() error { return batchCleanup(txCtx, promoter, models[:i]) })
				}
			}

			if _, err := query.Exec(txCtx); err != nil {
				return withCleanup(err, cleanup)
			}

			if c.postCreateMany != nil {
				if err := c.postCreateMany(models, params.List, ctx, tx); err != nil {
					return withCleanup(err, cleanup)
				}
			}

			pks := make([]map[string]any, len(models))
			for i := range models {
				pk, err := db.ModelPKs(&models[i])
				if err != nil {
					return withCleanup(err, cleanup)
				}

				pks[i] = pk
			}

			return result.Ok(pks).Response(ctx)
		})
	}, nil
}
