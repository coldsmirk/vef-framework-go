package scaffold

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/internal/gopatch"
	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/internal/naming"
	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/internal/project"
	"github.com/coldsmirk/vef-framework-go/schema"
)

// frameworkModule is the framework's own module path, the prefix separating
// its imports from the project's in a generated file.
const frameworkModule = "github.com/coldsmirk/vef-framework-go"

// ResourceRequest is one `new resource` invocation.
type ResourceRequest struct {
	// Table is the physical table to derive the entity from.
	Table string
	// Module is the business module to generate into.
	Module string
	// Entity overrides the entity name derived from the table.
	Entity string
	// Alias overrides the derived table alias.
	Alias string
	// Ops overrides the project's configured operation set.
	Ops []string
	// Search overrides which columns become search criteria; nil applies the
	// default of every scalar column.
	Search map[string]searchOperator
	// ConfigPath overrides where application.toml is read from.
	ConfigPath string
	// Source names the data source to inspect; empty means primary.
	Source string
	// Force overwrites generated files that already exist.
	Force bool
}

// GenerateResource derives a model, a payload and an API resource from a
// database table, and registers them in the module's registries.
func GenerateResource(ctx context.Context, proj *project.Project, req ResourceRequest, plan *Plan) error {
	inspector, err := proj.Inspect(req.ConfigPath, req.Source)
	if err != nil {
		return err
	}

	defer func() { _ = inspector.Close() }()

	table, err := inspector.Service.GetTableSchema(ctx, req.Table)
	if err != nil {
		return fmt.Errorf("inspect table %s: %w", req.Table, err)
	}

	entity, err := buildResourceEntity(proj, req, table)
	if err != nil {
		return err
	}

	if err := planEntityFiles(proj, entity, plan, req.Force); err != nil {
		return err
	}

	reportEntityCaveats(entity, plan)

	return planEntityRegistration(proj, entity, plan)
}

// reportEntityCaveats surfaces what the generator could not derive from the
// table. Both cases produce code that compiles and is wrong in a way the
// compiler cannot see, so staying silent about them would be the worst of the
// available behaviors.
func reportEntityCaveats(entity Entity, plan *Plan) {
	for _, column := range entity.UnmappedColumns {
		plan.Note(fmt.Sprintf("column %s has an unrecognized type and was generated as string — correct it by hand", column))
	}

	if entity.KeylessTable {
		plan.Note(fmt.Sprintf("table %s declares no primary key, so update and delete will fail at runtime", entity.Table))
	}
}

// buildResourceEntity assembles the entity together with the project-level
// conventions the resource template needs.
func buildResourceEntity(proj *project.Project, req ResourceRequest, table *schema.TableSchema) (Entity, error) {
	entityName := req.Entity
	if entityName == "" {
		entityName = naming.EntityFromTable(table.Name, proj.Domain(req.Module))
	}

	entity := BuildEntity(table, EntityOptions{
		Module: req.Module,
		Entity: entityName,
		Alias:  req.Alias,
		Search: req.Search,
	})

	entity.ResourceName = proj.ResourceName(req.Module, entity.Snake)
	resolveAuditUserModel(proj, &entity)

	ops := req.Ops
	if len(ops) == 0 {
		ops = proj.Config.Resource.Ops
	}

	renderer := operationRenderer{
		model:          "model." + entity.Name,
		search:         "payload." + entity.Name + "Search",
		params:         "payload." + entity.Name + "Params",
		permission:     func(action string) string { return proj.PermissionToken(req.Module, entity.Snake, action) },
		auditUserModel: entity.AuditUserModel,
		audit:          proj.Config.Resource.Audit,
	}

	for _, name := range ops {
		op, err := renderer.render(name)
		if err != nil {
			return Entity{}, err
		}

		entity.Ops = append(entity.Ops, op)
	}

	return entity, nil
}

// entityPackages are the three packages one entity is spread across, in the
// order the plan reports them.
var entityPackages = []struct {
	pkg      string
	template string
}{
	{"model", "model.go.tmpl"},
	{"payload", "payload.go.tmpl"},
	{"resource", "resource.go.tmpl"},
}

