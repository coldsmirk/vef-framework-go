package orm

import (
	"strconv"

	"github.com/uptrace/bun/schema"

	"github.com/coldsmirk/vef-framework-go/sortx"
)

// WindowPartitionable defines window functions that support partitioning.
type WindowPartitionable[T any] interface {
	// Over starts configuring the OVER clause for the window function.
	Over() T
}

// BaseWindowPartitionBuilder defines the base window partition builder interface.
type BaseWindowPartitionBuilder[T any] interface {
	// PartitionBy adds PARTITION BY columns to the window definition.
	PartitionBy(columns ...string) T
	// PartitionByExpr adds a raw PARTITION BY expression to the window definition.
	PartitionByExpr(expr any) T
	// OrderBy adds ORDER BY clauses with ascending direction.
	OrderBy(columns ...string) T
	// OrderByDesc adds ORDER BY clauses with descending direction.
	OrderByDesc(columns ...string) T
	// OrderByExpr adds an ORDER BY clause using a raw expression.
	OrderByExpr(expr any) T
}

// WindowPartitionBuilder defines the window partition builder interface.
type WindowPartitionBuilder interface {
	BaseWindowPartitionBuilder[WindowPartitionBuilder]
}

// WindowFrameablePartitionBuilder defines window functions that support partitioning and frame specification.
type WindowFrameablePartitionBuilder interface {
	BaseWindowPartitionBuilder[WindowFrameablePartitionBuilder]
	// Rows configures a ROWS frame clause.
	Rows() WindowFrameBuilder
	// Range configures a RANGE frame clause.
	Range() WindowFrameBuilder
	// Groups configures a GROUPS frame clause.
	Groups() WindowFrameBuilder
}

// WindowBoundable defines window frame boundaries.
type WindowBoundable[T any] interface {
	// CurrentRow sets the boundary to the current row.
	CurrentRow() T
	// Preceding sets the boundary to N rows before the current row.
	Preceding(n int) T
	// Following sets the boundary to N rows after the current row.
	Following(n int) T
}

// WindowStartBoundable defines window frame start boundaries.
type WindowStartBoundable[T any] interface {
	WindowBoundable[T]

	// UnboundedPreceding sets the start boundary to the first row of the partition.
	UnboundedPreceding() T
}

// WindowEndBoundable defines window frame end boundaries.
type WindowEndBoundable[T any] interface {
	WindowStartBoundable[T]

	// UnboundedFollowing sets the end boundary to the last row of the partition.
	UnboundedFollowing() T
}

// WindowFrameBuilder defines the window frame builder interface.
type WindowFrameBuilder interface {
	WindowStartBoundable[WindowFrameBuilder]

	// And switches to configuring the end boundary for BETWEEN ... AND ... syntax.
	And() WindowFrameEndBuilder
}

// WindowFrameEndBuilder defines the window frame end boundary builder interface.
type WindowFrameEndBuilder interface {
	WindowEndBoundable[WindowFrameEndBuilder]
}

// RowNumberBuilder defines the ROW_NUMBER() window function builder.
type RowNumberBuilder interface {
	WindowPartitionable[WindowFrameablePartitionBuilder]
}

// RankBuilder defines the RANK() window function builder.
type RankBuilder interface {
	WindowPartitionable[WindowFrameablePartitionBuilder]
}

// DenseRankBuilder defines the DENSE_RANK() window function builder.
type DenseRankBuilder interface {
	WindowPartitionable[WindowFrameablePartitionBuilder]
}

// PercentRankBuilder defines the PERCENT_RANK() window function builder.
type PercentRankBuilder interface {
	WindowPartitionable[WindowFrameablePartitionBuilder]
}

// CumeDistBuilder defines the CUME_DIST() window function builder.
type CumeDistBuilder interface {
	WindowPartitionable[WindowFrameablePartitionBuilder]
}

// NTileBuilder defines the NTILE(n) window function builder.
type NTileBuilder interface {
	WindowPartitionable[WindowFrameablePartitionBuilder]

	// Buckets sets the number of roughly equal-sized groups to distribute rows into.
	Buckets(n int) NTileBuilder
}

// LagBuilder defines the LAG() window function builder.
type LagBuilder interface {
	WindowPartitionable[WindowPartitionBuilder]

	// Column specifies the column to access from the previous row.
	Column(column string) LagBuilder
	// Expr specifies a raw expression to access from the previous row.
	Expr(expr any) LagBuilder
	// Offset sets the number of rows to look back (default 1).
	Offset(offset int) LagBuilder
	// DefaultValue sets the fallback value when no previous row exists.
	DefaultValue(value any) LagBuilder
}

// LeadBuilder defines the LEAD() window function builder.
type LeadBuilder interface {
	WindowPartitionable[WindowPartitionBuilder]

	// Column specifies the column to access from the next row.
	Column(column string) LeadBuilder
	// Expr specifies a raw expression to access from the next row.
	Expr(expr any) LeadBuilder
	// Offset sets the number of rows to look ahead (default 1).
	Offset(offset int) LeadBuilder
	// DefaultValue sets the fallback value when no next row exists.
	DefaultValue(value any) LeadBuilder
}

