package orm

import (
	"github.com/uptrace/bun/schema"

	"github.com/coldsmirk/vef-framework-go/sortx"
)

// OrderBuilder provides a fluent interface for building ORDER BY clauses.
type OrderBuilder interface {
	// Column specifies the column name to order by
	Column(column string) OrderBuilder
	// Expr allows ordering by a raw SQL expression instead of a column name.
	Expr(expr any) OrderBuilder
	// Asc sets ascending sort direction.
	Asc() OrderBuilder
	// Desc sets descending sort direction.
	Desc() OrderBuilder
	// NullsFirst places NULL values before non-NULL values in the sort order.
	NullsFirst() OrderBuilder
	// NullsLast places NULL values after non-NULL values in the sort order.
	NullsLast() OrderBuilder
}

// orderExpr implements OrderBuilder interface.
type orderExpr struct {
	builders   ExprBuilder
	column     string
	direction  sortx.OrderDirection
	nullsOrder sortx.NullsOrder
	expr       any
}

func (o *orderExpr) Column(column string) OrderBuilder {
	o.column = column
	o.expr = nil

	return o
}

func (o *orderExpr) Expr(expr any) OrderBuilder {
	o.expr = expr
	o.column = ""

	return o
}

func (o *orderExpr) Asc() OrderBuilder {
	o.direction = sortx.OrderAsc

	return o
}

func (o *orderExpr) Desc() OrderBuilder {
	o.direction = sortx.OrderDesc

	return o
}

func (o *orderExpr) NullsFirst() OrderBuilder {
	o.nullsOrder = sortx.NullsFirst

	return o
}

func (o *orderExpr) NullsLast() OrderBuilder {
	o.nullsOrder = sortx.NullsLast

	return o
}

func (o *orderExpr) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	if o.column == "" && o.expr == nil {
		return nil, ErrMissingColumnOrExpression
	}

	if o.column != "" {
		b, err = o.builders.Column(o.column).AppendQuery(gen, b)
	} else {
		b, err = o.builders.Expr("?", o.expr).AppendQuery(gen, b)
	}

	if err != nil {
		return
	}

	b = append(b, ' ')
	b = append(b, o.direction.String()...)

	if o.nullsOrder != sortx.NullsDefault {
		b = append(b, ' ')
		b = append(b, o.nullsOrder.String()...)
	}

	return b, nil
}

type orderByClause struct {
	exprs []orderExpr
}

// AppendQuery renders the ORDER BY clause with all accumulated expressions.
func (o *orderByClause) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	b = append(b, "ORDER BY "...)

	for i, expr := range o.exprs {
		if i > 0 {
			b = append(b, ", "...)
		}

		if b, err = expr.AppendQuery(gen, b); err != nil {
			return
		}
	}

	return b, nil
}

func newOrderExpr(builders ExprBuilder) *orderExpr {
	return &orderExpr{
		builders:   builders,
		direction:  sortx.OrderAsc,
		nullsOrder: sortx.NullsDefault,
	}
}

func newOrderByClause(exprs ...orderExpr) *orderByClause {
	return &orderByClause{
		exprs: exprs,
	}
}
