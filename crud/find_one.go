package crud

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/mold"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// FindOne provides a fluent interface for building find one endpoints.
// Returns a single record matching the search criteria.
type FindOne[TModel, TSearch any] interface {
	api.OperationsProvider
	Find[TModel, TSearch, TModel, FindOne[TModel, TSearch]]
}

type findOneOperation[TModel, TSearch any] struct {
	Find[TModel, TSearch, TModel, FindOne[TModel, TSearch]]
}

func (a *findOneOperation[TModel, TSearch]) Provide() []api.OperationSpec {
	return []api.OperationSpec{a.Build(a.findOne)}
}

func (a *findOneOperation[TModel, TSearch]) findOne(db orm.DB) (func(ctx fiber.Ctx, db orm.DB, transformer mold.Transformer, search TSearch, meta api.Meta) error, error) {
	if err := a.Setup(db, defaultFindConfig); err != nil {
		return nil, err
	}

	return func(ctx fiber.Ctx, db orm.DB, transformer mold.Transformer, search TSearch, meta api.Meta) error {
		var (
			model TModel
			query = db.NewSelect().Model(&model)
		)

		if err := a.ConfigureQuery(query, search, meta, ctx, QueryRoot); err != nil {
			return err
		}

		// Limit to 1 record for efficiency
		if err := query.SelectModelColumns().
			Limit(1).
			Scan(ctx.Context()); err != nil {
			return err
		}

		if err := transformer.Struct(ctx.Context(), &model); err != nil {
			return err
		}

		return result.Ok(a.Process(model, search, ctx)).Response(ctx)
	}, nil
}