// FirstValueBuilder defines the FIRST_VALUE() window function builder.
type FirstValueBuilder interface {
	WindowPartitionable[WindowFrameablePartitionBuilder]
	NullHandlingBuilder[FirstValueBuilder]

	// Column specifies the column to retrieve the first value from.
	Column(column string) FirstValueBuilder
	// Expr specifies a raw expression to retrieve the first value from.
	Expr(expr any) FirstValueBuilder
}

// LastValueBuilder defines the LAST_VALUE() window function builder.
type LastValueBuilder interface {
	WindowPartitionable[WindowFrameablePartitionBuilder]
	NullHandlingBuilder[LastValueBuilder]

	// Column specifies the column to retrieve the last value from.
	Column(column string) LastValueBuilder
	// Expr specifies a raw expression to retrieve the last value from.
	Expr(expr any) LastValueBuilder
}

// NthValueBuilder defines the NTH_VALUE() window function builder.
type NthValueBuilder interface {
	WindowPartitionable[WindowFrameablePartitionBuilder]
	NullHandlingBuilder[NthValueBuilder]

	// Column specifies the column to retrieve the Nth value from.
	Column(column string) NthValueBuilder
	// Expr specifies a raw expression to retrieve the Nth value from.
	Expr(expr any) NthValueBuilder
	// N sets the position (1-based) of the value to retrieve within the window frame.
	N(n int) NthValueBuilder
	// FromFirst counts the position from the first row of the window frame.
	FromFirst() NthValueBuilder
	// FromLast counts the position from the last row of the window frame.
	FromLast() NthValueBuilder
}

// WindowCountBuilder defines COUNT() as window function builder.
type WindowCountBuilder interface {
	BaseAggregate[WindowCountBuilder]
	DistinctableAggregate[WindowCountBuilder]
	WindowPartitionable[WindowFrameablePartitionBuilder]

	// All configures COUNT(*) semantics for the window function.
	All() WindowCountBuilder
}

// WindowSumBuilder defines SUM() as window function builder.
type WindowSumBuilder interface {
	BaseAggregate[WindowSumBuilder]
	DistinctableAggregate[WindowSumBuilder]
	WindowPartitionable[WindowFrameablePartitionBuilder]
}

// WindowAvgBuilder defines AVG() as window function builder.
type WindowAvgBuilder interface {
	BaseAggregate[WindowAvgBuilder]
	DistinctableAggregate[WindowAvgBuilder]
	WindowPartitionable[WindowFrameablePartitionBuilder]
}

// WindowMinBuilder defines MIN() as window function builder.
type WindowMinBuilder interface {
	BaseAggregate[WindowMinBuilder]
	WindowPartitionable[WindowFrameablePartitionBuilder]
}

// WindowMaxBuilder defines MAX() as window function builder.
type WindowMaxBuilder interface {
	BaseAggregate[WindowMaxBuilder]
	WindowPartitionable[WindowFrameablePartitionBuilder]
}

// WindowStringAggBuilder defines STRING_AGG() as window function builder.
type WindowStringAggBuilder interface {
	BaseAggregate[WindowStringAggBuilder]
	DistinctableAggregate[WindowStringAggBuilder]
	OrderableAggregate[WindowStringAggBuilder]
	NullHandlingBuilder[WindowStringAggBuilder]
	WindowPartitionable[WindowFrameablePartitionBuilder]

	// Separator sets the delimiter string between concatenated values.
	Separator(separator string) WindowStringAggBuilder
}

// WindowArrayAggBuilder defines ARRAY_AGG() as window function builder.
type WindowArrayAggBuilder interface {
	BaseAggregate[WindowArrayAggBuilder]
	DistinctableAggregate[WindowArrayAggBuilder]
	OrderableAggregate[WindowArrayAggBuilder]
	NullHandlingBuilder[WindowArrayAggBuilder]
	WindowPartitionable[WindowFrameablePartitionBuilder]
}

// WindowStdDevBuilder defines STDDEV() as window function builder.
type WindowStdDevBuilder interface {
	BaseAggregate[WindowStdDevBuilder]
	StatisticalAggregate[WindowStdDevBuilder]
	WindowPartitionable[WindowFrameablePartitionBuilder]
}

// WindowVarianceBuilder defines VARIANCE() as window function builder.
type WindowVarianceBuilder interface {
	BaseAggregate[WindowVarianceBuilder]
	StatisticalAggregate[WindowVarianceBuilder]
	WindowPartitionable[WindowFrameablePartitionBuilder]
}

// WindowJSONObjectAggBuilder defines JSON_OBJECT_AGG() as window function builder.
type WindowJSONObjectAggBuilder interface {
	BaseAggregate[WindowJSONObjectAggBuilder]
	DistinctableAggregate[WindowJSONObjectAggBuilder]
	OrderableAggregate[WindowJSONObjectAggBuilder]
	WindowPartitionable[WindowFrameablePartitionBuilder]

	// KeyColumn specifies the column to use as JSON object keys.
	KeyColumn(column string) WindowJSONObjectAggBuilder
	// KeyExpr specifies a raw expression to use as JSON object keys.
	KeyExpr(expr any) WindowJSONObjectAggBuilder
}

