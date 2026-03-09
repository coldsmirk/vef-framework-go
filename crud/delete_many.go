package crud

import (
	"context"
	"fmt"
	"reflect"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/storage"
)

// DeleteMany provides a fluent interface for building batch delete endpoints.
// Deletes multiple models atomically with validation and pre/post hooks.
type DeleteMany[TModel any] interface {
	api.OperationsProvider
	Builder[DeleteMany[TModel]]

	// WithPreDeleteMany registers a processor that is called before the models are deleted from the database.
	WithPreDeleteMany(processor PreDeleteManyProcessor[TModel]) DeleteMany[TModel]
	// WithPostDeleteMany registers a processor that is called after the models are deleted within the same transaction.
	WithPostDeleteMany(processor PostDeleteManyProcessor[TModel]) DeleteMany[TModel]
	// DisableDataPerm disables automatic data permission filtering for batch delete queries.
	DisableDataPerm() DeleteMany[TModel]
}

type deleteManyOperation[TModel any] struct {
	Builder[DeleteMany[TModel]]

	preDeleteMany    PreDeleteManyProcessor[TModel]
	postDeleteMany   PostDeleteManyProcessor[TModel]
	dataPermDisabled bool
}

func (d *deleteManyOperation[TModel]) Provide() []api.OperationSpec {
	return []api.OperationSpec{d.Build(d.deleteMany)}
}

func (d *deleteManyOperation[TModel]) WithPreDeleteMany(processor PreDeleteManyProcessor[TModel]) DeleteMany[TModel] {
	d.preDeleteMany = processor

	return d
}

func (d *deleteManyOperation[TModel]) WithPostDeleteMany(processor PostDeleteManyProcessor[TModel]) DeleteMany[TModel] {
	d.postDeleteMany = processor

	return d
}

func (d *deleteManyOperation[TModel]) DisableDataPerm() DeleteMany[TModel] {
	d.dataPermDisabled = true

	return d
}

func (d *deleteManyOperation[TModel]) deleteMany(db orm.DB, sc storage.Service, publisher event.Publisher) (func(ctx fiber.Ctx, db orm.DB, params DeleteManyParams) error, error) {
	promoter := storage.NewPromoter[TModel](sc, publisher)
	schema := db.TableOf((*TModel)(nil))
	pks := db.ModelPKFields((*TModel)(nil))

	if len(pks) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrModelNoPrimaryKey, schema.Name)
	}

	return func(ctx fiber.Ctx, db orm.DB, params DeleteManyParams) error {
		if len(params.PKs) == 0 {
			return result.Ok().Response(ctx)
		}

		models := make([]TModel, len(params.PKs))

		for i, pkValue := range params.PKs {
			modelValue := reflect.ValueOf(&models[i]).Elem()

			if pkMap, ok := pkValue.(map[string]any); ok {
				for _, pk := range pks {
					value, ok := pkMap[pk.Name]
					if !ok {
						return result.Err(i18n.T("primary_key_required", map[string]any{"field": pk.Name}))
					}

					if err := pk.Set(modelValue, value); err != nil {
						return err
					}
				}
			} else {
				if len(pks) != 1 {
					return result.Err(i18n.T("composite_primary_key_requires_map"))
				}

				if err := pks[0].Set(modelValue, pkValue); err != nil {
					return err
				}
			}

			query := db.NewSelect().Model(&models[i]).WherePK()
			if !d.dataPermDisabled {
				if err := ApplyDataPermission(query, ctx); err != nil {
					return err
				}
			}

			if err := query.Scan(ctx.Context(), &models[i]); err != nil {
				return err
			}
		}

		return db.RunInTX(ctx.Context(), func(txCtx context.Context, tx orm.DB) error {
			query := tx.NewDelete().Model(&models)
			if d.preDeleteMany != nil {
				if err := d.preDeleteMany(models, query, ctx, tx); err != nil {
					return err
				}
			}

			if _, err := query.WherePK().Exec(txCtx); err != nil {
				return err
			}

			if d.postDeleteMany != nil {
				if err := d.postDeleteMany(models, ctx, tx); err != nil {
					return err
				}
			}

			if cleanupErr := batchCleanup(txCtx, promoter, models); cleanupErr != nil {
				return fmt.Errorf("delete succeeded but cleanup files failed: %w", cleanupErr)
			}

			return result.Ok().Response(ctx)
		})
	}, nil
}
