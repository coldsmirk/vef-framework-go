package crud

import (
	"fmt"

	"github.com/gofiber/fiber/v3"
	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/dbx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/tree"
)

// FindTreeOptions provides a fluent interface for building find tree options endpoints.
// Returns hierarchical options using recursive CTEs for tree-structured dropdowns.
type FindTreeOptions[TModel, TSearch any] interface {
	api.OperationsProvider
	Find[TModel, TSearch, []TreeDataOption, FindTreeOptions[TModel, TSearch]]

	// WithDefaultColumnMapping sets fallback column mapping for label, value, description, and sort columns.
	WithDefaultColumnMapping(mapping *DataOptionColumnMapping) FindTreeOptions[TModel, TSearch]
	// WithIDColumn sets the column used to identify individual tree nodes.
	WithIDColumn(name string) FindTreeOptions[TModel, TSearch]
	// WithParentIDColumn sets the column that establishes parent-child relationships between nodes.
	WithParentIDColumn(name string) FindTreeOptions[TModel, TSearch]
}

type findTreeOptionsOperation[TModel, TSearch any] struct {
	Find[TModel, TSearch, []TreeDataOption, FindTreeOptions[TModel, TSearch]]

	defaultColumnMapping *DataOptionColumnMapping
	idColumn             string
	parentIDColumn       string
}

func (a *findTreeOptionsOperation[TModel, TSearch]) Provide() []api.OperationSpec {
	return []api.OperationSpec{a.Build(a.findTreeOptions)}
}

func (a *findTreeOptionsOperation[TModel, TSearch]) WithDefaultColumnMapping(mapping *DataOptionColumnMapping) FindTreeOptions[TModel, TSearch] {
	a.defaultColumnMapping = mapping

	return a
}

func (a *findTreeOptionsOperation[TModel, TSearch]) WithIDColumn(name string) FindTreeOptions[TModel, TSearch] {
	a.idColumn = name

	return a
}

func (a *findTreeOptionsOperation[TModel, TSearch]) WithParentIDColumn(name string) FindTreeOptions[TModel, TSearch] {
	a.parentIDColumn = name

	return a
}

// WithCondition adds a WHERE condition using ConditionBuilder.
// Defaults to QueryBase for tree options unless specific parts are provided.
func (a *findTreeOptionsOperation[TModel, TSearch]) WithCondition(fn func(cb orm.ConditionBuilder), parts ...QueryPart) FindTreeOptions[TModel, TSearch] {
	a.Find.WithCondition(fn, lo.Ternary(len(parts) > 0, parts, []QueryPart{QueryBase})...)

	return a
}

// WithQueryApplier adds a custom query applier function.
// Defaults to QueryBase for tree options unless specific parts are provided.
func (a *findTreeOptionsOperation[TModel, TSearch]) WithQueryApplier(applier func(query orm.SelectQuery, search TSearch, ctx fiber.Ctx) error, parts ...QueryPart) FindTreeOptions[TModel, TSearch] {
	a.Find.WithQueryApplier(applier, lo.Ternary(len(parts) > 0, parts, []QueryPart{QueryBase})...)

	return a
}

func (a *findTreeOptionsOperation[TModel, TSearch]) findTreeOptions(db orm.DB) (func(ctx fiber.Ctx, db orm.DB, config DataOptionConfig, search TSearch, meta api.Meta) error, error) {
	if err := a.Setup(db, &FindOperationConfig{
		QueryParts: &QueryPartsConfig{
			Condition:         []QueryPart{QueryBase},
			Sort:              []QueryPart{QueryRoot},
			AuditUserRelation: []QueryPart{QueryBase, QueryRecursive},
		},
	}); err != nil {
		return nil, err
	}

	table := db.TableOf((*TModel)(nil))
	treeAdapter := tree.Adapter[TreeDataOption]{
		GetID:       func(t TreeDataOption) string { return t.ID },
		GetParentID: func(t TreeDataOption) *string { return t.ParentID },
		SetChildren: func(t *TreeDataOption, children []TreeDataOption) { t.Children = children },
	}

	if !table.HasField(a.idColumn) {
		return nil, fmt.Errorf("%w: column %q does not exist in model %T (tree node id)", errColumnNotFound, a.idColumn, (*TModel)(nil))
	}

	if !table.HasField(a.parentIDColumn) {
		return nil, fmt.Errorf("%w: column %q does not exist in model %T (parent reference)", errColumnNotFound, a.parentIDColumn, (*TModel)(nil))
	}

	return func(ctx fiber.Ctx, db orm.DB, config DataOptionConfig, search TSearch, meta api.Meta) error {
		var (
			flatOptions []TreeDataOption
			query       = db.NewSelect().Model((*TModel)(nil))
			cteErr      error
		)

		mergeOptionColumnMapping(&config.DataOptionColumnMapping, a.defaultColumnMapping)

		if err := validateOptionColumns(table, &config.DataOptionColumnMapping); err != nil {
			return err
		}

		metaColumns := parseMetaColumns(config.MetaColumns)
		if err := validateMetaColumns(table, metaColumns); err != nil {
			return err
		}

		// applyTreeColumns selects id and parent_id columns on a CTE sub-query.
		applyTreeColumns := func(q orm.SelectQuery) {
			selectColumn(q, a.idColumn, IDColumn)
			selectColumn(q, a.parentIDColumn, ParentIDColumn)
		}

		query.WithRecursive(
			"_tree", func(cteQuery orm.SelectQuery) {
				applyTreeColumns(cteQuery.Model((*TModel)(nil)))

				if err := a.ConfigureQuery(cteQuery, search, meta, ctx, QueryBase); err != nil {
					cteErr = err

					return
				}

				// Recursive part: find all ancestor nodes
				cteQuery.UnionAll(func(recursiveQuery orm.SelectQuery) {
					applyTreeColumns(recursiveQuery.Model((*TModel)(nil)))

					if err := a.ConfigureQuery(recursiveQuery, search, meta, ctx, QueryRecursive); err != nil {
						cteErr = err

						return
					}

					// Join with CTE to traverse the tree
					recursiveQuery.JoinTable(
						"_tree",
						func(cb orm.ConditionBuilder) {
							cb.EqualsColumn(a.idColumn, dbx.ColumnWithAlias(a.parentIDColumn, "_tree"))
						},
					)
				})
			}).
			With("_ids", func(query orm.SelectQuery) {
				query.Table("_tree").
					Select(IDColumn).
					Distinct()
			})

		if cteErr != nil {
			return cteErr
		}

		applyTreeColumns(query)
		selectColumn(query, config.LabelColumn, LabelColumn)
		selectColumn(query, config.ValueColumn, ValueColumn)

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

		query.Where(func(cb orm.ConditionBuilder) {
			cb.InSubQuery(a.idColumn, func(query orm.SelectQuery) {
				query.Table("_ids")
			})
		})

		if err := a.ConfigureQuery(query, search, meta, ctx, QueryRoot); err != nil {
			return err
		}

		if err := query.Limit(maxOptionsLimit).
			Scan(ctx.Context(), &flatOptions); err != nil {
			return err
		}

		treeOptions := tree.Build(flatOptions, treeAdapter)

		return result.Ok(a.Process(treeOptions, search, ctx)).Response(ctx)
	}, nil
}
