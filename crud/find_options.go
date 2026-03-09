package crud

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// FindOptions provides a fluent interface for building find options endpoints.
// Returns a simplified list of options (value, label, description) for dropdowns and selects.
type FindOptions[TModel, TSearch any] interface {
	api.OperationsProvider
	Find[TModel, TSearch, []DataOption, FindOptions[TModel, TSearch]]

	// WithDefaultColumnMapping sets fallback column mapping for value, label, and description columns.
	WithDefaultColumnMapping(mapping *DataOptionColumnMapping) FindOptions[TModel, TSearch]
}

// selectColumn adds a column to the SELECT clause, aliasing it if the name differs from the target.
func selectColumn(query orm.SelectQuery, column, target string) {
	if column == target {
		query.Select(column)
	} else {
		query.SelectAs(column, target)
	}
}

type findOptionsOperation[TModel, TSearch any] struct {
	Find[TModel, TSearch, []DataOption, FindOptions[TModel, TSearch]]

	defaultColumnMapping *DataOptionColumnMapping
}

func (a *findOptionsOperation[TModel, TSearch]) Provide() []api.OperationSpec {
	return []api.OperationSpec{a.Build(a.findOptions)}
}

// This mapping provides fallback values for column mapping when not explicitly specified in queries.
func (a *findOptionsOperation[TModel, TSearch]) WithDefaultColumnMapping(mapping *DataOptionColumnMapping) FindOptions[TModel, TSearch] {
	a.defaultColumnMapping = mapping

	return a
}

func (a *findOptionsOperation[TModel, TSearch]) findOptions(db orm.DB) (func(ctx fiber.Ctx, db orm.DB, config DataOptionConfig, search TSearch, meta api.Meta) error, error) {
	if err := a.Setup(db, defaultFindConfig); err != nil {
		return nil, err
	}

	table := db.TableOf((*TModel)(nil))

	return func(ctx fiber.Ctx, db orm.DB, config DataOptionConfig, search TSearch, meta api.Meta) error {
		var (
			options []DataOption
			query   = db.NewSelect().Model((*TModel)(nil))
		)

		mergeOptionColumnMapping(&config.DataOptionColumnMapping, a.defaultColumnMapping)

		if err := validateOptionColumns(table, &config.DataOptionColumnMapping); err != nil {
			return err
		}

		metaColumns := parseMetaColumns(config.MetaColumns)
		if err := validateMetaColumns(table, metaColumns); err != nil {
			return err
		}

		selectColumn(query, config.ValueColumn, ValueColumn)
		selectColumn(query, config.LabelColumn, LabelColumn)

		if config.DescriptionColumn != "" {
			selectColumn(query, config.DescriptionColumn, DescriptionColumn)
		}

		query.ApplyIf(len(metaColumns) > 0, func(sq orm.SelectQuery) {
			sq.SelectExpr(
				func(eb orm.ExprBuilder) any {
					return buildMetaJSONExpr(eb, metaColumns)
				},
				"meta",
			)
		})

		if err := a.ConfigureQuery(query, search, meta, ctx, QueryRoot); err != nil {
			return err
		}

		if err := query.Limit(maxOptionsLimit).
			Scan(ctx.Context(), &options); err != nil {
			return err
		}

		// Ensure empty slice instead of nil for consistent JSON response
		if options == nil {
			options = []DataOption{}
		}

		return result.Ok(a.Process(options, search, ctx)).Response(ctx)
	}, nil
}
