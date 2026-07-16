package storage

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
)

// identifierMaxLen bounds every generated identifier (table and column names)
// to 63 characters — PostgreSQL's hard limit and the bound encoded by
// approval.ValidateBusinessIdentifier. MySQL (64) and SQLite (effectively
// unbounded) tolerate the same cap, so one limit keeps the generated DDL
// portable across all three dialects.
const identifierMaxLen = 63

// physicalTablePrefix is prepended to every generated form table so the
// projection tables share the apv_ namespace of the rest of the module and
// are visually distinct from the metadata tables (apv_form_table*).
const physicalTablePrefix = "apv_form_"

// reservedColumns are the columns the generator always emits regardless of the
// form schema. A form field whose key collides with one of these is rejected
// rather than silently shadowing the built-in column.
var reservedColumns = map[string]struct{}{
	"id":          {},
	"instance_id": {},
	"row_index":   {},
	"created_at":  {},
}

// nonIdentifierRune matches any character that is not a lowercase ASCII letter,
// digit, or underscore. sanitizeForIdentifier collapses runs of these into a
// single underscore so an arbitrary flow code becomes a candidate identifier;
// the result is still validated with approval.ValidateBusinessIdentifier before
// it reaches any DDL string.
var nonIdentifierRune = regexp.MustCompile(`[^a-z0-9_]+`)

// columnSpec is the resolved physical column the generator will both CREATE and
// record in apv_form_table_column. ColumnType is already dialect-specific.
type columnSpec struct {
	Name           string
	Type           string
	IsNullable     bool
	SourceFieldKey *string
	SortOrder      int
}

// tableSpec is one resolved physical table: the main projection table
// (SourceFieldKey == "") or one child table per detail-table field. Both
// generation paths — CREATE TABLE and metadata registration — consume the
// same specs, so the physical table and its recorded metadata can never
// disagree on identifiers; row writes then read that metadata.
type tableSpec struct {
	Name           string
	SourceFieldKey string
	Columns        []columnSpec
}

// IsChild reports whether the spec projects a detail-table field rather
// than the main form row.
func (s tableSpec) IsChild() bool { return s.SourceFieldKey != "" }

// buildPhysicalTableName derives the physical table name for a version. The
// version's globally-unique id (an XID) is the uniqueness guarantee, so the
// name can never collide across tenants, across flow codes that sanitize to the
// same string, or across truncated codes — distinct versions always yield
// distinct tables. The sanitized flow code is kept purely as a human-readable
// prefix (and truncated to leave room for the always-present id suffix); even
// an empty or all-punctuation code still produces a valid, unique name.
//
// versionID is a generated XID ([0-9a-v]{20}); it is composed after the leading
// "apv_form_" prefix so a leading digit can never start the identifier. The
// final composed name is validated as a defense-in-depth gate.
func buildPhysicalTableName(flowCode, versionID string) (string, error) {
	code := sanitizeForIdentifier(flowCode)

	// Budget for the readable code: total cap minus the prefix and the
	// "_<versionID>" suffix that always follows it.
	codeBudget := identifierMaxLen - len(physicalTablePrefix) - len(versionID) - 1
	if codeBudget > 0 && len(code) > codeBudget {
		code = code[:codeBudget]
	}

	// Trim a trailing underscore left by truncation so the name reads cleanly.
	code = strings.TrimRight(code, "_")

	var name string
	if codeBudget > 0 && code != "" {
		name = physicalTablePrefix + code + "_" + versionID
	} else {
		// No readable code (empty/all-punctuation) or no budget for one: the
		// id alone keeps the name valid and unique.
		name = physicalTablePrefix + versionID
	}

	if err := approval.ValidateBusinessIdentifier(name); err != nil {
		return "", fmt.Errorf("%w: generated table name %q rejected: %w", ErrInvalidGeneratedIdentifier, name, err)
	}

	return name, nil
}

// sanitizeForIdentifier lower-cases s and replaces every run of characters that
// are not [a-z0-9_] with a single underscore. It does NOT guarantee the result
// is a valid identifier (it may be empty or start with a digit); callers must
// still pass the final composed name through approval.ValidateBusinessIdentifier.
func sanitizeForIdentifier(s string) string {
	lowered := strings.ToLower(strings.TrimSpace(s))

	return nonIdentifierRune.ReplaceAllString(lowered, "_")
}

