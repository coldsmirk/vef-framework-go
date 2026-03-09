package crud

import (
	"slices"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/sortx"
)

// Find provides a fluent interface for building find endpoints.
// All configuration is done through FindOperationOption passed to NewFindXxx constructors.
type Find[TModel, TSearch, TProcessorIn, TOperation any] interface {
	Builder[TOperation]

	// Setup initializes the find operation with framework-level options and validates configuration.
	// Must be called before query execution. Config specifies which QueryParts framework options apply to.
	Setup(db orm.DB, config *FindOperationConfig, opts ...*FindOperationOption) error
	// ConfigureQuery applies FindOperationOption for the specified QueryPart to the query.
	// Returns error if any option applier fails.
	ConfigureQuery(query orm.SelectQuery, search TSearch, meta api.Meta, ctx fiber.Ctx, part QueryPart) error
	// Process applies post-query processing to transform results.
	// Returns input unchanged if no Processor is configured.
	Process(input TProcessorIn, search TSearch, ctx fiber.Ctx) any

	// WithProcessor registers a post-query processor to transform or enrich results before returning.
	WithProcessor(processor Processor[TProcessorIn, TSearch]) TOperation
	// WithOptions appends custom FindOperationOption to the query configuration.
	WithOptions(opts ...*FindOperationOption) TOperation
	// WithSelect adds a column to the SELECT clause for specified query parts.
	WithSelect(column string, parts ...QueryPart) TOperation
	// WithSelectAs adds a column with an alias to the SELECT clause for specified query parts.
	WithSelectAs(column, alias string, parts ...QueryPart) TOperation
	// WithDefaultSort sets the default sort order when no dynamic sorting is provided in the request.
	WithDefaultSort(sort ...*sortx.OrderSpec) TOperation
	// WithCondition adds a WHERE condition using ConditionBuilder for specified query parts.
	WithCondition(fn func(cb orm.ConditionBuilder), parts ...QueryPart) TOperation
	// DisableDataPerm disables automatic data permission filtering for this endpoint.
	// IMPORTANT: Must be called before the API is registered (before Setup() is invoked).
	DisableDataPerm() TOperation
	// WithRelation adds a relation join to the query for specified query parts.
	WithRelation(relation *orm.RelationSpec, parts ...QueryPart) TOperation
	// WithAuditUserNames joins audit user model to populate creator/updater name fields.
	WithAuditUserNames(userModel any, nameColumn ...string) TOperation
	// WithQueryApplier adds a custom query modification function for specified query parts.
	WithQueryApplier(applier func(query orm.SelectQuery, search TSearch, ctx fiber.Ctx) error, parts ...QueryPart) TOperation
}

// defaultFindConfig is the default configuration for standard (non-tree) find operations.
// All options apply to QueryRoot only.
var defaultFindConfig = &FindOperationConfig{
	QueryParts: &QueryPartsConfig{
		Condition:         []QueryPart{QueryRoot},
		Sort:              []QueryPart{QueryRoot},
		AuditUserRelation: []QueryPart{QueryRoot},
	},
}

// baseFindOperation is the base implementation for all find operations.
// It provides a unified query configuration system using FindOperationOption.
type baseFindOperation[TModel, TSearch, TProcessorIn, TOperation any] struct {
	Builder[TOperation]

	setupDone           bool
	dataPermDisabled    bool
	options             []*FindOperationOption
	optionsByPart       map[QueryPart][]*FindOperationOption
	auditUserModel      any
	auditUserNameColumn string
	defaultSort         []*sortx.OrderSpec
	processor           Processor[TProcessorIn, TSearch]

	self TOperation
}

// Setup initializes the find operation with database and configuration.
// This method is called once in factory functions and is safe to call multiple times.
// Subsequent calls are no-ops.
func (a *baseFindOperation[TModel, TSearch, TProcessorIn, TOperation]) Setup(db orm.DB, config *FindOperationConfig, opts ...*FindOperationOption) error {
	if a.setupDone {
		return nil
	}
	defer func() { a.setupDone = true }()

	if config != nil && config.QueryParts != nil {
		if err := a.setupQueryParts(db, config.QueryParts); err != nil {
			return err
		}
	}

	a.options = append(a.options, opts...)

	// Pre-group options by QueryPart for efficient lookup in ConfigureQuery
	a.optionsByPart = make(map[QueryPart][]*FindOperationOption)
	for _, opt := range a.options {
		for _, part := range opt.Parts {
			a.optionsByPart[part] = append(a.optionsByPart[part], opt)
		}
	}

	return nil
}