// renderEntityFiles renders an entity's three source files, keyed by the
// package each belongs to.
func renderEntityFiles(proj *project.Project, entity Entity) (map[string][]byte, error) {
	groups := gopatch.ImportGroups{Framework: frameworkModule, Local: proj.ModulePath}

	imports := map[string][]gopatch.Import{
		"model":    modelImports(entity),
		"payload":  payloadImports(entity),
		"resource": resourceImports(proj, entity),
	}

	rendered := make(map[string][]byte, len(entityPackages))

	for _, target := range entityPackages {
		source, err := render(target.template, map[string]any{"Entity": entity}, imports[target.pkg], groups)
		if err != nil {
			return nil, err
		}

		rendered[target.pkg] = source
	}

	return rendered, nil
}

// planEntityFiles renders the three generated files into the plan.
func planEntityFiles(proj *project.Project, entity Entity, plan *Plan, force bool) error {
	rendered, err := renderEntityFiles(proj, entity)
	if err != nil {
		return err
	}

	file := naming.FileName(entity.Snake)
	for _, target := range entityPackages {
		plan.AddFile(filepath.Join(proj.PackageDir(entity.Module, target.pkg), file), rendered[target.pkg], force)
	}

	return nil
}

// planEntityRegistration wires the entity into the module's two registries:
// the model registry variable block and the module's fx option list. Missing
// either one produces an application that compiles and answers 404, which is
// why the generator owns both rather than printing a reminder.
func planEntityRegistration(proj *project.Project, entity Entity, plan *Plan) error {
	models := filepath.Join(proj.PackageDir(entity.Module, "model"), "models.go")
	if err := plan.AddPatchOrCreate(models, "package model\n", func(file *gopatch.File) (bool, error) {
		return file.AppendVarSpec(entity.Name+"Model", "new("+entity.Name+")")
	}); err != nil {
		return err
	}

	constructor := fmt.Sprintf("vef.ProvideAPIResource(resource.New%sResource)", entity.Name)

	return planModuleRegistration(proj, entity.Module, plan, "resource", constructor)
}

// planModuleRegistration appends one fx option to a module's option list,
// creating the module's skeleton when it does not exist yet so a generator can
// open a new business module in the same command that fills it. A module that
// had to be created is also wired into the application's entry point.
func planModuleRegistration(proj *project.Project, module string, plan *Plan, pkg, option string) error {
	groups := gopatch.ImportGroups{Framework: frameworkModule, Local: proj.ModulePath}
	moduleFile := proj.ModuleFile(module)
	_, created := os.Stat(moduleFile)

	err := plan.AddPatchOrCreate(moduleFile, moduleSkeleton(proj, module), func(file *gopatch.File) (bool, error) {
		// Every step's flag counts. Reporting only the last one lets a run that
		// added an import but no option be recorded as "already registered",
		// and AddPatchOrCreate discards the buffer of a patch that changed
		// nothing — so the import would be silently dropped.
		var changed bool

		for _, path := range []string{frameworkModule, proj.ImportPath(module, pkg)} {
			added, err := file.EnsureImport(path, groups)
			if err != nil {
				return false, err
			}

			changed = changed || added
		}

		appended, err := file.AppendCallArg("Module", option)

		return changed || appended, err
	})
	if err != nil || created == nil {
		return err
	}

	return planEntryPointRegistration(proj, module, plan)
}