// ValidateTableFormSchema verifies that every form field key in a table-mode
// field list can be turned into a safe, unique, non-reserved physical column
// identifier. It is the deploy-time gate for StorageTable: calling it when a
// version is deployed surfaces an unmappable field key as a configuration error
// the admin sees on save, instead of deferring the failure to publish (where it
// would surface opaquely). The publish-time generator re-derives and re-checks
// the same identifiers as defense-in-depth, so this never weakens the guarantee.
func ValidateTableFormSchema(fields []approval.FormFieldDefinition) error {
	// Seed with the built-in columns so a field key colliding with one of them
	// is reported as reserved, and two field keys sanitizing to the same column
	// are reported as a duplicate. Table fields get their own pool per child
	// table — a child column only has to be unique within its table — plus a
	// shared pool of sanitized table keys, which become child table names.
	seen := reservedColumnPool()
	childNames := map[string]struct{}{}

	for _, field := range fields {
		if field.Kind == approval.FieldTable {
			if err := validateChildTableSchema(field, childNames); err != nil {
				return err
			}

			continue
		}

		if err := validateColumnField(field, seen); err != nil {
			return err
		}
	}

	return nil
}

// reservedColumnPool seeds a fresh identifier pool with the built-in columns.
func reservedColumnPool() map[string]struct{} {
	pool := make(map[string]struct{}, len(reservedColumns))
	for name := range reservedColumns {
		pool[name] = struct{}{}
	}

	return pool
}

// validateColumnField checks one field's mapping into the given identifier
// pool: a known column-type override, a safe sanitized identifier, and no
// collision with built-ins or earlier fields.
func validateColumnField(field approval.FormFieldDefinition, seen map[string]struct{}) error {
	// An explicit column-type override must be one of the defined logical
	// types; an empty ColumnType is valid (it falls back to a kind-derived
	// type). Catching it here fails an unknown type at deploy rather than
	// silently widening it to TEXT at publish.
	if field.ColumnType != "" && !field.ColumnType.IsValid() {
		return fmt.Errorf("%w: field key %q declares column type %q",
			ErrInvalidColumnType, field.Key, field.ColumnType)
	}

	column := sanitizeForIdentifier(field.Key)

	if err := approval.ValidateBusinessIdentifier(column); err != nil {
		return fmt.Errorf("%w: field key %q maps to invalid column %q: %w",
			ErrInvalidGeneratedIdentifier, field.Key, column, err)
	}

	if _, reserved := reservedColumns[column]; reserved {
		return fmt.Errorf("%w: field key %q collides with built-in column %q",
			ErrReservedColumnName, field.Key, column)
	}

	if _, dup := seen[column]; dup {
		return fmt.Errorf("%w: field key %q maps to already-used column %q",
			ErrDuplicateColumnName, field.Key, column)
	}

	seen[column] = struct{}{}

	return nil
}

// validateChildTableSchema checks a detail-table field's physical mapping:
// the sanitized table key (the child table's name suffix) must be a safe,
// unique identifier, and every column must map safely within the child
// table's own pool.
func validateChildTableSchema(field approval.FormFieldDefinition, childNames map[string]struct{}) error {
	// De-dupe on the SAME truncated suffix generation will emit — two long
	// keys sharing a prefix must collide here, at deploy, not silently at
	// publish where CREATE TABLE IF NOT EXISTS would mask the second table.
	suffix := childTableSuffix(field.Key)

	if err := approval.ValidateBusinessIdentifier(suffix); err != nil {
		return fmt.Errorf("%w: table field key %q maps to invalid name %q: %w",
			ErrInvalidGeneratedIdentifier, field.Key, suffix, err)
	}

	if _, dup := childNames[suffix]; dup {
		return fmt.Errorf("%w: table field key %q maps to already-used table name %q",
			ErrDuplicateColumnName, field.Key, suffix)
	}

	childNames[suffix] = struct{}{}

	pool := reservedColumnPool()
	for _, column := range field.Columns {
		if err := validateColumnField(column, pool); err != nil {
			return fmt.Errorf("in table %q: %w", field.Key, err)
		}
	}

	return nil
}

