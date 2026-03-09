package orm

import (
	"github.com/samber/lo"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"

	"github.com/coldsmirk/vef-framework-go/sortx"
)

// BaseAggregate defines the basic aggregate function interface with generic type support.
type BaseAggregate[T any] interface {
	// Column specifies the column to aggregate.
	Column(column string) T
	// Expr specifies a raw expression to aggregate.
	Expr(expr any) T
	// Filter applies a FILTER clause to the aggregate expression.
	Filter(func(ConditionBuilder)) T
}

// DistinctableAggregate defines aggregate functions that support DISTINCT operations.
type DistinctableAggregate[T any] interface {
	// Distinct marks the aggregate to operate on DISTINCT values.
	Distinct() T
}

// OrderableAggregate defines aggregate functions that support ordering.
type OrderableAggregate[T any] interface {
	// OrderBy adds ORDER BY clauses with ascending direction inside the aggregate.
	OrderBy(columns ...string) T
	// OrderByDesc adds ORDER BY clauses with descending direction inside the aggregate.
	OrderByDesc(columns ...string) T
	// OrderByExpr adds an ORDER BY clause based on a raw expression inside the aggregate.
	OrderByExpr(expr any) T
}

// NullHandlingBuilder defines aggregate functions that support NULL value handling.
type NullHandlingBuilder[T any] interface {
	// IgnoreNulls configures the aggregate to ignore NULL values.
	IgnoreNulls() T
	// RespectNulls configures the aggregate to respect NULL values.
	RespectNulls() T
}

// StatisticalAggregate defines aggregate functions that support statistical modes.
type StatisticalAggregate[T any] interface {
	// Population configures the aggregate to use population statistics (e.g., STDDEV_POP).
	Population() T
	// Sample configures the aggregate to use sample statistics (e.g., STDDEV_SAMP).
	Sample() T
}

// CountBuilder defines the COUNT aggregate function builder.
type CountBuilder interface {
	BaseAggregate[CountBuilder]
	DistinctableAggregate[CountBuilder]
	// All configures COUNT(*) semantics.
	All() CountBuilder
}

// SumBuilder defines the SUM aggregate function builder.
type SumBuilder interface {
	BaseAggregate[SumBuilder]
	DistinctableAggregate[SumBuilder]
}

// AvgBuilder defines the AVG aggregate function builder.
type AvgBuilder interface {
	BaseAggregate[AvgBuilder]
	DistinctableAggregate[AvgBuilder]
}

// MinBuilder defines the MIN aggregate function builder.
type MinBuilder interface {
	BaseAggregate[MinBuilder]
}

// MaxBuilder defines the MAX aggregate function builder.
type MaxBuilder interface {
	BaseAggregate[MaxBuilder]
}

// StringAggBuilder defines the STRING_AGG aggregate function builder.
type StringAggBuilder interface {
	BaseAggregate[StringAggBuilder]
	DistinctableAggregate[StringAggBuilder]
	OrderableAggregate[StringAggBuilder]
	NullHandlingBuilder[StringAggBuilder]

	// Separator sets the delimiter string between concatenated values.
	Separator(separator string) StringAggBuilder
}

// ArrayAggBuilder defines the ARRAY_AGG aggregate function builder.
type ArrayAggBuilder interface {
	BaseAggregate[ArrayAggBuilder]
	DistinctableAggregate[ArrayAggBuilder]
	OrderableAggregate[ArrayAggBuilder]
	NullHandlingBuilder[ArrayAggBuilder]
}

// StdDevBuilder defines the STDDEV aggregate function builder.
type StdDevBuilder interface {
	BaseAggregate[StdDevBuilder]
	StatisticalAggregate[StdDevBuilder]
}

// VarianceBuilder defines the VARIANCE aggregate function builder.
type VarianceBuilder interface {
	BaseAggregate[VarianceBuilder]
	StatisticalAggregate[VarianceBuilder]
}

// JSONObjectAggBuilder defines the JSON_OBJECT_AGG aggregate function builder.
type JSONObjectAggBuilder interface {
	BaseAggregate[JSONObjectAggBuilder]
	DistinctableAggregate[JSONObjectAggBuilder]
	OrderableAggregate[JSONObjectAggBuilder]

	// KeyColumn specifies the column to use as JSON object keys.
	KeyColumn(column string) JSONObjectAggBuilder
	// KeyExpr specifies a raw expression to use as JSON object keys.
	KeyExpr(expr any) JSONObjectAggBuilder
}

// JSONArrayAggBuilder defines the JSON_ARRAY_AGG aggregate function builder.
type JSONArrayAggBuilder interface {
	BaseAggregate[JSONArrayAggBuilder]
	DistinctableAggregate[JSONArrayAggBuilder]
	OrderableAggregate[JSONArrayAggBuilder]
}