// WindowJSONArrayAggBuilder defines JSON_ARRAY_AGG() as window function builder.
type WindowJSONArrayAggBuilder interface {
	BaseAggregate[WindowJSONArrayAggBuilder]
	DistinctableAggregate[WindowJSONArrayAggBuilder]
	OrderableAggregate[WindowJSONArrayAggBuilder]
	WindowPartitionable[WindowFrameablePartitionBuilder]
}

// WindowBitOrBuilder defines BIT_OR() as window function builder.
type WindowBitOrBuilder interface {
	BaseAggregate[WindowBitOrBuilder]
	WindowPartitionable[WindowFrameablePartitionBuilder]
}

// WindowBitAndBuilder defines BIT_AND() as window function builder.
type WindowBitAndBuilder interface {
	BaseAggregate[WindowBitAndBuilder]
	WindowPartitionable[WindowFrameablePartitionBuilder]
}

// WindowBoolOrBuilder defines BOOL_OR() as window function builder.
type WindowBoolOrBuilder interface {
	BaseAggregate[WindowBoolOrBuilder]
	WindowPartitionable[WindowFrameablePartitionBuilder]
}

// WindowBoolAndBuilder defines BOOL_AND() as window function builder.
type WindowBoolAndBuilder interface {
	BaseAggregate[WindowBoolAndBuilder]
	WindowPartitionable[WindowFrameablePartitionBuilder]
}

type partitionExpr struct {
	builders ExprBuilder
	column   string
	expr     any
}

func (p *partitionExpr) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	if p.column != "" {
		return p.builders.Column(p.column).AppendQuery(gen, b)
	}

	if p.expr != nil {
		return p.builders.Expr("?", p.expr).AppendQuery(gen, b)
	}

	return b, nil
}

type baseWindowExpr struct {
	eb             ExprBuilder
	funcExpr       schema.QueryAppender
	funcName       string
	args           []any
	nullsMode      NullsMode
	fromDir        FromDirection
	partitionExprs []partitionExpr
	orderExprs     []orderExpr
	frameType      FrameType
	frameStartKind FrameBoundKind
	frameStartN    int
	frameEndKind   FrameBoundKind
	frameEndN      int
}

func (w *baseWindowExpr) setArgs(args ...any) {
	w.args = args
}

func (w *baseWindowExpr) appendPartitionBy(columns ...string) {
	for _, column := range columns {
		w.partitionExprs = append(w.partitionExprs, partitionExpr{
			builders: w.eb,
			column:   column,
		})
	}
}

func (w *baseWindowExpr) appendPartitionByExpr(expr any) {
	w.partitionExprs = append(w.partitionExprs, partitionExpr{
		builders: w.eb,
		expr:     expr,
	})
}

func (w *baseWindowExpr) appendOrderBy(columns ...string) {
	for _, column := range columns {
		w.orderExprs = append(w.orderExprs, orderExpr{
			builders:   w.eb,
			column:     column,
			direction:  sortx.OrderAsc,
			nullsOrder: sortx.NullsDefault,
		})
	}
}

func (w *baseWindowExpr) appendOrderByDesc(columns ...string) {
	for _, column := range columns {
		w.orderExprs = append(w.orderExprs, orderExpr{
			builders:   w.eb,
			column:     column,
			direction:  sortx.OrderDesc,
			nullsOrder: sortx.NullsDefault,
		})
	}
}

func (w *baseWindowExpr) appendOrderByExpr(expr any) {
	w.orderExprs = append(w.orderExprs, orderExpr{
		builders:   w.eb,
		expr:       expr,
		direction:  sortx.OrderAsc,
		nullsOrder: sortx.NullsDefault,
	})
}

func (w *baseWindowExpr) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	if w.funcExpr == nil {
		b = append(b, w.funcName...)
		b = append(b, '(')

		if len(w.args) > 0 {
			if b, err = w.eb.Exprs(w.args...).AppendQuery(gen, b); err != nil {
				return
			}
		}

		b = append(b, ')')

		if w.fromDir != FromDefault || w.nullsMode != NullsDefault {
			dialectBytes, err := w.eb.FragmentByDialect(DialectFragments{
				Oracle: func() ([]byte, error) {
					var dialectB []byte
					if w.fromDir != FromDefault {
						dialectB = append(dialectB, ' ')
						dialectB = append(dialectB, w.fromDir.String()...)
					}

					if w.nullsMode != NullsDefault {
						dialectB = append(dialectB, ' ')
						dialectB = append(dialectB, w.nullsMode.String()...)
					}

					return dialectB, nil
				},
				SQLServer: func() ([]byte, error) {
					var dialectB []byte
					if w.nullsMode != NullsDefault {
						dialectB = append(dialectB, ' ')
						dialectB = append(dialectB, w.nullsMode.String()...)
					}

					return dialectB, nil
				},
				Default: func() ([]byte, error) {
					return nil, nil
				},
			})
			if err != nil {
				return b, err
			}

			b = append(b, dialectBytes...)
		}
	} else {
		if b, err = w.funcExpr.AppendQuery(gen, b); err != nil {
			return
		}
	}

	b = append(b, " OVER "...)
	b = append(b, '(')

	if len(w.partitionExprs) > 0 {
		b = append(b, "PARTITION BY "...)

		for i, expr := range w.partitionExprs {
			if i > 0 {
				b = append(b, ", "...)
			}

			if b, err = expr.AppendQuery(gen, b); err != nil {
				return
			}
		}
	}

	if len(w.orderExprs) > 0 {
		b = append(b, ' ')
		if b, err = newOrderByClause(w.orderExprs...).AppendQuery(gen, b); err != nil {
			return
		}
	}

	if w.frameType != FrameDefault {
		if len(w.partitionExprs) > 0 || len(w.orderExprs) > 0 {
			b = append(b, ' ')
		}

		b = append(b, w.frameType.String()...)

		b = append(b, ' ')
		if w.frameEndKind != FrameBoundNone {
			b = append(b, "BETWEEN "...)
			b = w.appendFrameBound(b, w.frameStartKind, w.frameStartN)
			b = append(b, " AND "...)
			b = w.appendFrameBound(b, w.frameEndKind, w.frameEndN)
		} else {
			b = w.appendFrameBound(b, w.frameStartKind, w.frameStartN)
		}
	}

	b = append(b, ')')

	return b, nil
}