// buildTableSpecs resolves every physical table for a version's parsed form
// fields in a given dialect: the main projection table (built-in id /
// instance_id columns, one column per scalar field in declared order,
// created_at last) plus one child table per detail-table field (id /
// instance_id / row_index, the table's columns, created_at). Field keys are
// validated as safe, unique, non-reserved identifiers up front by
// ValidateTableFormSchema, so the names produced here are known good.
func buildTableSpecs(kind config.DBKind, flowCode, versionID string, fields []approval.FormFieldDefinition) ([]tableSpec, error) {
	t := typesFor(kind)
	if t == (sqlTypes{}) {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedDialect, kind)
	}

	if err := ValidateTableFormSchema(fields); err != nil {
		return nil, err
	}

	mainName, err := buildPhysicalTableName(flowCode, versionID)
	if err != nil {
		return nil, err
	}

	main := tableSpec{Name: mainName}

	// Built-in leading columns. id is the table PK; instance_id links back to
	// apv_instance.id and carries a UNIQUE constraint so each instance projects
	// exactly one row (start and resubmit replace, never append).
	main.Columns = append(main.Columns,
		columnSpec{Name: "id", Type: t.pk, IsNullable: false, SortOrder: 0},
		columnSpec{Name: "instance_id", Type: t.id, IsNullable: false, SortOrder: 1},
	)

	specs := []tableSpec{main}
	sortOrder := 2

	for _, field := range fields {
		if field.Kind == approval.FieldTable {
			child, err := buildChildTableSpec(kind, t, versionID, field)
			if err != nil {
				return nil, err
			}

			specs = append(specs, child)

			continue
		}

		key := field.Key
		specs[0].Columns = append(specs[0].Columns, columnSpec{
			Name:           sanitizeForIdentifier(field.Key),
			Type:           columnTypeFor(kind, t, field),
			IsNullable:     !field.IsRequired,
			SourceFieldKey: &key,
			SortOrder:      sortOrder,
		})
		sortOrder++
	}

	specs[0].Columns = append(specs[0].Columns, columnSpec{Name: "created_at", Type: t.timestamp, IsNullable: false, SortOrder: sortOrder})

	return specs, nil
}

// buildChildTableSpec resolves one detail-table field into its child table:
// name = apv_form_<versionID>__<sanitized field key> (fixed suffix budget),
// rows keyed by instance_id (indexed, NOT unique — many rows per instance)
// and ordered by row_index.
func buildChildTableSpec(kind config.DBKind, t sqlTypes, versionID string, field approval.FormFieldDefinition) (tableSpec, error) {
	name, err := buildChildTableName(versionID, field.Key)
	if err != nil {
		return tableSpec{}, err
	}

	child := tableSpec{Name: name, SourceFieldKey: field.Key}

	child.Columns = append(child.Columns,
		columnSpec{Name: "id", Type: t.pk, IsNullable: false, SortOrder: 0},
		columnSpec{Name: "instance_id", Type: t.id, IsNullable: false, SortOrder: 1},
		columnSpec{Name: "row_index", Type: t.integer, IsNullable: false, SortOrder: 2},
	)

	sortOrder := 3

	for _, column := range field.Columns {
		key := column.Key
		child.Columns = append(child.Columns, columnSpec{
			Name:           sanitizeForIdentifier(column.Key),
			Type:           columnTypeFor(kind, t, column),
			IsNullable:     !column.IsRequired,
			SourceFieldKey: &key,
			SortOrder:      sortOrder,
		})
		sortOrder++
	}

	child.Columns = append(child.Columns, columnSpec{Name: "created_at", Type: t.timestamp, IsNullable: false, SortOrder: sortOrder})

	return child, nil
}

// childSuffixBudget is the fixed identifier budget for a child table's
// field-key suffix: the 63-char cap minus the always-present prefix, the
// 20-char version XID, and the "__" separator. Deliberately independent of
// how much of the identifier space the main table's human-readable flow
// code consumed — a long flow code must never starve child tables.
const childSuffixBudget = identifierMaxLen - len(physicalTablePrefix) - 20 - 2

// buildChildTableName composes apv_form_<versionID>__<sanitized field key>.
// The version XID alone guarantees global uniqueness and the truncated
// suffix is unique per schema (ValidateTableFormSchema de-dupes the same
// truncation childTableSuffix applies), so composed names cannot collide.
func buildChildTableName(versionID, fieldKey string) (string, error) {
	name := physicalTablePrefix + versionID + "__" + childTableSuffix(fieldKey)

	if err := approval.ValidateBusinessIdentifier(name); err != nil {
		return "", fmt.Errorf("%w: generated child table name %q rejected: %w", ErrInvalidGeneratedIdentifier, name, err)
	}

	return name, nil
}