// BitOrBuilder defines the BIT_OR aggregate function builder.
type BitOrBuilder interface {
	BaseAggregate[BitOrBuilder]
}

// BitAndBuilder defines the BIT_AND aggregate function builder.
type BitAndBuilder interface {
	BaseAggregate[BitAndBuilder]
}

// BoolOrBuilder defines the BOOL_OR aggregate function builder.
type BoolOrBuilder interface {
	BaseAggregate[BoolOrBuilder]
}

// BoolAndBuilder defines the BOOL_AND aggregate function builder.
type BoolAndBuilder interface {
	BaseAggregate[BoolAndBuilder]
}

type dialectAggConfig struct {
	funcName        string
	argsTransformer func(eb ExprBuilder, state *aggregateQueryState) schema.QueryAppender
	clearDistinct   bool
	clearOrderBy    bool
	clearNullsMode  bool
}

type dialectStrategy struct {
	postgres *dialectAggConfig
	mysql    *dialectAggConfig
	sqlite   *dialectAggConfig
	oracle   *dialectAggConfig
	sqlsrv   *dialectAggConfig
}

// bitNonZeroCaseTransformer wraps the expression in CASE WHEN expr != 0 THEN 1 ELSE 0 END.
// Used by SQLite to emulate BIT_OR/BIT_AND via MAX/MIN.
func bitNonZeroCaseTransformer(eb ExprBuilder, state *aggregateQueryState) schema.QueryAppender {
	return eb.Case(func(cb CaseBuilder) {
		cb.WhenExpr(eb.Expr("? != 0", state.argsExpr)).Then(1).Else(0)
	})
}

// boolCaseTransformer wraps the expression in CASE WHEN expr THEN 1 ELSE 0 END.
// Used by MySQL/SQLite to emulate BOOL_OR/BOOL_AND via MAX/MIN.
func boolCaseTransformer(eb ExprBuilder, state *aggregateQueryState) schema.QueryAppender {
	return eb.Case(func(cb CaseBuilder) {
		cb.WhenExpr(state.argsExpr).Then(1).Else(0)
	})
}

var bitOrStrategy = &dialectStrategy{
	postgres: &dialectAggConfig{funcName: "BIT_OR"},
	mysql:    &dialectAggConfig{funcName: "BIT_OR"},
	sqlite:   &dialectAggConfig{funcName: "MAX", argsTransformer: bitNonZeroCaseTransformer},
}

var bitAndStrategy = &dialectStrategy{
	postgres: &dialectAggConfig{funcName: "BIT_AND"},
	mysql:    &dialectAggConfig{funcName: "BIT_AND"},
	sqlite:   &dialectAggConfig{funcName: "MIN", argsTransformer: bitNonZeroCaseTransformer},
}

var boolOrStrategy = &dialectStrategy{
	postgres: &dialectAggConfig{funcName: "BOOL_OR"},
	mysql:    &dialectAggConfig{funcName: "MAX", argsTransformer: boolCaseTransformer},
	sqlite:   &dialectAggConfig{funcName: "MAX", argsTransformer: boolCaseTransformer},
}

var boolAndStrategy = &dialectStrategy{
	postgres: &dialectAggConfig{funcName: "BOOL_AND"},
	mysql:    &dialectAggConfig{funcName: "MIN", argsTransformer: boolCaseTransformer},
	sqlite:   &dialectAggConfig{funcName: "MIN", argsTransformer: boolCaseTransformer},
}

var jsonArrayAggStrategy = &dialectStrategy{
	postgres: &dialectAggConfig{funcName: "JSON_AGG"},
	mysql:    &dialectAggConfig{funcName: "JSON_ARRAYAGG"},
	sqlite:   &dialectAggConfig{funcName: "JSON_GROUP_ARRAY"},
}

var jsonObjectAggStrategy = &dialectStrategy{
	postgres: &dialectAggConfig{funcName: "JSON_OBJECT_AGG"},
	mysql:    &dialectAggConfig{funcName: "JSON_OBJECTAGG"},
	sqlite:   &dialectAggConfig{funcName: "JSON_GROUP_OBJECT"},
}

// nullsIgnoreTransformer wraps the expression in CASE WHEN expr IS NOT NULL THEN expr END
// when NullsIgnore mode is active. Used to emulate IGNORE NULLS on databases that lack native support.
func nullsIgnoreTransformer(eb ExprBuilder, state *aggregateQueryState) schema.QueryAppender {
	if state.nullsMode == NullsIgnore {
		return eb.Case(func(cb CaseBuilder) {
			cb.WhenExpr(eb.IsNotNull(state.argsExpr)).Then(state.argsExpr)
		})
	}

	return state.argsExpr
}