func (*baseWindowExpr) appendFrameBound(b []byte, kind FrameBoundKind, n int) []byte {
	switch kind {
	case FrameBoundUnboundedPreceding, FrameBoundUnboundedFollowing, FrameBoundCurrentRow:
		return append(b, kind.String()...)
	case FrameBoundPreceding, FrameBoundFollowing:
		b = strconv.AppendInt(b, int64(n), 10)
		b = append(b, ' ')

		return append(b, kind.String()...)

	default:
		return b
	}
}

func (w *baseWindowExpr) over() WindowFrameablePartitionBuilder {
	baseBuilder := &baseWindowPartitionBuilder[WindowFrameablePartitionBuilder]{
		baseWindowExpr: w,
	}
	builder := &baseWindowFrameablePartitionBuilder[WindowFrameablePartitionBuilder]{
		baseWindowPartitionBuilder: baseBuilder,
	}
	baseBuilder.self = builder

	return builder
}

func (w *baseWindowExpr) overSimple() WindowPartitionBuilder {
	builder := &baseWindowPartitionBuilder[WindowPartitionBuilder]{
		baseWindowExpr: w,
	}
	builder.self = builder

	return builder
}

type baseWindowPartitionBuilder[T any] struct {
	*baseWindowExpr

	self T
}

func (b *baseWindowPartitionBuilder[T]) PartitionBy(columns ...string) T {
	b.appendPartitionBy(columns...)

	return b.self
}

func (b *baseWindowPartitionBuilder[T]) PartitionByExpr(expr any) T {
	b.appendPartitionByExpr(expr)

	return b.self
}

func (b *baseWindowPartitionBuilder[T]) OrderBy(columns ...string) T {
	b.appendOrderBy(columns...)

	return b.self
}

func (b *baseWindowPartitionBuilder[T]) OrderByDesc(columns ...string) T {
	b.appendOrderByDesc(columns...)

	return b.self
}

func (b *baseWindowPartitionBuilder[T]) OrderByExpr(expr any) T {
	b.appendOrderByExpr(expr)

	return b.self
}

type baseWindowFrameablePartitionBuilder[T any] struct {
	*baseWindowPartitionBuilder[T]
}

func (b *baseWindowFrameablePartitionBuilder[T]) Rows() WindowFrameBuilder {
	b.frameType = FrameRows

	return &windowFrameBuilder{baseWindowExpr: b.baseWindowExpr}
}

func (b *baseWindowFrameablePartitionBuilder[T]) Range() WindowFrameBuilder {
	b.frameType = FrameRange

	return &windowFrameBuilder{baseWindowExpr: b.baseWindowExpr}
}

func (b *baseWindowFrameablePartitionBuilder[T]) Groups() WindowFrameBuilder {
	b.frameType = FrameGroups

	return &windowFrameBuilder{baseWindowExpr: b.baseWindowExpr}
}

type windowFrameBuilder struct {
	*baseWindowExpr
}

func (b *windowFrameBuilder) UnboundedPreceding() WindowFrameBuilder {
	b.frameStartKind = FrameBoundUnboundedPreceding

	return b
}

func (b *windowFrameBuilder) CurrentRow() WindowFrameBuilder {
	b.frameStartKind = FrameBoundCurrentRow

	return b
}

func (b *windowFrameBuilder) Preceding(n int) WindowFrameBuilder {
	b.frameStartKind = FrameBoundPreceding
	b.frameStartN = n

	return b
}

func (b *windowFrameBuilder) Following(n int) WindowFrameBuilder {
	b.frameStartKind = FrameBoundFollowing
	b.frameStartN = n

	return b
}

func (b *windowFrameBuilder) And() WindowFrameEndBuilder {
	return &windowFrameEndBuilder{baseWindowExpr: b.baseWindowExpr}
}

type windowFrameEndBuilder struct {
	*baseWindowExpr
}

func (b *windowFrameEndBuilder) UnboundedPreceding() WindowFrameEndBuilder {
	b.frameEndKind = FrameBoundUnboundedPreceding

	return b
}

