package crud

import (
	"context"
	"fmt"
	"reflect"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/copier"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/storage"
)

// UpdateMany provides a fluent interface for building batch update endpoints.
// Updates multiple models atomically with validation, merge, and pre/post hooks.
type UpdateMany[TModel, TParams any] interface {
	api.OperationsProvider
	Builder[UpdateMany[TModel, TParams]]

	// WithPreUpdateMany registers a processor that is called before the models are updated in the database.
	WithPreUpdateMany(processor PreUpdateManyProcessor[TModel, TParams]) UpdateMany[TModel, TParams]
	// WithPostUpdateMany registers a processor that is called after the models are updated within the same transaction.
	WithPostUpdateMany(processor PostUpdateManyProcessor[TModel, TParams]) UpdateMany[TModel, TParams]
	// DisableDataPerm disables automatic data permission filtering for batch update queries.
	DisableDataPerm() UpdateMany[TModel, TParams]
}

type updateManyOperation[TModel, TParams any] struct {
	Builder[UpdateMany[TModel, TParams]]

	preUpdateMany    PreUpdateManyProcessor[TModel, TParams]
	postUpdateMany   PostUpdateManyProcessor[TModel, TParams]
	dataPermDisabled bool
}

func (u *updateManyOperation[TModel, TParams]) Provide() []api.OperationSpec {
	return []api.OperationSpec{u.Build(u.updateMany)}
}

func (u *updateManyOperation[TModel, TParams]) WithPreUpdateMany(processor PreUpdateManyProcessor[TModel, TParams]) UpdateMany[TModel, TParams] {
	u.preUpdateMany = processor

	return u
}

func (u *updateManyOperation[TModel, TParams]) WithPostUpdateMany(processor PostUpdateManyProcessor[TModel, TParams]) UpdateMany[TModel, TParams] {
	u.postUpdateMany = processor

	return u
}

func (u *updateManyOperation[TModel, TParams]) DisableDataPerm() UpdateMany[TModel, TParams] {
	u.dataPermDisabled = true

	return u
}

func (u *updateManyOperation[TModel, TParams]) updateMany(db orm.DB, sc storage.Service, publisher event.Publisher) (func(ctx fiber.Ctx, db orm.DB, params UpdateManyParams[TParams]) error, error) {
	promoter := storage.NewPromoter[TModel](sc, publisher)
	schema := db.TableOf((*TModel)(nil))
	pks := db.ModelPKFields((*TModel)(nil))

	if len(pks) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrModelNoPrimaryKey, schema.Name)
	}

	return func(ctx fiber.Ctx, db orm.DB, params UpdateManyParams[TParams]) error {
		if len(params.List) == 0 {
			return result.Ok().Response(ctx)
		}

		oldModels := make([]TModel, len(params.List))
		models := make([]TModel, len(params.List))

		for i := range params.List {
			if err := copier.Copy(&params.List[i], &models[i]); err != nil {
				return err
			}

			modelValue := reflect.ValueOf(&models[i]).Elem()
			for _, pk := range pks {
				pkValue, err := pk.Value(modelValue)
				if err != nil {
					return err
				}

				if reflect.ValueOf(pkValue).IsZero() {
					return result.Err(i18n.T("primary_key_required", map[string]any{"field": pk.Name}))
				}
			}

			query := db.NewSelect().Model(&models[i]).WherePK()
			if !u.dataPermDisabled {
				if err := ApplyDataPermission(query, ctx); err != nil {
					return err
				}
			}

			if err := query.Scan(ctx.Context(), &oldModels[i]); err != nil {
				return err
			}
		}

		return db.RunInTX(ctx.Context(), func(txCtx context.Context, tx orm.DB) error {
			n := len(oldModels)
			rollback := func() error { return batchRollback(txCtx, promoter, oldModels, models, n) }

			query := tx.NewUpdate().Model(&oldModels)

			if u.preUpdateMany != nil {
				if err := u.preUpdateMany(oldModels, models, params.List, query, ctx, tx); err != nil {
					return err
				}
			}

			for i := range models {
				if err := copier.Copy(&models[i], &oldModels[i], copier.WithIgnoreEmpty()); err != nil {
					return err
				}
			}

			for i := range oldModels {
				if err := promoter.Promote(txCtx, &oldModels[i], &models[i]); err != nil {
					err = fmt.Errorf("promote files for model %d failed: %w", i, err)

					return withCleanup(err, func() error { return batchRollback(txCtx, promoter, oldModels, models, i) })
				}
			}

			if _, err := query.Bulk().Exec(txCtx); err != nil {
				return withCleanup(err, rollback)
			}

			if u.postUpdateMany != nil {
				if err := u.postUpdateMany(oldModels, models, params.List, ctx, tx); err != nil {
					return withCleanup(err, rollback)
				}
			}

			return result.Ok().Response(ctx)
		})
	}, nil
}