var arrayAggStrategy = &dialectStrategy{
	postgres: &dialectAggConfig{funcName: "ARRAY_AGG"},
	mysql: &dialectAggConfig{
		funcName:        "JSON_ARRAYAGG",
		argsTransformer: nullsIgnoreTransformer,
		clearDistinct:   true,
		clearOrderBy:    true,
		clearNullsMode:  true,
	},
	sqlite: &dialectAggConfig{
		funcName:        "JSON_GROUP_ARRAY",
		argsTransformer: nullsIgnoreTransformer,
		clearDistinct:   true,
		clearOrderBy:    true,
		clearNullsMode:  true,
	},
}

var stringAggStrategy = &dialectStrategy{
	postgres: &dialectAggConfig{
		funcName: "STRING_AGG",
		argsTransformer: func(eb ExprBuilder, state *aggregateQueryState) schema.QueryAppender {
			return eb.Expr("?, ?", nullsIgnoreTransformer(eb, state), state.separator)
		},
	},
	mysql: &dialectAggConfig{
		funcName: "GROUP_CONCAT",
		argsTransformer: func(eb ExprBuilder, state *aggregateQueryState) schema.QueryAppender {
			argsExpr := nullsIgnoreTransformer(eb, state)
			if len(state.orderExprs) > 0 {
				return eb.Expr("? ? SEPARATOR ?", argsExpr, newOrderByClause(state.orderExprs...), state.separator)
			}

			return eb.Expr("? SEPARATOR ?", argsExpr, state.separator)
		},
		clearOrderBy:   true,
		clearNullsMode: true,
	},
	sqlite: &dialectAggConfig{
		funcName: "GROUP_CONCAT",
		argsTransformer: func(eb ExprBuilder, state *aggregateQueryState) schema.QueryAppender {
			argsExpr := nullsIgnoreTransformer(eb, state)
			// SQLite DISTINCT limitation: only one argument allowed
			if state.distinct {
				return argsExpr
			}

			return eb.Expr("?, ?", argsExpr, state.separator)
		},
		clearNullsMode: true,
	},
}

// statisticalStrategy builds a dialectStrategy for statistical aggregates (STDDEV, VARIANCE).
// PgPrefix is the PostgreSQL function prefix (e.g., "STDDEV", "VAR") used with "_POP"/"_SAMP" suffixes.
// MysqlPrefix follows the same pattern. MysqlDefault is the fallback name when no mode is specified.
func statisticalStrategy(pgPrefix, mysqlPrefix, mysqlDefault string) *dialectStrategy {
	return &dialectStrategy{
		postgres: &dialectAggConfig{
			argsTransformer: func(_ ExprBuilder, state *aggregateQueryState) schema.QueryAppender {
				mode := lo.CoalesceOrEmpty(state.statisticalMode.String(), StatisticalPopulation.String())
				state.funcName = pgPrefix + "_" + mode

				return state.argsExpr
			},
		},
		mysql: &dialectAggConfig{
			argsTransformer: func(_ ExprBuilder, state *aggregateQueryState) schema.QueryAppender {
				if state.statisticalMode == StatisticalPopulation || state.statisticalMode == StatisticalSample {
					state.funcName = mysqlPrefix + "_" + state.statisticalMode.String()
				} else {
					state.funcName = mysqlDefault
				}

				return state.argsExpr
			},
		},
	}
}

var stdDevStrategy = statisticalStrategy("STDDEV", "STDDEV", "STDDEV")

var varianceStrategy = statisticalStrategy("VAR", "VAR", "VARIANCE")

type aggregateQueryState struct {
	funcName        string
	argsExpr        schema.QueryAppender
	distinct        bool
	filter          schema.QueryAppender
	orderExprs      []orderExpr
	nullsMode       NullsMode
	separator       string
	statisticalMode StatisticalMode
}

type baseAggregateExpr struct {
	qb         QueryBuilder
	eb         ExprBuilder
	funcName   string
	argsExpr   schema.QueryAppender
	distinct   bool
	filter     schema.QueryAppender
	orderExprs []orderExpr
	nullsMode  NullsMode
	strategy   *dialectStrategy
}

func (a *baseAggregateExpr) getDialectConfig() *dialectAggConfig {
	if a.strategy == nil {
		return nil
	}

	var cfg *dialectAggConfig
	a.eb.ExecByDialect(DialectExecs{
		Postgres:  func() { cfg = a.strategy.postgres },
		MySQL:     func() { cfg = a.strategy.mysql },
		SQLite:    func() { cfg = a.strategy.sqlite },
		Oracle:    func() { cfg = a.strategy.oracle },
		SQLServer: func() { cfg = a.strategy.sqlsrv },
	})

	return cfg
}

