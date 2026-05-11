package crud

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/samber/lo"
	"github.com/uptrace/bun/schema"

	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// columnRef pairs a logical name with a column identifier for validation.
type columnRef struct {
	name   string
	column string
}

// validateColumnsExist validates that the specified columns exist in the model schema.
func validateColumnsExist(schema *schema.Table, columns ...columnRef) error {
	for _, c := range columns {
		if c.column != "" && !schema.HasField(c.column) {
			return result.Err(i18n.T("field_not_exist_in_model", map[string]any{
				"field": c.column,
				"name":  c.name,
				"model": schema.TypeName,
			}))
		}
	}

	return nil
}

// validateOptionColumns validates columns for DataOptionColumnMapping.
func validateOptionColumns(schema *schema.Table, mapping *DataOptionColumnMapping) error {
	columns := []columnRef{
		{"labelColumn", mapping.LabelColumn},
		{"valueColumn", mapping.ValueColumn},
		{"descriptionColumn", mapping.DescriptionColumn},
	}

	return validateColumnsExist(schema, columns...)
}

// mergeOptionColumnMapping merges the provided mapping with default mapping.
// Uses fallback values for empty columns based on the provided default mapping or system defaults.
func mergeOptionColumnMapping(mapping, defaultMapping *DataOptionColumnMapping) {
	if defaultMapping == nil {
		defaultMapping = defaultDataOptionColumnMapping
	}

	if mapping.LabelColumn == "" {
		mapping.LabelColumn = lo.CoalesceOrEmpty(defaultMapping.LabelColumn, defaultLabelColumn)
	}

	if mapping.ValueColumn == "" {
		mapping.ValueColumn = lo.CoalesceOrEmpty(defaultMapping.ValueColumn, defaultValueColumn)
	}

	if mapping.DescriptionColumn == "" {
		mapping.DescriptionColumn = defaultMapping.DescriptionColumn
	}

	if len(mapping.MetaColumns) == 0 {
		mapping.MetaColumns = defaultMapping.MetaColumns
	}
}

// ApplyDataPermission applies data permission filtering to a SelectQuery.
func ApplyDataPermission(query orm.SelectQuery, ctx fiber.Ctx) error {
	if applier := contextx.DataPermApplier(ctx); applier != nil {
		if err := applier.Apply(query); err != nil {
			return fmt.Errorf("failed to apply data permission: %w", err)
		}
	}

	return nil
}

// GetAuditUserNameRelations returns RelationSpecs for creator and updater joins.
func GetAuditUserNameRelations(userModel any, nameColumn ...string) []*orm.RelationSpec {
	nc := defaultAuditUserNameColumn
	if len(nameColumn) > 0 {
		nc = nameColumn[0]
	}

	auditRelation := func(alias, fk, aliasColumn string) *orm.RelationSpec {
		return &orm.RelationSpec{
			Model:         userModel,
			Alias:         alias,
			JoinType:      orm.JoinLeft,
			ForeignColumn: fk,
			SelectedColumns: []orm.ColumnInfo{
				{Name: nc, Alias: aliasColumn},
			},
		}
	}

	return []*orm.RelationSpec{
		auditRelation("creator", "created_by", orm.ColumnCreatedByName),
		auditRelation("updater", "updated_by", orm.ColumnUpdatedByName),
	}
}

// columnAliasPattern matches "column AS alias" format (case-insensitive AS, flexible spaces).
var columnAliasPattern = regexp.MustCompile(`^\s*(.+?)\s+(?i:as)\s+(.+?)\s*$`)

// parseMetaColumn parses a single meta column specification into (column, alias).
// Supports formats:
//   - "column" -> ("column", "column")
//   - "column AS alias" -> ("column", "alias")
//   - "column as alias" -> ("column", "alias")
func parseMetaColumn(spec string) (column, alias string) {
	if matches := columnAliasPattern.FindStringSubmatch(spec); len(matches) == 3 {
		column = strings.TrimSpace(matches[1])
		alias = strings.TrimSpace(matches[2])

		return column, alias
	}

	// No alias specified, use column name as alias
	trimmed := strings.TrimSpace(spec)

	return trimmed, trimmed
}

// parseMetaColumns parses meta column specifications into structured info.
func parseMetaColumns(specs []string) []orm.ColumnInfo {
	if len(specs) == 0 {
		return nil
	}

	result := make([]orm.ColumnInfo, len(specs))
	for i, spec := range specs {
		columnName, aliasName := parseMetaColumn(spec)
		result[i] = orm.ColumnInfo{Name: columnName, Alias: aliasName}
	}

	return result
}

// validateMetaColumns validates that all meta columns exist in the table schema.
func validateMetaColumns(schema *schema.Table, metaColumns []orm.ColumnInfo) error {
	for _, col := range metaColumns {
		if !schema.HasField(col.Name) {
			return result.Err(i18n.T("field_not_exist_in_model", map[string]any{
				"field": col.Name,
				"name":  "metaColumns",
				"model": schema.TypeName,
			}))
		}
	}

	return nil
}

// buildMetaJSONExpr constructs a JSON_OBJECT expression for meta columns.
func buildMetaJSONExpr(eb orm.ExprBuilder, metaColumns []orm.ColumnInfo) schema.QueryAppender {
	jsonArgs := make([]any, 0, len(metaColumns)*2)
	for _, col := range metaColumns {
		jsonArgs = append(jsonArgs, col.Alias, eb.Column(col.Name))
	}

	return eb.JSONObject(jsonArgs...)
}