func (b *windowFrameEndBuilder) CurrentRow() WindowFrameEndBuilder {
	b.frameEndKind = FrameBoundCurrentRow

	return b
}

func (b *windowFrameEndBuilder) Preceding(n int) WindowFrameEndBuilder {
	b.frameEndKind = FrameBoundPreceding
	b.frameEndN = n

	return b
}

func (b *windowFrameEndBuilder) Following(n int) WindowFrameEndBuilder {
	b.frameEndKind = FrameBoundFollowing
	b.frameEndN = n

	return b
}

func (b *windowFrameEndBuilder) UnboundedFollowing() WindowFrameEndBuilder {
	b.frameEndKind = FrameBoundUnboundedFollowing

	return b
}

type baseWindowNullHandlingBuilder[T any] struct {
	*baseWindowExpr

	self T
}

func (b *baseWindowNullHandlingBuilder[T]) IgnoreNulls() T {
	b.nullsMode = NullsIgnore

	return b.self
}

func (b *baseWindowNullHandlingBuilder[T]) RespectNulls() T {
	b.nullsMode = NullsRespect

	return b.self
}

// simpleWindowExpr is a shared implementation for parameterless window functions
// (ROW_NUMBER, RANK, DENSE_RANK, PERCENT_RANK, CUME_DIST).
type simpleWindowExpr struct {
	*baseWindowExpr
}

func (s *simpleWindowExpr) Over() WindowFrameablePartitionBuilder {
	return s.over()
}

type nTileExpr struct {
	*baseWindowExpr
}

func (ne *nTileExpr) Over() WindowFrameablePartitionBuilder {
	return ne.over()
}

func (ne *nTileExpr) Buckets(n int) NTileBuilder {
	ne.setArgs(n)

	return ne
}

type offsetWindowExpr[T any] struct {
	*baseWindowExpr

	self         T
	column       string
	expr         any
	offset       int
	defaultValue any
}

func (o *offsetWindowExpr[T]) Over() WindowPartitionBuilder {
	return o.overSimple()
}

func (o *offsetWindowExpr[T]) Column(column string) T {
	o.column = column
	o.expr = nil

	return o.self
}

func (o *offsetWindowExpr[T]) Expr(expr any) T {
	o.expr = expr
	o.column = ""

	return o.self
}

func (o *offsetWindowExpr[T]) Offset(offset int) T {
	o.offset = offset

	return o.self
}

func (o *offsetWindowExpr[T]) DefaultValue(value any) T {
	o.defaultValue = value

	return o.self
}

func (o *offsetWindowExpr[T]) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	var args []any

	if o.column != "" {
		args = append(args, o.eb.Column(o.column))
	} else if o.expr != nil {
		args = append(args, o.expr)
	}

	if o.offset > 0 {
		args = append(args, o.offset)
	}

	if o.defaultValue != nil {
		args = append(args, o.defaultValue)
	}

	o.setArgs(args...)

	return o.baseWindowExpr.AppendQuery(gen, b)
}

type lagExpr struct {
	*offsetWindowExpr[LagBuilder]
}

type leadExpr struct {
	*offsetWindowExpr[LeadBuilder]
}

type firstValueExpr struct {
	*baseWindowExpr
	*baseWindowNullHandlingBuilder[FirstValueBuilder]
}

func (fv *firstValueExpr) Over() WindowFrameablePartitionBuilder {
	return fv.over()
}

func (fv *firstValueExpr) Column(column string) FirstValueBuilder {
	fv.setArgs(fv.eb.Column(column))

	return fv
}

func (fv *firstValueExpr) Expr(expr any) FirstValueBuilder {
	fv.setArgs(expr)

	return fv
}

type lastValueExpr struct {
	*baseWindowExpr
	*baseWindowNullHandlingBuilder[LastValueBuilder]
}

func (lv *lastValueExpr) Over() WindowFrameablePartitionBuilder {
	return lv.over()
}

func (lv *lastValueExpr) Column(column string) LastValueBuilder {
	lv.setArgs(lv.eb.Column(column))

	return lv
}

func (lv *lastValueExpr) Expr(expr any) LastValueBuilder {
	lv.setArgs(expr)

	return lv
}

type nthValueExpr struct {
	*baseWindowExpr
	*baseWindowNullHandlingBuilder[NthValueBuilder]

	column string
	expr   any
	n      int
}

func (nv *nthValueExpr) Over() WindowFrameablePartitionBuilder {
	return nv.over()
}

func (nv *nthValueExpr) Column(column string) NthValueBuilder {
	nv.column = column
	nv.expr = nil

	return nv
}

func (nv *nthValueExpr) Expr(expr any) NthValueBuilder {
	nv.expr = expr
	nv.column = ""

	return nv
}

func (nv *nthValueExpr) N(nth int) NthValueBuilder {
	nv.n = nth

	return nv
}

func (nv *nthValueExpr) FromFirst() NthValueBuilder {
	nv.fromDir = FromFirst

	return nv
}

func (nv *nthValueExpr) FromLast() NthValueBuilder {
	nv.fromDir = FromLast

	return nv
}