func (a *baseAggregateExpr) buildQueryState() aggregateQueryState {
	return aggregateQueryState{
		funcName:   a.funcName,
		argsExpr:   a.argsExpr,
		distinct:   a.distinct,
		filter:     a.filter,
		orderExprs: a.orderExprs,
		nullsMode:  a.nullsMode,
	}
}

func (a *baseAggregateExpr) applyDialectConfig(state *aggregateQueryState, cfg *dialectAggConfig) {
	if cfg == nil {
		return
	}

	if cfg.funcName != "" {
		state.funcName = cfg.funcName
	}

	if cfg.argsTransformer != nil {
		state.argsExpr = cfg.argsTransformer(a.eb, state)
	}

	if cfg.clearDistinct {
		state.distinct = false
	}

	if cfg.clearOrderBy {
		state.orderExprs = nil
	}

	if cfg.clearNullsMode {
		state.nullsMode = NullsDefault
	}
}

func (a *baseAggregateExpr) dialectAwareAppendQuery(gen schema.QueryGen, b []byte) ([]byte, error) {
	state := a.buildQueryState()

	cfg := a.getDialectConfig()
	if cfg == nil {
		return nil, ErrDialectUnsupportedOperation
	}

	a.applyDialectConfig(&state, cfg)

	return a.appendQueryWithState(gen, b, state)
}

func (a *baseAggregateExpr) setFilter(builder func(ConditionBuilder)) {
	a.filter = a.qb.BuildCondition(builder)
}

func (a *baseAggregateExpr) appendOrderBy(columns ...string) {
	for _, column := range columns {
		a.orderExprs = append(a.orderExprs, orderExpr{
			builders:   a.eb,
			column:     column,
			direction:  sortx.OrderAsc,
			nullsOrder: sortx.NullsDefault,
		})
	}
}

func (a *baseAggregateExpr) appendOrderByDesc(columns ...string) {
	for _, column := range columns {
		a.orderExprs = append(a.orderExprs, orderExpr{
			builders:   a.eb,
			column:     column,
			direction:  sortx.OrderDesc,
			nullsOrder: sortx.NullsDefault,
		})
	}
}

func (a *baseAggregateExpr) appendOrderByExpr(expr any) {
	a.orderExprs = append(a.orderExprs, orderExpr{
		builders:   a.eb,
		expr:       expr,
		direction:  sortx.OrderAsc,
		nullsOrder: sortx.NullsDefault,
	})
}

func (a *baseAggregateExpr) AppendQuery(gen schema.QueryGen, b []byte) ([]byte, error) {
	return a.appendQueryWithState(gen, b, a.buildQueryState())
}

func (a *baseAggregateExpr) appendQueryWithState(gen schema.QueryGen, b []byte, state aggregateQueryState) (_ []byte, err error) {
	if state.argsExpr == nil {
		return nil, ErrAggregateMissingArgs
	}

	if state.filter != nil {
		var handled bool

		a.eb.ExecByDialect(DialectExecs{
			MySQL: func() {
				b, err = a.appendCompatibleFilterQueryWithState(gen, b, state)
				handled = true
			},
			Oracle: func() {
				b, err = a.appendCompatibleFilterQueryWithState(gen, b, state)
				handled = true
			},
			SQLServer: func() {
				b, err = a.appendCompatibleFilterQueryWithState(gen, b, state)
				handled = true
			},
		})

		if handled {
			return b, err
		}
	}

	b = append(b, state.funcName...)
	b = append(b, '(')

	if state.distinct {
		b = append(b, "DISTINCT "...)
	}

	if b, err = state.argsExpr.AppendQuery(gen, b); err != nil {
		return
	}

	if len(state.orderExprs) > 0 {
		b = append(b, ' ')
		if b, err = newOrderByClause(state.orderExprs...).AppendQuery(gen, b); err != nil {
			return
		}
	}

	b = append(b, ')')

	if state.nullsMode != NullsDefault {
		b = append(b, ' ')
		b = append(b, state.nullsMode.String()...)
	}

	if state.filter != nil {
		if b, err = newFilterClause(state.filter).AppendQuery(gen, b); err != nil {
			return
		}
	}

	return b, nil
}

func (a *baseAggregateExpr) appendCompatibleFilterQueryWithState(gen schema.QueryGen, b []byte, state aggregateQueryState) (_ []byte, err error) {
	funcName := state.funcName

	switch funcName {
	case "COUNT":
		b = append(b, "SUM"...)
	default:
		b = append(b, funcName...)
	}

	b = append(b, '(')

	if state.distinct {
		b = append(b, "DISTINCT "...)
	}

	if b, err = a.eb.Case(func(cb CaseBuilder) {
		when := cb.WhenExpr(state.filter)
		switch funcName {
		case "COUNT":
			when.Then(1)
		default:
			when.Then(state.argsExpr)
		}

		switch funcName {
		case "COUNT", "SUM":
			cb.Else(0)
		default:
			cb.Else(a.eb.Null())
		}
	}).AppendQuery(gen, b); err != nil {
		return
	}

	b = append(b, ')')

	return b, nil
}

