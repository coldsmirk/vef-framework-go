package orm

import (
	"reflect"

	"github.com/uptrace/bun/schema"

	collections "github.com/coldsmirk/go-collections"
)

// autoColumnHandlers manages audit fields (ID, timestamps, user tracking) on insert/update.
var autoColumnHandlers = []ColumnHandler{
	&IDHandler{},
	&CreatedAtHandler{},
	&UpdatedAtHandler{},
	&CreatedByHandler{},
	&UpdatedByHandler{},
}

type InsertAutoColumnPlanItem struct {
	field   *schema.Field
	handler InsertColumnHandler
}

type UpdateAutoColumnPlanItem struct {
	field   *schema.Field
	handler UpdateColumnHandler
}

var (
	insertAutoColumnPlanCache = collections.NewConcurrentHashMap[*schema.Table, []InsertAutoColumnPlanItem]()
	updateAutoColumnPlanCache = collections.NewConcurrentHashMap[*schema.Table, []UpdateAutoColumnPlanItem]()
)

// ColumnHandler provides the column name that the handler manages.
type ColumnHandler interface {
	// Name returns the database column name this handler manages (e.g., "id", "created_at").
	Name() string
}

// InsertColumnHandler manages columns automatically during insert operations.
type InsertColumnHandler interface {
	ColumnHandler
	// OnInsert sets the column value automatically when a new row is inserted.
	OnInsert(query *BunInsertQuery, table *schema.Table, field *schema.Field, model any, value reflect.Value)
}

// UpdateColumnHandler manages columns during both insert and update operations.
type UpdateColumnHandler interface {
	InsertColumnHandler
	// OnUpdate sets the column value automatically when an existing row is updated.
	OnUpdate(query *BunUpdateQuery, table *schema.Table, field *schema.Field, model any, value reflect.Value)
}

// processAutoColumns applies auto column handlers to a model before insert/update operations.
func processAutoColumns(query any, table *schema.Table, modelValue any, mv reflect.Value) {
	if !mv.IsValid() || (mv.Kind() == reflect.Pointer && mv.IsNil()) {
		return
	}

	switch q := query.(type) {
	case *BunInsertQuery:
		applyInsertAutoColumns(q, table, modelValue, mv, getInsertAutoColumnPlan(table))
	case *BunUpdateQuery:
		applyUpdateAutoColumns(q, table, modelValue, mv, getUpdateAutoColumnPlan(table))
	}
}

func getInsertAutoColumnPlan(table *schema.Table) []InsertAutoColumnPlanItem {
	plan, _ := insertAutoColumnPlanCache.GetOrCompute(table, func() []InsertAutoColumnPlanItem {
		items := make([]InsertAutoColumnPlanItem, 0, len(autoColumnHandlers))
		for _, handler := range autoColumnHandlers {
			insertHandler, ok := handler.(InsertColumnHandler)
			if !ok {
				continue
			}

			field, ok := table.FieldMap[handler.Name()]
			if !ok {
				continue
			}

			items = append(items, InsertAutoColumnPlanItem{
				field:   field,
				handler: insertHandler,
			})
		}

		return items
	})

	return plan
}

func getUpdateAutoColumnPlan(table *schema.Table) []UpdateAutoColumnPlanItem {
	plan, _ := updateAutoColumnPlanCache.GetOrCompute(table, func() []UpdateAutoColumnPlanItem {
		items := make([]UpdateAutoColumnPlanItem, 0, len(autoColumnHandlers))
		for _, handler := range autoColumnHandlers {
			updateHandler, ok := handler.(UpdateColumnHandler)
			if !ok {
				continue
			}

			field, ok := table.FieldMap[handler.Name()]
			if !ok {
				continue
			}

			items = append(items, UpdateAutoColumnPlanItem{
				field:   field,
				handler: updateHandler,
			})
		}

		return items
	})

	return plan
}

func applyInsertAutoColumns(
	query *BunInsertQuery,
	table *schema.Table,
	modelValue any,
	mv reflect.Value,
	plan []InsertAutoColumnPlanItem,
) {
	if !mv.IsValid() {
		return
	}

	// Handle slice values (batch operations) by processing each element.
	if mv.Kind() == reflect.Slice {
		for i := range mv.Len() {
			elem := mv.Index(i)
			if elem.Kind() == reflect.Pointer {
				if elem.IsNil() {
					continue
				}

				elem = elem.Elem()
			}

			if !elem.IsValid() {
				continue
			}

			applyInsertAutoColumns(query, table, elem.Interface(), elem, plan)
		}

		return
	}

	for _, item := range plan {
		item.handler.OnInsert(query, table, item.field, modelValue, item.field.Value(mv))
	}
}

func applyUpdateAutoColumns(
	query *BunUpdateQuery,
	table *schema.Table,
	modelValue any,
	mv reflect.Value,
	plan []UpdateAutoColumnPlanItem,
) {
	if !mv.IsValid() {
		return
	}

	// Handle slice values (batch operations) by processing each element.
	if mv.Kind() == reflect.Slice {
		for i := range mv.Len() {
			elem := mv.Index(i)
			if elem.Kind() == reflect.Pointer {
				if elem.IsNil() {
					continue
				}

				elem = elem.Elem()
			}

			if !elem.IsValid() {
				continue
			}

			applyUpdateAutoColumns(query, table, elem.Interface(), elem, plan)
		}

		return
	}

	for _, item := range plan {
		item.handler.OnUpdate(query, table, item.field, modelValue, item.field.Value(mv))
	}
}
