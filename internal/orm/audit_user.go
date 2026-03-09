package orm

import (
	"reflect"

	"github.com/uptrace/bun/schema"
)

// operatorExprBuilder returns the current operator expression for audit columns.
func operatorExprBuilder(eb ExprBuilder) any {
	return eb.Expr(ExprOperator)
}

// CreatedByHandler sets created_by using the current operator on insert.
type CreatedByHandler struct{}

func (cb *CreatedByHandler) OnInsert(query *BunInsertQuery, _ *schema.Table, _ *schema.Field, _ any, value reflect.Value) {
	if value.IsZero() {
		query.ColumnExpr(cb.Name(), operatorExprBuilder)
	}
}

func (*CreatedByHandler) Name() string {
	return ColumnCreatedBy
}

// UpdatedByHandler sets updated_by using the current operator on insert and update.
type UpdatedByHandler struct{}

func (ub *UpdatedByHandler) OnUpdate(query *BunUpdateQuery, _ *schema.Table, _ *schema.Field, _ any, _ reflect.Value) {
	if query.hasSet {
		query.SetExpr(ub.Name(), operatorExprBuilder)
	} else {
		query.ColumnExpr(ub.Name(), operatorExprBuilder)
	}
}

func (ub *UpdatedByHandler) OnInsert(query *BunInsertQuery, _ *schema.Table, _ *schema.Field, _ any, value reflect.Value) {
	if value.IsZero() {
		query.ColumnExpr(ub.Name(), operatorExprBuilder)
	}
}

func (*UpdatedByHandler) Name() string {
	return ColumnUpdatedBy
}