// setupQueryParts configures query options based on QueryPartsConfig.
func (a *baseFindOperation[TModel, TSearch, TProcessorIn, TOperation]) setupQueryParts(db orm.DB, qp *QueryPartsConfig) error {
	table := db.TableOf((*TModel)(nil))

	condParts := partsOrDefault(qp.Condition)
	sortParts := partsOrDefault(qp.Sort)
	auditParts := partsOrDefault(qp.AuditUserRelation)

	a.options = append(a.options, withSearchApplier[TSearch](condParts...))

	if !a.dataPermDisabled {
		a.options = append(a.options, withDataPerm(condParts...))
	}

	if a.auditUserModel != nil {
		if err := a.setupAuditUserNames(table, auditParts); err != nil {
			return err
		}
	}

	a.setupDefaultSort(table, sortParts)

	return nil
}

// setupAuditUserNames validates and configures audit user name relations.
func (a *baseFindOperation[TModel, TSearch, TProcessorIn, TOperation]) setupAuditUserNames(table *orm.Table, parts []QueryPart) error {
	switch len(table.PKs) {
	case 0:
		return ErrModelNoPrimaryKey
	case 1:
		a.options = append(a.options, withAuditUserNames(a.auditUserModel, a.auditUserNameColumn, parts...))

		return nil
	default:
		return ErrAuditUserCompositePK
	}
}

// setupDefaultSort configures default sorting based on model schema.
func (a *baseFindOperation[TModel, TSearch, TProcessorIn, TOperation]) setupDefaultSort(table *orm.Table, parts []QueryPart) {
	switch {
	case a.defaultSort == nil && len(table.PKs) == 1:
		// Auto-sort by single primary key descending
		a.options = append(a.options, withSort([]*sortx.OrderSpec{
			{Column: table.PKs[0].Name, Direction: sortx.OrderDesc},
		}, parts...))
	case a.defaultSort == nil:
		// Fallback: sort by created_at if available
		if field, ok := table.FieldMap[orm.ColumnCreatedAt]; ok {
			a.options = append(a.options, withSort([]*sortx.OrderSpec{
				{Column: field.Name, Direction: sortx.OrderDesc},
			}, parts...))
		}

	case len(a.defaultSort) > 0:
		a.options = append(a.options, withSort(a.defaultSort, parts...))
	}
}

// partsOrDefault returns the given parts, or []QueryPart{QueryRoot} if nil.
func partsOrDefault(parts []QueryPart) []QueryPart {
	if parts != nil {
		return parts
	}

	return []QueryPart{QueryRoot}
}

// ConfigureQuery applies all query configuration options for the specified query part.
func (a *baseFindOperation[TModel, TSearch, TProcessorIn, TOperation]) ConfigureQuery(query orm.SelectQuery, search TSearch, meta api.Meta, ctx fiber.Ctx, part QueryPart) error {
	applied := make(map[*FindOperationOption]bool)

	for _, opts := range [2][]*FindOperationOption{a.optionsByPart[part], a.optionsByPart[QueryAll]} {
		for _, opt := range opts {
			if applied[opt] {
				continue
			}

			if err := opt.Applier(query, search, meta, ctx); err != nil {
				return err
			}

			applied[opt] = true
		}
	}

	return nil
}

// Process applies post-query processing to transform or enrich the query results.
// This method is called after data is fetched from the database but before returning to the client.
// If no Processor is configured via WithProcessor(), it returns the input unchanged.
func (a *baseFindOperation[TModel, TSearch, TProcessorIn, TOperation]) Process(input TProcessorIn, search TSearch, ctx fiber.Ctx) any {
	if a.processor == nil {
		return input
	}

	return a.processor(input, search, ctx)
}