// planEntryPointRegistration adds a freshly created module to the application's
// vef.Run call.
//
// Without it a new module compiles, registers nothing, and answers 404 — the
// same failure the other two registrations exist to prevent, one level up. It
// is best-effort on purpose: the entry point's location and shape are a
// convention, not a contract, so a project that wires itself differently gets a
// told-you-what-to-add note instead of a failed command.
func planEntryPointRegistration(proj *project.Project, module string, plan *Plan) error {
	entryPoint := filepath.Join(proj.Root, "cmd", "server", "main.go")
	groups := gopatch.ImportGroups{Framework: frameworkModule, Local: proj.ModulePath}
	pkg := proj.ModulePackageName(module)

	file, err := gopatch.Open(entryPoint)

	switch {
	case errors.Is(err, os.ErrNotExist):
		// No entry point where the convention puts one: the project wires
		// itself somewhere this generator should not go guessing.
		plan.Note(fmt.Sprintf("add %s.Module to your application's vef.Run call", pkg))

		return nil
	case err != nil:
		return err
	}

	appended, err := file.AppendCallArgIn("main", "vef.Run", pkg+".Module")
	if err != nil {
		return err
	}

	if !appended {
		plan.Note(fmt.Sprintf("add %s.Module to your application's vef.Run call", pkg))

		return nil
	}

	if _, err := file.EnsureImport(proj.ImportPath(module, ""), groups); err != nil {
		return err
	}

	if err := file.Format(); err != nil {
		return err
	}

	plan.AddContent(entryPoint, file.Source())

	return nil
}

// moduleSkeleton is the module.go a new business module starts from.
func moduleSkeleton(proj *project.Project, module string) string {
	return fmt.Sprintf(`//go:generate vef-cli generate-model-schema -i ./model -o ./schema -p schema
package %s

var Module = vef.Module(
	%q,
)
`, proj.ModulePackageName(module), "app:"+strings.ReplaceAll(module, "/", ":"))
}

func modelImports(entity Entity) []gopatch.Import {
	return plainImports(append([]string{frameworkModule + "/orm"}, fieldImports(entity)...))
}

func payloadImports(entity Entity) []gopatch.Import {
	return plainImports(append([]string{frameworkModule + "/api"}, fieldImports(entity)...))
}

func resourceImports(proj *project.Project, entity Entity) []gopatch.Import {
	imports := plainImports([]string{
		frameworkModule + "/api",
		frameworkModule + "/crud",
		proj.ImportPath(entity.Module, "model"),
		proj.ImportPath(entity.Module, "payload"),
	})

	if entity.AuditUserModelImport != "" {
		imports = append(imports, gopatch.Import{Alias: entity.AuditUserModelAlias, Path: entity.AuditUserModelImport})
	}

	return imports
}

// fieldImports collects the packages the entity's field types are spelled from.
func fieldImports(entity Entity) []string {
	paths := collections.NewHashSet[string]()
	for _, field := range entity.Fields {
		if field.ImportPath != "" {
			paths.Add(field.ImportPath)
		}
	}

	return paths.ToSlice()
}

func plainImports(paths []string) []gopatch.Import {
	imports := make([]gopatch.Import, 0, len(paths))
	for _, path := range paths {
		imports = append(imports, gopatch.Import{Path: path})
	}

	return imports
}

// resolveAuditUserModel turns the project's configured WithAuditUserNames
// reference into the selector the resource writes and the import that backs it.
//
// The reference nearly always points at another module's model package, which
// collides with the entity's own — so the import is aliased with the owning
// module's name (acme/internal/sys/model becomes sysmodel), the same shape a
// developer writes by hand.
func resolveAuditUserModel(proj *project.Project, entity *Entity) {
	qualified := strings.TrimSpace(proj.Config.Resource.AuditUserModel)

	dot := strings.LastIndexByte(qualified, '.')
	if dot <= 0 || dot == len(qualified)-1 {
		return
	}

	importPath, identifier := qualified[:dot], qualified[dot+1:]
	pkg := lastSegment(importPath)

	alias := ""
	if pkg == lastSegment(proj.ImportPath(entity.Module, "model")) && importPath != proj.ImportPath(entity.Module, "model") {
		alias = lastSegment(trimLastSegment(importPath)) + pkg
	}

	entity.AuditUserModelImport = importPath
	entity.AuditUserModelAlias = alias

	if alias != "" {
		pkg = alias
	}

	entity.AuditUserModel = pkg + "." + identifier
}

func lastSegment(path string) string {
	if slash := strings.LastIndexByte(path, '/'); slash >= 0 {
		return path[slash+1:]
	}

	return path
}

func trimLastSegment(path string) string {
	if slash := strings.LastIndexByte(path, '/'); slash >= 0 {
		return path[:slash]
	}

	return path
}
