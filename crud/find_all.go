package crud

import (
	"github.com/coldsmirk/go-streams"
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/mold"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// FindAll provides a fluent interface for building find all endpoints.
// Returns all records matching the search criteria (with a safety limit).
type FindAll[TModel, TSearch any] interface {
	api.OperationsProvider
	Find[TModel, TSearch, []TModel, FindAll[TModel, TSearch]]
}

type findAllOperation[TModel, TSearch any] struct {
	Find[TModel, TSearch, []TModel, FindAll[TModel, TSearch]]
}

func (a *findAllOperation[TModel, TSearch]) Provide() []api.OperationSpec {
	return []api.OperationSpec{a.Build(a.findAll)}
}

func (a *findAllOperation[TModel, TSearch]) findAll(db orm.DB) (func(ctx fiber.Ctx, db orm.DB, transformer mold.Transformer, search TSearch, meta api.Meta) error, error) {
	if err := a.Setup(db, defaultFindConfig); err != nil {
		return nil, err
	}

	return func(ctx fiber.Ctx, db orm.DB, transformer mold.Transformer, search TSearch, meta api.Meta) error {
		var (
			models []TModel
			query  = db.NewSelect().Model(&models)
		)

		if err := a.ConfigureQuery(query, search, meta, ctx, QueryRoot); err != nil {
			return err
		}

		if err := query.SelectModelColumns().
			Limit(maxQueryLimit).
			Scan(ctx.Context()); err != nil {
			return err
		}

		if err := streams.Range(0, len(models)).ForEachErr(func(i int) error {
			return transformer.Struct(ctx.Context(), &models[i])
		}); err != nil {
			return err
		}

		if models == nil {
			models = []TModel{}
		}

		return result.Ok(a.Process(models, search, ctx)).Response(ctx)
	}, nil
}
