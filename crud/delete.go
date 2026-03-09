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

// Delete provides a fluent interface for building delete endpoints.
// Validates primary key, loads model, and supports pre/post processing hooks.
type Delete[TModel any] interface {
	api.OperationsProvider
	Builder[Delete[TModel]]

	// WithPreDelete registers a processor that is called before the model is deleted from the database.
	WithPreDelete(processor PreDeleteProcessor[TModel]) Delete[TModel]
	// WithPostDelete registers a processor that is called after the model is deleted within the same transaction.
	WithPostDelete(processor PostDeleteProcessor[TModel]) Delete[TModel]
	// DisableDataPerm disables automatic data permission filtering for delete queries.
	DisableDataPerm() Delete[TModel]
}

type deleteOperation[TModel any] struct {
	Builder[Delete[TModel]]

	preDelete        PreDeleteProcessor[TModel]
	postDelete       PostDeleteProcessor[TModel]
	dataPermDisabled bool
}

func (d *deleteOperation[TModel]) Provide() []api.OperationSpec {
	return []api.OperationSpec{d.Build(d.delete)}
}

func (d *deleteOperation[TModel]) WithPreDelete(processor PreDeleteProcessor[TModel]) Delete[TModel] {
	d.preDelete = processor

	return d
}

func (d *deleteOperation[TModel]) WithPostDelete(processor PostDeleteProcessor[TModel]) Delete[TModel] {
	d.postDelete = processor

	return d
}

func (d *deleteOperation[TModel]) DisableDataPerm() Delete[TModel] {
	d.dataPermDisabled = true

	return d
}

func (d *deleteOperation[TModel]) delete(db orm.DB, sc storage.Service, publisher event.Publisher) (func(ctx fiber.Ctx, db orm.DB, params api.Params) error, error) {
	promoter := storage.NewPromoter[TModel](sc, publisher)
	schema := db.TableOf((*TModel)(nil))
	pks := db.ModelPKFields((*TModel)(nil))

	if len(pks) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrModelNoPrimaryKey, schema.Name)
	}

	return func(ctx fiber.Ctx, db orm.DB, params api.Params) error {
		var (
			model      TModel
			modelValue = reflect.ValueOf(&model).Elem()
		)

		for _, pk := range pks {
			value, ok := params[pk.Name]
			if !ok {
				return result.Err(i18n.T("primary_key_required", map[string]any{"field": pk.Name}))
			}

			if err := pk.Set(modelValue, value); err != nil {
				return err
			}
		}

		query := db.NewSelect().Model(&model).WherePK()
		if !d.dataPermDisabled {
			if err := ApplyDataPermission(query, ctx); err != nil {
				return err
			}
		}

		if err := query.Scan(ctx.Context(), &model); err != nil {
			return err
		}

		return db.RunInTX(ctx.Context(), func(txCtx context.Context, tx orm.DB) error {
			query := tx.NewDelete().Model(&model)
			if d.preDelete != nil {
				if err := d.preDelete(&model, query, ctx, tx); err != nil {
					return err
				}
			}

			if _, err := query.WherePK().Exec(txCtx); err != nil {
				return err
			}

			if d.postDelete != nil {
				if err := d.postDelete(&model, ctx, tx); err != nil {
					return err
				}
			}

			if err := promoter.Promote(txCtx, nil, &model); err != nil {
				return fmt.Errorf("delete succeeded but cleanup files failed: %w", err)
			}

			return result.Ok().Response(ctx)
		})
	}, nil
}