type baseAggregateBuilder[T any] struct {
	*baseAggregateExpr

	self T
}

func (b *baseAggregateBuilder[T]) Column(column string) T {
	b.argsExpr = b.eb.Column(column)

	return b.self
}

func (b *baseAggregateBuilder[T]) Expr(expr any) T {
	b.argsExpr = b.eb.Expr("?", expr)

	return b.self
}

func (b *baseAggregateBuilder[T]) Filter(builder func(ConditionBuilder)) T {
	b.setFilter(builder)

	return b.self
}

type distinctableAggregateBuilder[T any] struct {
	*baseAggregateBuilder[T]
}

func (b *distinctableAggregateBuilder[T]) Distinct() T {
	b.distinct = true

	return b.self
}

type orderableAggregateBuilder[T any] struct {
	*baseAggregateBuilder[T]
}

func (b *orderableAggregateBuilder[T]) OrderBy(columns ...string) T {
	b.appendOrderBy(columns...)

	return b.self
}

func (b *orderableAggregateBuilder[T]) OrderByDesc(columns ...string) T {
	b.appendOrderByDesc(columns...)

	return b.self
}

func (b *orderableAggregateBuilder[T]) OrderByExpr(expr any) T {
	b.appendOrderByExpr(expr)

	return b.self
}

type baseNullHandlingBuilder[T any] struct {
	*baseAggregateBuilder[T]
}

func (b *baseNullHandlingBuilder[T]) IgnoreNulls() T {
	b.nullsMode = NullsIgnore

	return b.self
}

func (b *baseNullHandlingBuilder[T]) RespectNulls() T {
	b.nullsMode = NullsRespect

	return b.self
}

type statisticalAggregateBuilder[T any] struct {
	*baseAggregateBuilder[T]

	statExpr *statisticalAggExpr
}

func (b *statisticalAggregateBuilder[T]) Population() T {
	b.statExpr.statisticalMode = StatisticalPopulation

	return b.self
}

func (b *statisticalAggregateBuilder[T]) Sample() T {
	b.statExpr.statisticalMode = StatisticalSample

	return b.self
}

type countExpr[T any] struct {
	*baseAggregateExpr
	*distinctableAggregateBuilder[T]
}

func (c *countExpr[T]) All() T {
	c.argsExpr = bun.Safe(columnAll)

	return c.self
}

// distinctableAggExpr is a shared implementation for aggregates that support
// DISTINCT operations (e.g., SUM, AVG).
type distinctableAggExpr[T any] struct {
	*baseAggregateExpr
	*distinctableAggregateBuilder[T]
}

// simpleAggExpr is a shared implementation for aggregates that only need
// BaseAggregate capabilities (e.g., MIN, MAX).
type simpleAggExpr[T any] struct {
	*baseAggregateExpr
	*baseAggregateBuilder[T]
}

type stringAggExpr[T any] struct {
	*baseAggregateExpr
	*baseAggregateBuilder[T]
	*distinctableAggregateBuilder[T]
	*orderableAggregateBuilder[T]
	*baseNullHandlingBuilder[T]

	separator string
}

func (s *stringAggExpr[T]) Separator(separator string) T {
	s.separator = separator

	return s.self
}

func (s *stringAggExpr[T]) AppendQuery(gen schema.QueryGen, b []byte) ([]byte, error) {
	state := s.buildQueryState()
	state.separator = s.separator

	cfg := s.getDialectConfig()
	if cfg == nil {
		return nil, ErrDialectUnsupportedOperation
	}

	s.applyDialectConfig(&state, cfg)

	return s.appendQueryWithState(gen, b, state)
}

type arrayAggExpr[T any] struct {
	*baseAggregateExpr
	*baseAggregateBuilder[T]
	*distinctableAggregateBuilder[T]
	*orderableAggregateBuilder[T]
	*baseNullHandlingBuilder[T]
}

func (a *arrayAggExpr[T]) AppendQuery(gen schema.QueryGen, b []byte) ([]byte, error) {
	return a.dialectAwareAppendQuery(gen, b)
}

type statisticalAggExpr struct {
	*baseAggregateExpr

	statisticalMode StatisticalMode
}

