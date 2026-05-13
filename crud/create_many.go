package crud

import (
	"context"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/copier"
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

func (c *createManyOperation[TModel, TParams]) createMany(files storage.Files) (func(ctx fiber.Ctx, db orm.DB, params CreateManyParams[TParams]) error, error) {
	typedFiles := storage.NewFilesFor[TModel](files)

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
			query := tx.NewInsert().Model(&models)
			if c.preCreateMany != nil {
				if err := c.preCreateMany(models, params.List, query, ctx, tx); err != nil {
					return err
				}
			}

			for i := range models {
				if err := typedFiles.OnCreate(txCtx, tx, &models[i]); err != nil {
					return err
				}
			}

			if _, err := query.Exec(txCtx); err != nil {
				return err
			}

			if c.postCreateMany != nil {
				if err := c.postCreateMany(models, params.List, ctx, tx); err != nil {
					return err
				}
			}

			pks := make([]map[string]any, len(models))
			for i := range models {
				pk, err := db.ModelPKs(&models[i])
				if err != nil {
					return err
				}

				pks[i] = pk
			}

			return result.Ok(pks).Response(ctx)
		})
	}, nil
}