// childTableSuffix sanitizes and truncates a table field's key into the
// fixed child-name budget. Validation and generation share this single
// transform, so what validation approves is exactly what generation emits.
func childTableSuffix(fieldKey string) string {
	suffix := sanitizeForIdentifier(fieldKey)
	if len(suffix) > childSuffixBudget {
		suffix = strings.TrimRight(suffix[:childSuffixBudget], "_")
	}

	return suffix
}

// renderTableStatements renders every DDL statement one physical table
// needs: the CREATE TABLE plus, for child tables on dialects without inline
// index syntax, the instance_id lookup index. All statements are idempotent
// so a republish or retry re-runs safely.
func renderTableStatements(kind config.DBKind, table tableSpec) ([]string, error) {
	createSQL, err := renderCreateTable(kind, table)
	if err != nil {
		return nil, err
	}

	statements := []string{createSQL}

	// MySQL inlines the index inside CREATE TABLE (no CREATE INDEX IF NOT
	// EXISTS); the other dialects add it as a separate idempotent statement.
	if table.IsChild() && kind != config.MySQL {
		statements = append(statements, fmt.Sprintf(
			"CREATE INDEX IF NOT EXISTS %s ON %s(instance_id)", childIndexName(table.Name), table.Name,
		))
	}

	return statements, nil
}

// childIndexName derives the child table's instance_id index name within
// the shared identifier budget.
func childIndexName(tableName string) string {
	const suffix = "__instance_id"

	base := "idx_" + tableName
	if len(base)+len(suffix) > identifierMaxLen {
		base = strings.TrimRight(base[:identifierMaxLen-len(suffix)], "_")
	}

	return base + suffix
}

// renderCreateTable builds the CREATE TABLE statement for one resolved table
// spec. The table name and every column name are validated with
// approval.ValidateBusinessIdentifier before interpolation, so no caller data
// reaches the fmt.Sprintf string. The main table's instance_id carries an
// inline UNIQUE constraint, which every dialect backs with an automatically
// named implicit index — enforcing one row per instance and serving
// instance_id lookups without a separately-named (and potentially
// truncation-colliding) index. A child table's instance_id is plain NOT NULL
// (many rows per instance); its lookup index is inlined here for MySQL and
// added as a separate statement by renderTableStatements elsewhere.
func renderCreateTable(kind config.DBKind, table tableSpec) (string, error) {
	if err := approval.ValidateBusinessIdentifier(table.Name); err != nil {
		return "", fmt.Errorf("%w: %q: %w", ErrInvalidGeneratedIdentifier, table.Name, err)
	}

	t := typesFor(kind)
	if t == (sqlTypes{}) {
		return "", fmt.Errorf("%w: %q", ErrUnsupportedDialect, kind)
	}

	lines := make([]string, 0, len(table.Columns)+1)
	for _, spec := range table.Columns {
		// Defense-in-depth: re-validate every column name at the render step so
		// a spec smuggled in from elsewhere cannot turn the format string into
		// an injection vector.
		if err := approval.ValidateBusinessIdentifier(spec.Name); err != nil {
			return "", fmt.Errorf("%w: column %q: %w", ErrInvalidGeneratedIdentifier, spec.Name, err)
		}

		nullClause := " NOT NULL"
		if spec.IsNullable {
			nullClause = ""
		}

		switch spec.Name {
		case "id":
			lines = append(lines, fmt.Sprintf("    %s %s %s", spec.Name, spec.Type, pkConstraint(kind, table.Name)))
		case "instance_id":
			// The main table projects exactly one row per instance (start and
			// resubmit replace); child tables hold one row per detail line.
			if table.IsChild() {
				lines = append(lines, fmt.Sprintf("    %s %s NOT NULL", spec.Name, spec.Type))
			} else {
				lines = append(lines, fmt.Sprintf("    %s %s NOT NULL UNIQUE", spec.Name, spec.Type))
			}

		case "created_at":
			lines = append(lines, fmt.Sprintf("    %s %s NOT NULL DEFAULT %s", spec.Name, spec.Type, t.now))
		default:
			lines = append(lines, fmt.Sprintf("    %s %s%s", spec.Name, spec.Type, nullClause))
		}
	}

	if table.IsChild() && kind == config.MySQL {
		lines = append(lines, "    INDEX idx_instance_id (instance_id)")
	}

	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n%s\n)", table.Name, strings.Join(lines, ",\n")), nil
}