func (s *statisticalAggExpr) AppendQuery(gen schema.QueryGen, b []byte) ([]byte, error) {
	state := s.buildQueryState()
	state.statisticalMode = s.statisticalMode

	cfg := s.getDialectConfig()
	if cfg == nil {
		return nil, ErrDialectUnsupportedOperation
	}

	s.applyDialectConfig(&state, cfg)

	return s.appendQueryWithState(gen, b, state)
}

type stdDevExpr[T any] struct {
	*statisticalAggExpr
	*statisticalAggregateBuilder[T]
}

type varianceExpr[T any] struct {
	*statisticalAggExpr
	*statisticalAggregateBuilder[T]
}

type jsonObjectAggExpr[T any] struct {
	*baseAggregateExpr
	*baseAggregateBuilder[T]
	*distinctableAggregateBuilder[T]
	*orderableAggregateBuilder[T]

	keyExpr schema.QueryAppender
}

func (j *jsonObjectAggExpr[T]) KeyColumn(column string) T {
	j.keyExpr = j.eb.Column(column)

	return j.self
}

func (j *jsonObjectAggExpr[T]) KeyExpr(expr any) T {
	j.keyExpr = j.eb.Expr("?", expr)

	return j.self
}

func (j *jsonObjectAggExpr[T]) AppendQuery(gen schema.QueryGen, b []byte) ([]byte, error) {
	if j.keyExpr == nil {
		return nil, ErrAggregateMissingArgs
	}

	state := j.buildQueryState()

	cfg := j.getDialectConfig()
	if cfg == nil {
		return nil, ErrDialectUnsupportedOperation
	}

	j.applyDialectConfig(&state, cfg)
	state.argsExpr = j.eb.Exprs(j.keyExpr, j.argsExpr)

	return j.appendQueryWithState(gen, b, state)
}

type jsonArrayAggExpr[T any] struct {
	*baseAggregateExpr
	*baseAggregateBuilder[T]
	*distinctableAggregateBuilder[T]
	*orderableAggregateBuilder[T]
}

func (j *jsonArrayAggExpr[T]) AppendQuery(gen schema.QueryGen, b []byte) ([]byte, error) {
	return j.dialectAwareAppendQuery(gen, b)
}

// dialectAggExpr is a shared implementation for dialect-aware aggregates that only need
// BaseAggregate capabilities (e.g., BIT_OR, BIT_AND, BOOL_OR, BOOL_AND).
type dialectAggExpr[T any] struct {
	*baseAggregateExpr
	*baseAggregateBuilder[T]
}

func (b *dialectAggExpr[T]) AppendQuery(gen schema.QueryGen, buf []byte) ([]byte, error) {
	return b.dialectAwareAppendQuery(gen, buf)
}

func newGenericSimpleAggExpr[T any](self T, qb QueryBuilder, funcName string) *simpleAggExpr[T] {
	baseExpr := &baseAggregateExpr{
		qb:       qb,
		eb:       qb.ExprBuilder(),
		funcName: funcName,
	}
	baseBuilder := &baseAggregateBuilder[T]{
		baseAggregateExpr: baseExpr,
	}
	expr := &simpleAggExpr[T]{
		baseAggregateExpr:    baseExpr,
		baseAggregateBuilder: baseBuilder,
	}

	baseBuilder.self = self

	return expr
}

func newMinExpr(qb QueryBuilder) *simpleAggExpr[MinBuilder] {
	expr := newGenericSimpleAggExpr[MinBuilder](nil, qb, "MIN")
	expr.self = expr

	return expr
}

func newMaxExpr(qb QueryBuilder) *simpleAggExpr[MaxBuilder] {
	expr := newGenericSimpleAggExpr[MaxBuilder](nil, qb, "MAX")
	expr.self = expr

	return expr
}

func newGenericCountExpr[T any](self T, qb QueryBuilder) *countExpr[T] {
	baseExpr := &baseAggregateExpr{
		qb:       qb,
		eb:       qb.ExprBuilder(),
		funcName: "COUNT",
	}
	baseBuilder := &baseAggregateBuilder[T]{
		baseAggregateExpr: baseExpr,
	}
	expr := &countExpr[T]{
		baseAggregateExpr: baseExpr,
		distinctableAggregateBuilder: &distinctableAggregateBuilder[T]{
			baseAggregateBuilder: baseBuilder,
		},
	}

	baseBuilder.self = self

	return expr
}

func newCountExpr(qb QueryBuilder) *countExpr[CountBuilder] {
	expr := newGenericCountExpr[CountBuilder](nil, qb)
	expr.self = expr

	return expr
}