func (nv *nthValueExpr) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	var args []any
	if nv.column != "" {
		args = append(args, nv.eb.Column(nv.column))
	} else if nv.expr != nil {
		args = append(args, nv.expr)
	}

	args = append(args, nv.n)
	nv.setArgs(args...)

	return nv.baseWindowExpr.AppendQuery(gen, b)
}

type windowCountExpr struct {
	*countExpr[WindowCountBuilder]
	*baseWindowExpr
}

func (wc *windowCountExpr) Over() WindowFrameablePartitionBuilder {
	return wc.over()
}

func (wc *windowCountExpr) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	wc.funcExpr = wc.countExpr

	return wc.baseWindowExpr.AppendQuery(gen, b)
}

type windowSumExpr struct {
	*distinctableAggExpr[WindowSumBuilder]
	*baseWindowExpr
}

func (ws *windowSumExpr) Over() WindowFrameablePartitionBuilder {
	return ws.over()
}

func (ws *windowSumExpr) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	ws.funcExpr = ws.distinctableAggExpr

	return ws.baseWindowExpr.AppendQuery(gen, b)
}

type windowAvgExpr struct {
	*distinctableAggExpr[WindowAvgBuilder]
	*baseWindowExpr
}

func (wa *windowAvgExpr) Over() WindowFrameablePartitionBuilder {
	return wa.over()
}

func (wa *windowAvgExpr) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	wa.funcExpr = wa.distinctableAggExpr

	return wa.baseWindowExpr.AppendQuery(gen, b)
}

type windowMinExpr struct {
	*simpleAggExpr[WindowMinBuilder]
	*baseWindowExpr
}

func (wm *windowMinExpr) Over() WindowFrameablePartitionBuilder {
	return wm.over()
}

func (wm *windowMinExpr) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	wm.funcExpr = wm.simpleAggExpr

	return wm.baseWindowExpr.AppendQuery(gen, b)
}

type windowMaxExpr struct {
	*simpleAggExpr[WindowMaxBuilder]
	*baseWindowExpr
}

func (wm *windowMaxExpr) Over() WindowFrameablePartitionBuilder {
	return wm.over()
}

func (wm *windowMaxExpr) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	wm.funcExpr = wm.simpleAggExpr

	return wm.baseWindowExpr.AppendQuery(gen, b)
}

type windowStringAggExpr struct {
	*stringAggExpr[WindowStringAggBuilder]
	*baseWindowExpr
}

func (ws *windowStringAggExpr) Over() WindowFrameablePartitionBuilder {
	return ws.over()
}

func (ws *windowStringAggExpr) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	ws.funcExpr = ws.stringAggExpr

	return ws.baseWindowExpr.AppendQuery(gen, b)
}

type windowArrayAggExpr struct {
	*arrayAggExpr[WindowArrayAggBuilder]
	*baseWindowExpr
}

func (wa *windowArrayAggExpr) Over() WindowFrameablePartitionBuilder {
	return wa.over()
}

func (wa *windowArrayAggExpr) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	wa.funcExpr = wa.arrayAggExpr

	return wa.baseWindowExpr.AppendQuery(gen, b)
}

type windowStdDevExpr struct {
	*stdDevExpr[WindowStdDevBuilder]
	*baseWindowExpr
}

func (ws *windowStdDevExpr) Over() WindowFrameablePartitionBuilder {
	return ws.over()
}

func (ws *windowStdDevExpr) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	ws.funcExpr = ws.stdDevExpr

	return ws.baseWindowExpr.AppendQuery(gen, b)
}

type windowVarianceExpr struct {
	*varianceExpr[WindowVarianceBuilder]
	*baseWindowExpr
}

func (wv *windowVarianceExpr) Over() WindowFrameablePartitionBuilder {
	return wv.over()
}

func (wv *windowVarianceExpr) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	wv.funcExpr = wv.varianceExpr

	return wv.baseWindowExpr.AppendQuery(gen, b)
}

type windowJSONObjectAggExpr struct {
	*jsonObjectAggExpr[WindowJSONObjectAggBuilder]
	*baseWindowExpr
}

func (wj *windowJSONObjectAggExpr) Over() WindowFrameablePartitionBuilder {
	return wj.over()
}

func (wj *windowJSONObjectAggExpr) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	wj.funcExpr = wj.jsonObjectAggExpr

	return wj.baseWindowExpr.AppendQuery(gen, b)
}

type windowJSONArrayAggExpr struct {
	*jsonArrayAggExpr[WindowJSONArrayAggBuilder]
	*baseWindowExpr
}

func (wj *windowJSONArrayAggExpr) Over() WindowFrameablePartitionBuilder {
	return wj.over()
}

func (wj *windowJSONArrayAggExpr) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	wj.funcExpr = wj.jsonArrayAggExpr

	return wj.baseWindowExpr.AppendQuery(gen, b)
}

type windowBitOrExpr struct {
	*dialectAggExpr[WindowBitOrBuilder]
	*baseWindowExpr
}

func (wb *windowBitOrExpr) Over() WindowFrameablePartitionBuilder {
	return wb.over()
}

func (wb *windowBitOrExpr) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	wb.funcExpr = wb.dialectAggExpr

	return wb.baseWindowExpr.AppendQuery(gen, b)
}

type windowBitAndExpr struct {
	*dialectAggExpr[WindowBitAndBuilder]
	*baseWindowExpr
}

