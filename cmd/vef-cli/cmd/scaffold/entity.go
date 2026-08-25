package scaffold

import (
	"fmt"
	"strings"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/internal/naming"
	"github.com/coldsmirk/vef-framework-go/schema"
)

// mixinColumn is one column an audit mixin declares, with the Go type the
// mixin gives it.
type mixinColumn struct {
	name string
	typ  goType
}

// auditMixin is one of the framework's audit mixins: the embed to write, the
// columns it declares, and whether it supplies the model's primary key.
type auditMixin struct {
	embed string
	// declaresID reports whether the mixin carries `ID string bun:"id,pk"`.
	// The tracked variants deliberately do not — they exist for models whose
	// primary key is composite and therefore lives on the model itself.
	declaresID bool
	columns    []mixinColumn
}

// auditMixins are the framework's audit mixins, ordered widest first so a
// table carrying the full set matches FullAuditedModel rather than one of its
// subsets. Recognizing them is what keeps a generated model from re-declaring
// columns the framework already owns.
//
// The declared Go type is part of the match, not decoration. A mixin does not
// merely add fields, it asserts what those columns are: orm.Model declares
// `ID string`, and the ORM generates an XID into any zero-valued string
// primary key on insert. Matching a `BIGINT` identity column on its name alone
// would therefore write a 20-character string into a numeric column on every
// create — failing on PostgreSQL and silently coercing on a lax MySQL. A table
// whose audit columns are typed differently simply matches no mixin and has
// them generated as ordinary fields, which is honest and still correct.
var auditMixins = []auditMixin{
	{
		embed: "orm.FullAuditedModel", declaresID: true,
		columns: []mixinColumn{
			{"id", goString},
			{"created_at", goTime},
			{"created_by", goString},
			{"updated_at", goTime},
			{"updated_by", goString},
		},
	},
	{
		embed: "orm.FullTrackedModel",
		columns: []mixinColumn{
			{"created_at", goTime},
			{"created_by", goString},
			{"updated_at", goTime},
			{"updated_by", goString},
		},
	},
	{
		embed: "orm.CreationAuditedModel", declaresID: true,
		columns: []mixinColumn{
			{"id", goString}, {"created_at", goTime}, {"created_by", goString},
		},
	},
	{
		embed:   "orm.CreationTrackedModel",
		columns: []mixinColumn{{"created_at", goTime}, {"created_by", goString}},
	},
	{
		embed: "orm.Model", declaresID: true,
		columns: []mixinColumn{{"id", goString}},
	},
}

// searchOperator is the search tag a generated search field carries.
type searchOperator string

const (
	searchContains searchOperator = "contains"
	searchEquals   searchOperator = "eq"
)

// Field is one generated struct field derived from a table column.
type Field struct {
	// Column is the database column name.
	Column string
	// Name is the Go field name.
	Name string
	// JSONName is the field's json tag name.
	JSONName string
	// Type is the Go type spelling, already carrying a pointer for a nullable
	// column.
	Type string
	// Nullable reports whether the column accepts NULL, which decides both the
	// pointer and the validation prefix.
	Nullable bool
	// Label is the column comment, surfaced through the label tag that drives
	// validation messages and export headers.
	Label string
	// MaxLength is the declared character bound, zero when unbounded.
	MaxLength int
	// PrimaryKey reports whether the column is part of the table's primary
	// key and therefore needs a bun pk tag on the generated field.
	PrimaryKey bool
	// Search is the search operator this field takes in a search payload.
	Search searchOperator
	// ImportPath is the package the field's type needs, empty for builtins.
	ImportPath string
}

// ModelTag renders the struct tag of the field's model declaration.
func (f Field) ModelTag() string {
	return f.tag(f.jsonTag(), f.bunTag(), f.validateRule())
}

// bunTag renders the bun tag, which carries two things the ORM cannot work out
// on its own.
//
// The column name is stated explicitly whenever the field name does not derive
// back to it. bun re-derives a column from the field name, and that derivation
// is not the inverse of the one used here — api_v1 becomes APIV1, which
// derives back to apiv1 — so a field left untagged would silently address a
// column that does not exist. Verifying the round trip per field, rather than
// trying to make the naming heuristic reversible for every input, is what makes
// this correct for names nobody has thought of yet.
//
// A key column carries pk. Without it bun sees a model with no primary key at
// all and every crud Update and Delete fails with ErrModelNoPrimaryKey — which
// is what a composite-keyed table hits the moment it is generated, since the
// mixins that suit those tables deliberately declare no ID field.
func (f Field) bunTag() string {
	name := ""
	if naming.SnakeCase(f.Name) != f.Column {
		name = f.Column
	}

	switch {
	case f.PrimaryKey:
		return fmt.Sprintf("bun:%q", name+",pk")
	case name != "":
		return fmt.Sprintf("bun:%q", name)
	default:
		return ""
	}
}