func newGenericDistinctableAggExpr[T any](self T, qb QueryBuilder, funcName string) *distinctableAggExpr[T] {
	baseExpr := &baseAggregateExpr{
		qb:       qb,
		eb:       qb.ExprBuilder(),
		funcName: funcName,
	}
	baseBuilder := &baseAggregateBuilder[T]{
		baseAggregateExpr: baseExpr,
	}
	expr := &distinctableAggExpr[T]{
		baseAggregateExpr: baseExpr,
		distinctableAggregateBuilder: &distinctableAggregateBuilder[T]{
			baseAggregateBuilder: baseBuilder,
		},
	}

	baseBuilder.self = self

	return expr
}

func newSumExpr(qb QueryBuilder) *distinctableAggExpr[SumBuilder] {
	expr := newGenericDistinctableAggExpr[SumBuilder](nil, qb, "SUM")
	expr.self = expr

	return expr
}

func newAvgExpr(qb QueryBuilder) *distinctableAggExpr[AvgBuilder] {
	expr := newGenericDistinctableAggExpr[AvgBuilder](nil, qb, "AVG")
	expr.self = expr

	return expr
}

func newGenericMinExpr[T any](self T, qb QueryBuilder) *simpleAggExpr[T] {
	return newGenericSimpleAggExpr(self, qb, "MIN")
}

func newGenericMaxExpr[T any](self T, qb QueryBuilder) *simpleAggExpr[T] {
	return newGenericSimpleAggExpr(self, qb, "MAX")
}

func newGenericStringAggExpr[T any](self T, qb QueryBuilder) *stringAggExpr[T] {
	baseExpr := &baseAggregateExpr{
		qb:       qb,
		eb:       qb.ExprBuilder(),
		strategy: stringAggStrategy,
	}
	baseBuilder := &baseAggregateBuilder[T]{
		baseAggregateExpr: baseExpr,
	}
	expr := &stringAggExpr[T]{
		baseAggregateExpr:    baseExpr,
		baseAggregateBuilder: baseBuilder,
		distinctableAggregateBuilder: &distinctableAggregateBuilder[T]{
			baseAggregateBuilder: baseBuilder,
		},
		orderableAggregateBuilder: &orderableAggregateBuilder[T]{
			baseAggregateBuilder: baseBuilder,
		},
		baseNullHandlingBuilder: &baseNullHandlingBuilder[T]{
			baseAggregateBuilder: baseBuilder,
		},
		separator: ",",
	}

	baseBuilder.self = self

	return expr
}

func newStringAggExpr(qb QueryBuilder) *stringAggExpr[StringAggBuilder] {
	expr := newGenericStringAggExpr[StringAggBuilder](nil, qb)
	expr.self = expr

	return expr
}

func newGenericArrayAggExpr[T any](self T, qb QueryBuilder) *arrayAggExpr[T] {
	baseExpr := &baseAggregateExpr{
		qb:       qb,
		eb:       qb.ExprBuilder(),
		strategy: arrayAggStrategy,
	}
	baseBuilder := &baseAggregateBuilder[T]{
		baseAggregateExpr: baseExpr,
	}
	expr := &arrayAggExpr[T]{
		baseAggregateExpr:    baseExpr,
		baseAggregateBuilder: baseBuilder,
		distinctableAggregateBuilder: &distinctableAggregateBuilder[T]{
			baseAggregateBuilder: baseBuilder,
		},
		orderableAggregateBuilder: &orderableAggregateBuilder[T]{
			baseAggregateBuilder: baseBuilder,
		},
		baseNullHandlingBuilder: &baseNullHandlingBuilder[T]{
			baseAggregateBuilder: baseBuilder,
		},
	}

	baseBuilder.self = self

	return expr
}

func newArrayAggExpr(qb QueryBuilder) *arrayAggExpr[ArrayAggBuilder] {
	expr := newGenericArrayAggExpr[ArrayAggBuilder](nil, qb)
	expr.self = expr

	return expr
}

func newStatisticalAggBase[T any](self T, qb QueryBuilder, strategy *dialectStrategy) (*statisticalAggExpr, *statisticalAggregateBuilder[T]) {
	baseExpr := &baseAggregateExpr{
		qb:       qb,
		eb:       qb.ExprBuilder(),
		strategy: strategy,
	}
	baseBuilder := &baseAggregateBuilder[T]{
		baseAggregateExpr: baseExpr,
	}
	statExpr := &statisticalAggExpr{
		baseAggregateExpr: baseExpr,
	}
	statBuilder := &statisticalAggregateBuilder[T]{
		baseAggregateBuilder: baseBuilder,
		statExpr:             statExpr,
	}

	baseBuilder.self = self

	return statExpr, statBuilder
}

func newGenericStdDevExpr[T any](self T, qb QueryBuilder) *stdDevExpr[T] {
	statExpr, statBuilder := newStatisticalAggBase(self, qb, stdDevStrategy)

	return &stdDevExpr[T]{
		statisticalAggExpr:          statExpr,
		statisticalAggregateBuilder: statBuilder,
	}
}