func (wb *windowBitAndExpr) Over() WindowFrameablePartitionBuilder {
	return wb.over()
}

func (wb *windowBitAndExpr) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	wb.funcExpr = wb.dialectAggExpr

	return wb.baseWindowExpr.AppendQuery(gen, b)
}

type windowBoolOrExpr struct {
	*dialectAggExpr[WindowBoolOrBuilder]
	*baseWindowExpr
}

func (wb *windowBoolOrExpr) Over() WindowFrameablePartitionBuilder {
	return wb.over()
}

func (wb *windowBoolOrExpr) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	wb.funcExpr = wb.dialectAggExpr

	return wb.baseWindowExpr.AppendQuery(gen, b)
}

type windowBoolAndExpr struct {
	*dialectAggExpr[WindowBoolAndBuilder]
	*baseWindowExpr
}

func (wb *windowBoolAndExpr) Over() WindowFrameablePartitionBuilder {
	return wb.over()
}

func (wb *windowBoolAndExpr) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	wb.funcExpr = wb.dialectAggExpr

	return wb.baseWindowExpr.AppendQuery(gen, b)
}

func newSimpleWindowExpr(eb ExprBuilder, funcName string) *simpleWindowExpr {
	return &simpleWindowExpr{
		baseWindowExpr: &baseWindowExpr{
			eb:       eb,
			funcName: funcName,
		},
	}
}

func newRowNumberExpr(eb ExprBuilder) *simpleWindowExpr {
	return newSimpleWindowExpr(eb, "ROW_NUMBER")
}

func newRankExpr(eb ExprBuilder) *simpleWindowExpr {
	return newSimpleWindowExpr(eb, "RANK")
}

func newDenseRankExpr(eb ExprBuilder) *simpleWindowExpr {
	return newSimpleWindowExpr(eb, "DENSE_RANK")
}

func newPercentRankExpr(eb ExprBuilder) *simpleWindowExpr {
	return newSimpleWindowExpr(eb, "PERCENT_RANK")
}

func newCumeDistExpr(eb ExprBuilder) *simpleWindowExpr {
	return newSimpleWindowExpr(eb, "CUME_DIST")
}

func newNTileExpr(eb ExprBuilder) *nTileExpr {
	return &nTileExpr{
		baseWindowExpr: &baseWindowExpr{
			eb:       eb,
			funcName: "NTILE",
		},
	}
}

func newLagExpr(eb ExprBuilder) *lagExpr {
	expr := &lagExpr{
		offsetWindowExpr: &offsetWindowExpr[LagBuilder]{
			baseWindowExpr: &baseWindowExpr{
				eb:       eb,
				funcName: "LAG",
			},
			offset: 1,
		},
	}
	expr.self = expr

	return expr
}

func newLeadExpr(eb ExprBuilder) *leadExpr {
	expr := &leadExpr{
		offsetWindowExpr: &offsetWindowExpr[LeadBuilder]{
			baseWindowExpr: &baseWindowExpr{
				eb:       eb,
				funcName: "LEAD",
			},
			offset: 1,
		},
	}
	expr.self = expr

	return expr
}

func newFirstValueExpr(eb ExprBuilder) *firstValueExpr {
	baseExpr := &baseWindowExpr{
		eb:       eb,
		funcName: "FIRST_VALUE",
	}
	baseBuilder := &baseWindowNullHandlingBuilder[FirstValueBuilder]{
		baseWindowExpr: baseExpr,
	}
	expr := &firstValueExpr{
		baseWindowExpr:                baseExpr,
		baseWindowNullHandlingBuilder: baseBuilder,
	}

	baseBuilder.self = expr

	return expr
}

func newLastValueExpr(eb ExprBuilder) *lastValueExpr {
	baseExpr := &baseWindowExpr{
		eb:       eb,
		funcName: "LAST_VALUE",
	}
	baseBuilder := &baseWindowNullHandlingBuilder[LastValueBuilder]{
		baseWindowExpr: baseExpr,
	}
	expr := &lastValueExpr{
		baseWindowExpr:                baseExpr,
		baseWindowNullHandlingBuilder: baseBuilder,
	}

	baseBuilder.self = expr

	return expr
}

func newNthValueExpr(eb ExprBuilder) *nthValueExpr {
	baseExpr := &baseWindowExpr{
		eb:       eb,
		funcName: "NTH_VALUE",
	}
	baseBuilder := &baseWindowNullHandlingBuilder[NthValueBuilder]{
		baseWindowExpr: baseExpr,
	}
	expr := &nthValueExpr{
		baseWindowExpr:                baseExpr,
		baseWindowNullHandlingBuilder: baseBuilder,
	}

	baseBuilder.self = expr

	return expr
}

func newWindowCountExpr(qb QueryBuilder) *windowCountExpr {
	expr := &windowCountExpr{
		baseWindowExpr: &baseWindowExpr{
			eb: qb.ExprBuilder(),
		},
	}

	expr.countExpr = newGenericCountExpr[WindowCountBuilder](expr, qb)

	return expr
}

