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

// Create provides a fluent interface for building create endpoints.
// Supports pre/post processing hooks and transaction-based model creation.
type Create[TModel, TParams any] interface {
	api.OperationsProvider
	Builder[Create[TModel, TParams]]

	// WithPreCreate registers a processor that is called before the model is saved to the database.
	WithPreCreate(processor PreCreateProcessor[TModel, TParams]) Create[TModel, TParams]
	// WithPostCreate registers a processor that is called after the model is saved within the same transaction.
	WithPostCreate(processor PostCreateProcessor[TModel, TParams]) Create[TModel, TParams]
}

type createOperation[TModel, TParams any] struct {
	Builder[Create[TModel, TParams]]

	preCreate  PreCreateProcessor[TModel, TParams]
	postCreate PostCreateProcessor[TModel, TParams]
}

func (c *createOperation[TModel, TParams]) Provide() []api.OperationSpec {
	return []api.OperationSpec{c.Build(c.create)}
}

func (c *createOperation[TModel, TParams]) WithPreCreate(processor PreCreateProcessor[TModel, TParams]) Create[TModel, TParams] {
	c.preCreate = processor

	return c
}

func (c *createOperation[TModel, TParams]) WithPostCreate(processor PostCreateProcessor[TModel, TParams]) Create[TModel, TParams] {
	c.postCreate = processor

	return c
}

func (c *createOperation[TModel, TParams]) create(sc storage.Service, publisher event.Publisher) (func(ctx fiber.Ctx, db orm.DB, params TParams) error, error) {
	promoter := storage.NewPromoter[TModel](sc, publisher)

	return func(ctx fiber.Ctx, db orm.DB, params TParams) error {
		var model TModel
		if err := copier.Copy(&params, &model); err != nil {
			return err
		}

		return db.RunInTX(ctx.Context(), func(txCtx context.Context, tx orm.DB) error {
			cleanup := func() error { return promoter.Promote(txCtx, nil, &model) }

			query := tx.NewInsert().Model(&model)
			if c.preCreate != nil {
				if err := c.preCreate(&model, &params, query, ctx, tx); err != nil {
					return err
				}
			}

			if err := promoter.Promote(txCtx, &model, nil); err != nil {
				return fmt.Errorf("promote files failed: %w", err)
			}

			if _, err := query.Exec(txCtx); err != nil {
				return withCleanup(err, cleanup)
			}

			if c.postCreate != nil {
				if err := c.postCreate(&model, &params, ctx, tx); err != nil {
					return withCleanup(err, cleanup)
				}
			}

			pks, err := db.ModelPKs(&model)
			if err != nil {
				return withCleanup(err, cleanup)
			}

			return result.Ok(pks).Response(ctx)
		})
	}, nil
}