func newStdDevExpr(qb QueryBuilder) *stdDevExpr[StdDevBuilder] {
	expr := newGenericStdDevExpr[StdDevBuilder](nil, qb)
	expr.self = expr

	return expr
}

func newGenericVarianceExpr[T any](self T, qb QueryBuilder) *varianceExpr[T] {
	statExpr, statBuilder := newStatisticalAggBase(self, qb, varianceStrategy)

	return &varianceExpr[T]{
		statisticalAggExpr:          statExpr,
		statisticalAggregateBuilder: statBuilder,
	}
}

func newVarianceExpr(qb QueryBuilder) *varianceExpr[VarianceBuilder] {
	expr := newGenericVarianceExpr[VarianceBuilder](nil, qb)
	expr.self = expr

	return expr
}

func newGenericJSONObjectAggExpr[T any](self T, qb QueryBuilder) *jsonObjectAggExpr[T] {
	baseExpr := &baseAggregateExpr{
		qb:       qb,
		eb:       qb.ExprBuilder(),
		strategy: jsonObjectAggStrategy,
	}
	baseBuilder := &baseAggregateBuilder[T]{
		baseAggregateExpr: baseExpr,
	}
	expr := &jsonObjectAggExpr[T]{
		baseAggregateExpr:    baseExpr,
		baseAggregateBuilder: baseBuilder,
		distinctableAggregateBuilder: &distinctableAggregateBuilder[T]{
			baseAggregateBuilder: baseBuilder,
		},
		orderableAggregateBuilder: &orderableAggregateBuilder[T]{
			baseAggregateBuilder: baseBuilder,
		},
	}

	baseBuilder.self = self

	return expr
}

func newJSONObjectAggExpr(qb QueryBuilder) *jsonObjectAggExpr[JSONObjectAggBuilder] {
	expr := newGenericJSONObjectAggExpr[JSONObjectAggBuilder](nil, qb)
	expr.self = expr

	return expr
}

func newGenericJSONArrayAggExpr[T any](self T, qb QueryBuilder) *jsonArrayAggExpr[T] {
	baseExpr := &baseAggregateExpr{
		qb:       qb,
		eb:       qb.ExprBuilder(),
		strategy: jsonArrayAggStrategy,
	}
	baseBuilder := &baseAggregateBuilder[T]{
		baseAggregateExpr: baseExpr,
	}
	expr := &jsonArrayAggExpr[T]{
		baseAggregateExpr:    baseExpr,
		baseAggregateBuilder: baseBuilder,
		distinctableAggregateBuilder: &distinctableAggregateBuilder[T]{
			baseAggregateBuilder: baseBuilder,
		},
		orderableAggregateBuilder: &orderableAggregateBuilder[T]{
			baseAggregateBuilder: baseBuilder,
		},
	}

	baseBuilder.self = self

	return expr
}

func newJSONArrayAggExpr(qb QueryBuilder) *jsonArrayAggExpr[JSONArrayAggBuilder] {
	expr := newGenericJSONArrayAggExpr[JSONArrayAggBuilder](nil, qb)
	expr.self = expr

	return expr
}

func newGenericDialectAggExpr[T any](self T, qb QueryBuilder, strategy *dialectStrategy) *dialectAggExpr[T] {
	baseExpr := &baseAggregateExpr{
		qb:       qb,
		eb:       qb.ExprBuilder(),
		strategy: strategy,
	}
	baseBuilder := &baseAggregateBuilder[T]{
		baseAggregateExpr: baseExpr,
	}
	expr := &dialectAggExpr[T]{
		baseAggregateExpr:    baseExpr,
		baseAggregateBuilder: baseBuilder,
	}

	baseBuilder.self = self

	return expr
}

func newBitOrExpr(qb QueryBuilder) *dialectAggExpr[BitOrBuilder] {
	expr := newGenericDialectAggExpr[BitOrBuilder](nil, qb, bitOrStrategy)
	expr.self = expr

	return expr
}

func newBitAndExpr(qb QueryBuilder) *dialectAggExpr[BitAndBuilder] {
	expr := newGenericDialectAggExpr[BitAndBuilder](nil, qb, bitAndStrategy)
	expr.self = expr

	return expr
}

func newBoolOrExpr(qb QueryBuilder) *dialectAggExpr[BoolOrBuilder] {
	expr := newGenericDialectAggExpr[BoolOrBuilder](nil, qb, boolOrStrategy)
	expr.self = expr

	return expr
}

func newBoolAndExpr(qb QueryBuilder) *dialectAggExpr[BoolAndBuilder] {
	expr := newGenericDialectAggExpr[BoolAndBuilder](nil, qb, boolAndStrategy)
	expr.self = expr

	return expr
}
