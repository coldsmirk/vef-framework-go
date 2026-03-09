package crud

import (
	"fmt"

	"github.com/coldsmirk/go-streams"
	"github.com/gofiber/fiber/v3"
	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/dbx"
	"github.com/coldsmirk/vef-framework-go/mold"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// FindTree provides a fluent interface for building find tree endpoints.
// Returns hierarchical data using recursive CTEs.
type FindTree[TModel, TSearch any] interface {
	api.OperationsProvider
	Find[TModel, TSearch, []TModel, FindTree[TModel, TSearch]]

	// WithIDColumn sets the column used to identify individual tree nodes.
	WithIDColumn(name string) FindTree[TModel, TSearch]
	// WithParentIDColumn sets the column that establishes parent-child relationships between nodes.
	WithParentIDColumn(name string) FindTree[TModel, TSearch]
}

type findTreeOperation[TModel, TSearch any] struct {
	Find[TModel, TSearch, []TModel, FindTree[TModel, TSearch]]

	idColumn       string
	parentIDColumn string
	treeBuilder    func(flatModels []TModel) []TModel
}

func (a *findTreeOperation[TModel, TSearch]) Provide() []api.OperationSpec {
	return []api.OperationSpec{a.Build(a.findTree)}
}

// This column is used to identify individual nodes and establish parent-child relationships.
func (a *findTreeOperation[TModel, TSearch]) WithIDColumn(name string) FindTree[TModel, TSearch] {
	a.idColumn = name

	return a
}

// This column establishes the hierarchical relationship between parent and child nodes.
func (a *findTreeOperation[TModel, TSearch]) WithParentIDColumn(name string) FindTree[TModel, TSearch] {
	a.parentIDColumn = name

	return a
}

// WithSelect adds a column to the SELECT clause.
// Defaults to QueryBase and QueryRecursive for tree queries unless specific parts are provided.
func (a *findTreeOperation[TModel, TSearch]) WithSelect(column string, parts ...QueryPart) FindTree[TModel, TSearch] {
	a.Find.WithSelect(column, lo.Ternary(len(parts) > 0, parts, []QueryPart{QueryBase, QueryRecursive})...)

	return a
}

// WithSelectAs adds a column with an alias to the SELECT clause.
// Defaults to QueryBase and QueryRecursive for tree queries unless specific parts are provided.
func (a *findTreeOperation[TModel, TSearch]) WithSelectAs(column, alias string, parts ...QueryPart) FindTree[TModel, TSearch] {
	a.Find.WithSelectAs(column, alias, lo.Ternary(len(parts) > 0, parts, []QueryPart{QueryBase, QueryRecursive})...)

	return a
}

// WithCondition adds a WHERE condition using ConditionBuilder.
// Defaults to QueryBase (filter starting nodes) unless specific parts are provided.
func (a *findTreeOperation[TModel, TSearch]) WithCondition(fn func(cb orm.ConditionBuilder), parts ...QueryPart) FindTree[TModel, TSearch] {
	a.Find.WithCondition(fn, lo.Ternary(len(parts) > 0, parts, []QueryPart{QueryBase})...)

	return a
}

// WithRelation adds a relation join to the query.
// Defaults to QueryBase and QueryRecursive for tree queries unless specific parts are provided.
func (a *findTreeOperation[TModel, TSearch]) WithRelation(relation *orm.RelationSpec, parts ...QueryPart) FindTree[TModel, TSearch] {
	a.Find.WithRelation(relation, lo.Ternary(len(parts) > 0, parts, []QueryPart{QueryBase, QueryRecursive})...)

	return a
}

// WithQueryApplier adds a custom query applier function.
// Defaults to QueryBase (apply during base CTE selection) unless specific parts are provided.
func (a *findTreeOperation[TModel, TSearch]) WithQueryApplier(applier func(query orm.SelectQuery, search TSearch, ctx fiber.Ctx) error, parts ...QueryPart) FindTree[TModel, TSearch] {
	a.Find.WithQueryApplier(applier, lo.Ternary(len(parts) > 0, parts, []QueryPart{QueryBase})...)

	return a
}

func (a *findTreeOperation[TModel, TSearch]) findTree(db orm.DB) (func(ctx fiber.Ctx, db orm.DB, transformer mold.Transformer, search TSearch, meta api.Meta) error, error) {
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
	if !table.HasField(a.idColumn) {
		return nil, fmt.Errorf("%w: column %q does not exist in model %T (tree node id)", ErrColumnNotFound, a.idColumn, (*TModel)(nil))
	}

	if !table.HasField(a.parentIDColumn) {
		return nil, fmt.Errorf("%w: column %q does not exist in model %T (parent reference)", ErrColumnNotFound, a.parentIDColumn, (*TModel)(nil))
	}

	return func(ctx fiber.Ctx, db orm.DB, transformer mold.Transformer, search TSearch, meta api.Meta) error {
		var (
			flatModels []TModel
			query      = db.NewSelect()
		)

		query.WithRecursive(
			"_tree", func(cteQuery orm.SelectQuery) {
				// Base query - the starting point of the tree traversal
				baseQuery := cteQuery.Model((*TModel)(nil)).SelectModelColumns()

				if err := a.ConfigureQuery(baseQuery, search, meta, ctx, QueryBase); err != nil {
					// Store error for later return
					SetQueryError(ctx, err)

					return
				}

				// Recursive part: find all ancestor/descendant nodes
				cteQuery.UnionAll(func(recursiveQuery orm.SelectQuery) {
					recursiveQuery.Model((*TModel)(nil)).SelectModelColumns()

					if err := a.ConfigureQuery(recursiveQuery, search, meta, ctx, QueryRecursive); err != nil {
						SetQueryError(ctx, err)

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
			Distinct().
			Table("_tree")

		if queryErr := QueryError(ctx); queryErr != nil {
			return queryErr
		}

		if err := a.ConfigureQuery(query, search, meta, ctx, QueryRoot); err != nil {
			return err
		}

		if err := query.Limit(maxQueryLimit).
			Scan(ctx.Context(), &flatModels); err != nil {
			return err
		}

		if err := streams.Range(0, len(flatModels)).ForEachErr(func(i int) error {
			return transformer.Struct(ctx.Context(), &flatModels[i])
		}); err != nil {
			return err
		}

		models := a.treeBuilder(flatModels)

		return result.Ok(a.Process(models, search, ctx)).Response(ctx)
	}, nil
}