// This function is called after data is fetched from the database but before returning to the client.
// Common use cases: data masking, computed fields, nested structure transformation, aggregation.
func (a *baseFindOperation[TModel, TSearch, TProcessorIn, TOperation]) WithProcessor(processor Processor[TProcessorIn, TSearch]) TOperation {
	a.processor = processor

	return a.self
}

// WithOptions adds multiple FindOperationOption to the query configuration.
// This is useful for composing reusable option sets.
func (a *baseFindOperation[TModel, TSearch, TProcessorIn, TOperation]) WithOptions(opts ...*FindOperationOption) TOperation {
	a.options = append(a.options, opts...)

	return a.self
}

// WithSelect adds a column to the SELECT clause.
// Applies to the root/main query by default (QueryRoot) unless specific parts are provided.
func (a *baseFindOperation[TModel, TSearch, TProcessorIn, TOperation]) WithSelect(column string, parts ...QueryPart) TOperation {
	a.options = append(a.options, withSelect(column, parts...))

	return a.self
}

// WithSelectAs adds a column with an alias to the SELECT clause.
// Applies to the root/main query by default (QueryRoot) unless specific parts are provided.
func (a *baseFindOperation[TModel, TSearch, TProcessorIn, TOperation]) WithSelectAs(column, alias string, parts ...QueryPart) TOperation {
	a.options = append(a.options, withSelectAs(column, alias, parts...))

	return a.self
}

// This is applied when no dynamic sorting is provided in the request.
// The orderSpecs are stored and applied during Setup() to allow framework-level defaults.
func (a *baseFindOperation[TModel, TSearch, TProcessorIn, TOperation]) WithDefaultSort(orderSpecs ...*sortx.OrderSpec) TOperation {
	if len(orderSpecs) > 0 {
		a.defaultSort = slices.Clone(orderSpecs)
	} else {
		a.defaultSort = []*sortx.OrderSpec{}
	}

	return a.self
}

// DisableDataPerm disables data permission filtering for this operation.
// By default, data permission filtering is enabled (WithDataPerm is auto-applied in Setup).
func (a *baseFindOperation[TModel, TSearch, TProcessorIn, TOperation]) DisableDataPerm() TOperation {
	a.dataPermDisabled = true

	return a.self
}

// WithAuditUserNames configures audit user names to be fetched (created_by_name, updated_by_name).
// If nameColumn is provided, uses the first value; otherwise defaults to "name".
func (a *baseFindOperation[TModel, TSearch, TProcessorIn, TOperation]) WithAuditUserNames(userModel any, nameColumn ...string) TOperation {
	a.auditUserModel = userModel
	if len(nameColumn) > 0 {
		a.auditUserNameColumn = nameColumn[0]
	} else {
		a.auditUserNameColumn = defaultAuditUserNameColumn
	}

	return a.self
}

// WithCondition adds a WHERE condition using ConditionBuilder.
// Applies to root query only by default (QueryRoot) unless specific parts are provided.
func (a *baseFindOperation[TModel, TSearch, TProcessorIn, TOperation]) WithCondition(fn func(cb orm.ConditionBuilder), parts ...QueryPart) TOperation {
	a.options = append(a.options, withCondition(fn, parts...))

	return a.self
}

// WithRelation adds a relation join to the query.
// Applies to the root/main query by default (QueryRoot) unless specific parts are provided.
func (a *baseFindOperation[TModel, TSearch, TProcessorIn, TOperation]) WithRelation(relation *orm.RelationSpec, parts ...QueryPart) TOperation {
	a.options = append(a.options, withRelation(relation, parts...))

	return a.self
}

// WithQueryApplier adds a custom query applier function.
// Applies to root query only by default (QueryRoot) unless specific parts are provided.
func (a *baseFindOperation[TModel, TSearch, TProcessorIn, TOperation]) WithQueryApplier(applier func(query orm.SelectQuery, search TSearch, ctx fiber.Ctx) error, parts ...QueryPart) TOperation {
	a.options = append(a.options, withQueryApplier(applier, parts...))

	return a.self
}
