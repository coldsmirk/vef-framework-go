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
// at generation time rather than silently shadowing the built-in column.
var reservedColumns = map[string]struct{}{
	"id":          {},
	"instance_id": {},
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
// schema can be turned into a safe, unique, non-reserved physical column
// identifier. It is the deploy-time gate for StorageTable: calling it when a
// version is deployed surfaces an unmappable field key as a configuration error
// the admin sees on save, instead of deferring the failure to publish (where it
// would surface opaquely). The publish-time generator re-derives and re-checks
// the same identifiers as defense-in-depth, so this never weakens the guarantee.
func ValidateTableFormSchema(schema *approval.FormDefinition) error {
	if schema == nil {
		return nil
	}

	// Seed with the built-in columns so a field key colliding with one of them
	// is reported as reserved, and two field keys sanitizing to the same column
	// are reported as a duplicate.
	seen := map[string]struct{}{"id": {}, "instance_id": {}, "created_at": {}}

	for _, field := range schema.Fields {
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
	}

	return nil
}

// buildColumnSpecs resolves the full ordered column set for a form schema in a
// given dialect: the built-in id / instance_id columns first, then one column
// per form field (in the schema's declared order), then created_at last. Field
// keys are validated as safe, unique, non-reserved identifiers up front by
// ValidateTableFormSchema, so the per-field names produced here are known good.
func buildColumnSpecs(kind config.DBKind, schema *approval.FormDefinition) ([]columnSpec, error) {
	t := typesFor(kind)
	if t == (sqlTypes{}) {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedDialect, kind)
	}

	if err := ValidateTableFormSchema(schema); err != nil {
		return nil, err
	}

	specs := make([]columnSpec, 0, fieldCount(schema)+3)

	// Built-in leading columns. id is the table PK; instance_id links back to
	// apv_instance.id and carries a UNIQUE constraint so each instance projects
	// exactly one row (start and resubmit replace, never append).
	specs = append(specs,
		columnSpec{Name: "id", Type: t.pk, IsNullable: false, SortOrder: 0},
		columnSpec{Name: "instance_id", Type: t.id, IsNullable: false, SortOrder: 1},
	)

	sortOrder := 2

	if schema != nil {
		for _, field := range schema.Fields {
			key := field.Key
			specs = append(specs, columnSpec{
				Name:           sanitizeForIdentifier(field.Key),
				Type:           columnTypeFor(kind, t, field),
				IsNullable:     !field.IsRequired,
				SourceFieldKey: &key,
				SortOrder:      sortOrder,
			})
			sortOrder++
		}
	}

	specs = append(specs, columnSpec{Name: "created_at", Type: t.timestamp, IsNullable: false, SortOrder: sortOrder})

	return specs, nil
}

// renderCreateTable builds the CREATE TABLE statement for the resolved column
// specs. tableName and every column name are already validated identifiers
// (buildPhysicalTableName / buildColumnSpecs), so interpolating them with
// fmt.Sprintf is safe; no caller data reaches this string. instance_id carries
// an inline UNIQUE constraint, which every dialect backs with an automatically
// named implicit index — enforcing one row per instance and serving instance_id
// lookups without a separately-named (and potentially truncation-colliding)
// index.
func renderCreateTable(kind config.DBKind, tableName string, specs []columnSpec) (string, error) {
	if err := approval.ValidateBusinessIdentifier(tableName); err != nil {
		return "", fmt.Errorf("%w: %q: %w", ErrInvalidGeneratedIdentifier, tableName, err)
	}

	t := typesFor(kind)
	if t == (sqlTypes{}) {
		return "", fmt.Errorf("%w: %q", ErrUnsupportedDialect, kind)
	}

	lines := make([]string, 0, len(specs))
	for _, spec := range specs {
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
			lines = append(lines, fmt.Sprintf("    %s %s %s", spec.Name, spec.Type, pkConstraint(kind, tableName)))
		case "instance_id":
			lines = append(lines, fmt.Sprintf("    %s %s NOT NULL UNIQUE", spec.Name, spec.Type))
		case "created_at":
			lines = append(lines, fmt.Sprintf("    %s %s NOT NULL DEFAULT %s", spec.Name, spec.Type, t.now))
		default:
			lines = append(lines, fmt.Sprintf("    %s %s%s", spec.Name, spec.Type, nullClause))
		}
	}

	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n%s\n)", tableName, strings.Join(lines, ",\n")), nil
}

func fieldCount(schema *approval.FormDefinition) int {
	if schema == nil {
		return 0
	}

	return len(schema.Fields)
}
