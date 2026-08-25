package scaffold

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrUnknownOperation reports an operation name the generator cannot render.
var ErrUnknownOperation = errors.New("unknown crud operation")

// typeArgs names the generic parameters a crud operation takes.
type typeArgs uint8

const (
	// modelAndSearch is the query shape: crud.FindPage[Model, Search].
	modelAndSearch typeArgs = iota
	// modelAndParams is the mutation shape: crud.Create[Model, Params].
	modelAndParams
	// modelOnly is the by-id shape: crud.Delete[Model].
	modelOnly
)

// operationSpec is one entry of the catalog `new resource` can render.
type operationSpec struct {
	// field is the embedded interface's name, which is also the struct field
	// name because crud embeds are addressed by interface name.
	field string
	// args selects which generic parameters the operation takes.
	args typeArgs
	// permission is the action segment of the permission token, empty for an
	// operation the project leaves open.
	permission string
	// auditUserNames adds WithAuditUserNames, which resolves the audit columns'
	// user ids into names.
	auditUserNames bool
	// audit enables the operation's audit log entry.
	audit bool
}

// operationCatalog is every operation `new resource` knows how to render,
// keyed by the name used on the command line and in vef.yml.
//
// find_tree is deliberately absent: crud.NewFindTree takes a tree-building
// function, which is business code no generator can supply. The permission
// column encodes the framework's convention — a read that returns records is
// gated by .query while an options feed stays open, because gating the feed is
// what produces the empty-dropdown class of bug on a screen the user is
// otherwise allowed to see.
var operationCatalog = map[string]operationSpec{
	"find_all":          {field: "FindAll", args: modelAndSearch, permission: "query", auditUserNames: true},
	"find_page":         {field: "FindPage", args: modelAndSearch, permission: "query", auditUserNames: true},
	"find_one":          {field: "FindOne", args: modelAndSearch, permission: "query", auditUserNames: true},
	"find_options":      {field: "FindOptions", args: modelAndSearch},
	"find_tree_options": {field: "FindTreeOptions", args: modelAndSearch},
	"export":            {field: "Export", args: modelAndSearch, permission: "export", auditUserNames: true},
	"create":            {field: "Create", args: modelAndParams, permission: "create", audit: true},
	"create_many":       {field: "CreateMany", args: modelAndParams, permission: "create", audit: true},
	"update":            {field: "Update", args: modelAndParams, permission: "update", audit: true},
	"update_many":       {field: "UpdateMany", args: modelAndParams, permission: "update", audit: true},
	"delete":            {field: "Delete", args: modelOnly, permission: "delete", audit: true},
	"delete_many":       {field: "DeleteMany", args: modelOnly, permission: "delete", audit: true},
	"import":            {field: "Import", args: modelOnly, permission: "import", audit: true},
}

// Operation is a rendered crud operation, ready for the resource template.
type Operation struct {
	// Field is the struct field and embedded interface name.
	Field string
	// Embed is the full embedded type, such as
	// crud.FindPage[model.Holiday, payload.HolidaySearch].
	Embed string
	// Init is the complete initializer, constructor plus builder chain.
	Init string
}

// KnownOperations lists every renderable operation name, sorted, for help text
// and error messages.
func KnownOperations() []string {
	names := make([]string, 0, len(operationCatalog))
	for name := range operationCatalog {
		names = append(names, name)
	}

	slices.Sort(names)

	return names
}

// operationRenderer holds everything shared across one entity's operations.
type operationRenderer struct {
	model          string
	search         string
	params         string
	permission     func(action string) string
	auditUserModel string
	audit          bool
}

// render builds the embed and initializer for one operation name.
func (r operationRenderer) render(name string) (Operation, error) {
	spec, ok := operationCatalog[name]
	if !ok {
		return Operation{}, fmt.Errorf("%w: %s (known: %s)", ErrUnknownOperation, name, strings.Join(KnownOperations(), ", "))
	}

	args := r.typeArgList(spec.args)
	embed := fmt.Sprintf("crud.%s%s", spec.field, args)

	var chain strings.Builder

	fmt.Fprintf(&chain, "crud.New%s%s()", spec.field, args)

	if spec.permission != "" {
		if token := r.permission(spec.permission); token != "" {
			fmt.Fprintf(&chain, ".RequiredPermission(%q)", token)
		}
	}

	if spec.auditUserNames && r.auditUserModel != "" {
		fmt.Fprintf(&chain, ".WithAuditUserNames(%s)", r.auditUserModel)
	}

	if spec.audit && r.audit {
		chain.WriteString(".EnableAudit()")
	}

	return Operation{Field: spec.field, Embed: embed, Init: chain.String()}, nil
}

func (r operationRenderer) typeArgList(args typeArgs) string {
	switch args {
	case modelAndSearch:
		return "[" + r.model + ", " + r.search + "]"
	case modelAndParams:
		return "[" + r.model + ", " + r.params + "]"
	default:
		return "[" + r.model + "]"
	}
}
