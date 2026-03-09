package crud

import (
	"reflect"

	"github.com/coldsmirk/go-streams"
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/mold"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/page"
	"github.com/coldsmirk/vef-framework-go/result"
)

// FindPage provides a fluent interface for building find page endpoints.
// Returns paginated results with total count.
type FindPage[TModel, TSearch any] interface {
	api.OperationsProvider
	Find[TModel, TSearch, []TModel, FindPage[TModel, TSearch]]

	// WithDefaultPageSize sets the fallback page size when the request's page size is zero or invalid.
	WithDefaultPageSize(size int) FindPage[TModel, TSearch]
}

type findPageOperation[TModel, TSearch any] struct {
	Find[TModel, TSearch, []TModel, FindPage[TModel, TSearch]]

	defaultPageSize int
}

func (a *findPageOperation[TModel, TSearch]) Provide() []api.OperationSpec {
	return []api.OperationSpec{a.Build(a.findPage)}
}

// This value is used when the request's page size is zero or invalid.
func (a *findPageOperation[TModel, TSearch]) WithDefaultPageSize(size int) FindPage[TModel, TSearch] {
	a.defaultPageSize = size

	return a
}

func (a *findPageOperation[TModel, TSearch]) findPage(db orm.DB) (func(ctx fiber.Ctx, db orm.DB, transformer mold.Transformer, pageable page.Pageable, search TSearch, meta api.Meta) error, error) {
	if err := a.Setup(db, defaultFindConfig); err != nil {
		return nil, err
	}

	return func(ctx fiber.Ctx, db orm.DB, transformer mold.Transformer, pageable page.Pageable, search TSearch, meta api.Meta) (err error) {
		pageable.Normalize(a.defaultPageSize)

		var (
			models []TModel
			query  = db.NewSelect().Model(&models).SelectModelColumns().Paginate(pageable)
			total  int64
		)

		if err = a.ConfigureQuery(query, search, meta, ctx, QueryRoot); err != nil {
			return err
		}

		if total, err = query.ScanAndCount(ctx.Context()); err != nil {
			return err
		}

		if total == 0 {
			return result.Ok(page.New(pageable, total, []any{})).Response(ctx)
		}

		if err := streams.Range(0, len(models)).ForEachErr(func(i int) error {
			return transformer.Struct(ctx.Context(), &models[i])
		}); err != nil {
			return err
		}

		processed := a.Process(models, search, ctx)

		// Fast path: no processor or processor returned same type
		if typedModels, ok := processed.([]TModel); ok {
			return result.Ok(page.New(pageable, total, typedModels)).Response(ctx)
		}

		// Slow path: processor returned a different slice type, use reflection
		rv := reflect.Indirect(reflect.ValueOf(processed))
		if rv.Kind() != reflect.Slice {
			return result.Err(
				i18n.T(ErrMessageProcessorMustReturnSlice, map[string]any{"type": reflect.TypeOf(processed).String()}),
				result.WithCode(ErrCodeProcessorInvalidReturn),
				result.WithStatus(fiber.StatusInternalServerError),
			)
		}

		items := make([]any, rv.Len())
		for i := range items {
			items[i] = rv.Index(i).Interface()
		}

		return result.Ok(page.New(pageable, total, items)).Response(ctx)
	}, nil
}