// ParamsTag renders the struct tag of the field's create/update declaration.
// It differs from the model tag by never marking a field omitempty: a params
// struct is what the client sends, and a nil there means "not supplied", which
// is the distinction the pointer already carries.
func (f Field) ParamsTag() string {
	return f.tag(fmt.Sprintf("json:%q", f.JSONName), f.validateRule())
}

// SearchTag renders the struct tag of the field's search declaration.
func (f Field) SearchTag() string {
	parts := []string{fmt.Sprintf("json:%q", f.JSONName)}
	if f.Search != "" {
		parts = append(parts, fmt.Sprintf("search:%q", string(f.Search)))
	}

	return f.tag(parts...)
}

// SearchType is the field's type inside a search payload, where every criterion
// is optional regardless of the column's nullability.
func (f Field) SearchType() string {
	return "*" + strings.TrimPrefix(f.Type, "*")
}

func (f Field) tag(parts ...string) string {
	parts = append(parts, fmt.Sprintf("label:%q", f.Label))

	return "`" + strings.Join(nonEmpty(parts), " ") + "`"
}

func (f Field) jsonTag() string {
	if f.Nullable {
		return fmt.Sprintf("json:%q", f.JSONName+",omitempty")
	}

	return fmt.Sprintf("json:%q", f.JSONName)
}

// validateRule renders the validate tag from the column's nullability and
// declared bound. A NOT NULL column without a default is required; everything
// else is omitempty, which is how the framework's own models read.
func (f Field) validateRule() string {
	var rules []string

	if f.Nullable {
		rules = append(rules, "omitempty")
	} else if f.Type == "string" {
		rules = append(rules, "required")
	}

	if f.MaxLength > 0 {
		rules = append(rules, fmt.Sprintf("max=%d", f.MaxLength))
	}

	if len(rules) == 0 {
		return ""
	}

	return fmt.Sprintf("validate:%q", strings.Join(rules, ","))
}

// Entity is a table reduced to everything the templates need to render a
// model, a payload and a resource.
type Entity struct {
	// Module is the business module the entity belongs to, possibly nested.
	Module string
	// Table is the physical table name.
	Table string
	// Alias is the table's query alias.
	Alias string
	// Name is the Go type name.
	Name string
	// Snake is the entity's snake_case name, used for file and resource names.
	Snake string
	// Comment is the table comment, rendered as the model's doc comment.
	Comment string
	// AuditEmbed is the framework audit mixin the table's columns matched,
	// empty when none did.
	AuditEmbed string
	// HasIDField reports whether the model ends up with a single string ID
	// field, which is what decides if the create/update payload carries one.
	HasIDField bool
	// KeylessTable reports that the table declares no primary key, so nothing
	// the generator writes can give crud one.
	KeylessTable bool
	// Fields are the business columns, audit columns excluded.
	Fields []Field
	// ResourceName is the api.NewRPCResource name.
	ResourceName string
	// Ops are the CRUD operations the resource embeds.
	Ops []Operation
	// AuditUserModel is the qualified WithAuditUserNames argument, empty when
	// the project declares none.
	AuditUserModel string
	// AuditUserModelImport is the package AuditUserModel is selected from.
	AuditUserModelImport string
	// AuditUserModelAlias is the local name that import takes, empty when the
	// package name does not collide with the entity's own model package.
	AuditUserModelAlias string
	// UnmappedColumns names columns whose declared type this tool did not
	// recognize and therefore typed as string.
	UnmappedColumns []string
}

// SearchFields are the fields a generated search payload carries.
func (e Entity) SearchFields() []Field {
	fields := make([]Field, 0, len(e.Fields))

	for _, field := range e.Fields {
		if field.Search != "" {
			fields = append(fields, field)
		}
	}

	return fields
}