func newWindowSumExpr(qb QueryBuilder) *windowSumExpr {
	expr := &windowSumExpr{
		baseWindowExpr: &baseWindowExpr{
			eb: qb.ExprBuilder(),
		},
	}

	expr.distinctableAggExpr = newGenericDistinctableAggExpr[WindowSumBuilder](expr, qb, "SUM")

	return expr
}

func newWindowAvgExpr(qb QueryBuilder) *windowAvgExpr {
	expr := &windowAvgExpr{
		baseWindowExpr: &baseWindowExpr{
			eb: qb.ExprBuilder(),
		},
	}

	expr.distinctableAggExpr = newGenericDistinctableAggExpr[WindowAvgBuilder](expr, qb, "AVG")

	return expr
}

func newWindowMinExpr(qb QueryBuilder) *windowMinExpr {
	expr := &windowMinExpr{
		baseWindowExpr: &baseWindowExpr{
			eb: qb.ExprBuilder(),
		},
	}

	expr.simpleAggExpr = newGenericMinExpr[WindowMinBuilder](expr, qb)

	return expr
}

func newWindowMaxExpr(qb QueryBuilder) *windowMaxExpr {
	expr := &windowMaxExpr{
		baseWindowExpr: &baseWindowExpr{
			eb: qb.ExprBuilder(),
		},
	}

	expr.simpleAggExpr = newGenericMaxExpr[WindowMaxBuilder](expr, qb)

	return expr
}

func newWindowStringAggExpr(qb QueryBuilder) *windowStringAggExpr {
	expr := &windowStringAggExpr{
		baseWindowExpr: &baseWindowExpr{
			eb: qb.ExprBuilder(),
		},
	}

	expr.stringAggExpr = newGenericStringAggExpr[WindowStringAggBuilder](expr, qb)

	return expr
}

func newWindowArrayAggExpr(qb QueryBuilder) *windowArrayAggExpr {
	expr := &windowArrayAggExpr{
		baseWindowExpr: &baseWindowExpr{
			eb: qb.ExprBuilder(),
		},
	}

	expr.arrayAggExpr = newGenericArrayAggExpr[WindowArrayAggBuilder](expr, qb)

	return expr
}

func newWindowStdDevExpr(qb QueryBuilder) *windowStdDevExpr {
	expr := &windowStdDevExpr{
		baseWindowExpr: &baseWindowExpr{
			eb: qb.ExprBuilder(),
		},
	}

	expr.stdDevExpr = newGenericStdDevExpr[WindowStdDevBuilder](expr, qb)

	return expr
}

func newWindowVarianceExpr(qb QueryBuilder) *windowVarianceExpr {
	expr := &windowVarianceExpr{
		baseWindowExpr: &baseWindowExpr{
			eb: qb.ExprBuilder(),
		},
	}

	expr.varianceExpr = newGenericVarianceExpr[WindowVarianceBuilder](expr, qb)

	return expr
}

func newWindowJSONObjectAggExpr(qb QueryBuilder) *windowJSONObjectAggExpr {
	expr := &windowJSONObjectAggExpr{
		baseWindowExpr: &baseWindowExpr{
			eb: qb.ExprBuilder(),
		},
	}

	expr.jsonObjectAggExpr = newGenericJSONObjectAggExpr[WindowJSONObjectAggBuilder](expr, qb)

	return expr
}

func newWindowJSONArrayAggExpr(qb QueryBuilder) *windowJSONArrayAggExpr {
	expr := &windowJSONArrayAggExpr{
		baseWindowExpr: &baseWindowExpr{
			eb: qb.ExprBuilder(),
		},
	}

	expr.jsonArrayAggExpr = newGenericJSONArrayAggExpr[WindowJSONArrayAggBuilder](expr, qb)

	return expr
}

func newWindowBitOrExpr(qb QueryBuilder) *windowBitOrExpr {
	expr := &windowBitOrExpr{
		baseWindowExpr: &baseWindowExpr{
			eb: qb.ExprBuilder(),
		},
	}

	expr.dialectAggExpr = newGenericDialectAggExpr[WindowBitOrBuilder](expr, qb, bitOrStrategy)

	return expr
}

func newWindowBitAndExpr(qb QueryBuilder) *windowBitAndExpr {
	expr := &windowBitAndExpr{
		baseWindowExpr: &baseWindowExpr{
			eb: qb.ExprBuilder(),
		},
	}

	expr.dialectAggExpr = newGenericDialectAggExpr[WindowBitAndBuilder](expr, qb, bitAndStrategy)

	return expr
}

func newWindowBoolOrExpr(qb QueryBuilder) *windowBoolOrExpr {
	expr := &windowBoolOrExpr{
		baseWindowExpr: &baseWindowExpr{
			eb: qb.ExprBuilder(),
		},
	}

	expr.dialectAggExpr = newGenericDialectAggExpr[WindowBoolOrBuilder](expr, qb, boolOrStrategy)

	return expr
}

func newWindowBoolAndExpr(qb QueryBuilder) *windowBoolAndExpr {
	expr := &windowBoolAndExpr{
		baseWindowExpr: &baseWindowExpr{
			eb: qb.ExprBuilder(),
		},
	}

	expr.dialectAggExpr = newGenericDialectAggExpr[WindowBoolAndBuilder](expr, qb, boolAndStrategy)

	return expr
}
