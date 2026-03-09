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

// Update provides a fluent interface for building update endpoints.
// Loads existing model, merges changes, and supports pre/post processing hooks.
type Update[TModel, TParams any] interface {
	api.OperationsProvider
	Builder[Update[TModel, TParams]]

	// WithPreUpdate registers a processor that is called before the model is updated in the database.
	WithPreUpdate(processor PreUpdateProcessor[TModel, TParams]) Update[TModel, TParams]
	// WithPostUpdate registers a processor that is called after the model is updated within the same transaction.
	WithPostUpdate(processor PostUpdateProcessor[TModel, TParams]) Update[TModel, TParams]
	// DisableDataPerm disables automatic data permission filtering for update queries.
	DisableDataPerm() Update[TModel, TParams]
}

type updateOperation[TModel, TParams any] struct {
	Builder[Update[TModel, TParams]]

	preUpdate        PreUpdateProcessor[TModel, TParams]
	postUpdate       PostUpdateProcessor[TModel, TParams]
	dataPermDisabled bool
}

func (u *updateOperation[TModel, TParams]) Provide() []api.OperationSpec {
	return []api.OperationSpec{u.Build(u.update)}
}

func (u *updateOperation[TModel, TParams]) WithPreUpdate(processor PreUpdateProcessor[TModel, TParams]) Update[TModel, TParams] {
	u.preUpdate = processor

	return u
}

func (u *updateOperation[TModel, TParams]) WithPostUpdate(processor PostUpdateProcessor[TModel, TParams]) Update[TModel, TParams] {
	u.postUpdate = processor

	return u
}

func (u *updateOperation[TModel, TParams]) DisableDataPerm() Update[TModel, TParams] {
	u.dataPermDisabled = true

	return u
}

func (u *updateOperation[TModel, TParams]) update(db orm.DB, sc storage.Service, publisher event.Publisher) (func(ctx fiber.Ctx, db orm.DB, params TParams) error, error) {
	promoter := storage.NewPromoter[TModel](sc, publisher)
	schema := db.TableOf((*TModel)(nil))
	pks := db.ModelPKFields((*TModel)(nil))

	if len(pks) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrModelNoPrimaryKey, schema.Name)
	}

	return func(ctx fiber.Ctx, db orm.DB, params TParams) error {
		var (
			oldModel   TModel
			model      TModel
			modelValue = reflect.ValueOf(&model).Elem()
		)

		if err := copier.Copy(&params, &model); err != nil {
			return err
		}

		for _, pk := range pks {
			pkValue, err := pk.Value(modelValue)
			if err != nil {
				return err
			}

			if reflect.ValueOf(pkValue).IsZero() {
				return result.Err(i18n.T("primary_key_required", map[string]any{"field": pk.Name}))
			}
		}

		query := db.NewSelect().Model(&model).WherePK()
		if !u.dataPermDisabled {
			if err := ApplyDataPermission(query, ctx); err != nil {
				return err
			}
		}

		if err := query.Scan(ctx.Context(), &oldModel); err != nil {
			return err
		}

		return db.RunInTX(ctx.Context(), func(txCtx context.Context, tx orm.DB) error {
			rollback := func() error { return promoter.Promote(txCtx, &model, &oldModel) }

			query := tx.NewUpdate().Model(&oldModel)
			if u.preUpdate != nil {
				if err := u.preUpdate(&oldModel, &model, &params, query, ctx, tx); err != nil {
					return err
				}
			}

			if err := copier.Copy(&model, &oldModel, copier.WithIgnoreEmpty()); err != nil {
				return err
			}

			if err := promoter.Promote(txCtx, &oldModel, &model); err != nil {
				return fmt.Errorf("promote files failed: %w", err)
			}

			if _, err := query.WherePK().Exec(txCtx); err != nil {
				return withCleanup(err, rollback)
			}

			if u.postUpdate != nil {
				if err := u.postUpdate(&oldModel, &model, &params, ctx, tx); err != nil {
					return withCleanup(err, rollback)
				}
			}

			return result.Ok().Response(ctx)
		})
	}, nil
}