// BuildEntity reduces an inspected table into the entity the templates render.
func BuildEntity(table *schema.TableSchema, opts EntityOptions) Entity {
	mixin, consumed := matchAuditMixin(table)
	primaryKey := primaryKeyColumns(table)

	entity := Entity{
		Module:       opts.Module,
		Table:        table.Name,
		Alias:        opts.Alias,
		Snake:        opts.Entity,
		Name:         naming.PascalCase(opts.Entity),
		Comment:      strings.TrimSpace(table.Comment),
		AuditEmbed:   mixin.embed,
		HasIDField:   mixin.declaresID,
		KeylessTable: primaryKey.Size() == 0,
	}

	if entity.Alias == "" {
		entity.Alias = naming.TableAlias(table.Name)
	}

	for _, column := range table.Columns {
		if consumed.Contains(column.Name) {
			continue
		}

		field, known := buildField(column, opts)
		if !known {
			entity.UnmappedColumns = append(entity.UnmappedColumns, column.Name+" "+column.Type)
		}

		// A key column the mixin did not supply has to declare itself, or the
		// model reaches bun with no primary key and every Update and Delete
		// fails at runtime.
		field.PrimaryKey = primaryKey.Contains(column.Name)

		entity.Fields = append(entity.Fields, field)
	}

	return entity
}

// primaryKeyColumns returns the table's primary key column names.
func primaryKeyColumns(table *schema.TableSchema) collections.Set[string] {
	if table.PrimaryKey == nil {
		return collections.NewHashSet[string]()
	}

	return collections.NewHashSetFrom(table.PrimaryKey.Columns...)
}

// EntityOptions carry the caller's choices into entity construction.
type EntityOptions struct {
	// Module is the business module the entity is generated into.
	Module string
	// Entity is the snake_case entity name.
	Entity string
	// Alias overrides the derived table alias.
	Alias string
	// Search decides which columns become search criteria; nil means every
	// scalar column does.
	Search map[string]searchOperator
}

// buildField converts one inspected column into a generated field.
func buildField(column schema.Column, opts EntityOptions) (Field, bool) {
	typ, maxLength, known := resolveGoType(column.Type)

	// The inspected bound wins over anything parsed out of the type name: only
	// some dialects spell the length into the declaration, and where they do
	// not the parsed value is zero rather than wrong.
	if typ.bounded && column.MaxLength > 0 {
		maxLength = column.MaxLength
	}

	field := Field{
		Column:     column.Name,
		Name:       naming.PascalCase(column.Name),
		JSONName:   naming.CamelCase(column.Name),
		Type:       typ.name,
		Nullable:   column.Nullable,
		Label:      strings.TrimSpace(column.Comment),
		MaxLength:  maxLength,
		ImportPath: typ.importPath,
	}

	if field.Label == "" {
		field.Label = field.Name
	}

	if column.Nullable {
		field.Type = "*" + field.Type
	}

	field.Search = searchFor(field, typ, opts.Search)

	return field, known
}

// searchFor decides the search operator a field carries.
//
// With no explicit selection every scalar column becomes a criterion — a
// string matched by substring, everything else by equality. Deleting the
// criteria a screen does not need is a smaller edit than discovering which
// columns exist and adding them back, and the alternative heuristic (only
// indexed columns) produces nothing at all on the tables this is generated
// against most.
func searchFor(field Field, typ goType, selected map[string]searchOperator) searchOperator {
	if selected != nil {
		return selected[field.Column]
	}

	switch typ {
	case goJSON, goBytes:
		return ""
	case goString:
		return searchContains
	default:
		return searchEquals
	}
}

// matchAuditMixin returns the widest audit mixin whose declared columns the
// table actually has, by name and by Go type, along with the column names that
// mixin covers.
func matchAuditMixin(table *schema.TableSchema) (auditMixin, collections.Set[string]) {
	present := make(map[string]goType, len(table.Columns))
	for _, column := range table.Columns {
		typ, _, _ := resolveGoType(column.Type)
		present[column.Name] = typ
	}

	for _, candidate := range auditMixins {
		if !describes(present, candidate.columns) {
			continue
		}

		covered := collections.NewHashSet[string]()
		for _, column := range candidate.columns {
			covered.Add(column.name)
		}

		return candidate, covered
	}

	return auditMixin{}, collections.NewHashSet[string]()
}

// describes reports whether every column a mixin declares exists in the table
// with the type the mixin gives it.
func describes(present map[string]goType, declared []mixinColumn) bool {
	for _, column := range declared {
		if present[column.name] != column.typ {
			return false
		}
	}

	return true
}

func nonEmpty(values []string) []string {
	out := make([]string, 0, len(values))

	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}

	return out
}
