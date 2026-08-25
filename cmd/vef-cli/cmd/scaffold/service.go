package scaffold

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/internal/gopatch"
	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/internal/naming"
	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/internal/project"
)

var (
	// ErrInvalidDependency reports a malformed --deps entry.
	ErrInvalidDependency = errors.New("invalid dependency, expected field:Type")
	// ErrServiceNameRequired reports an empty service name.
	ErrServiceNameRequired = errors.New("service name is required")
)

// knownDependencyImports maps the package qualifier of a dependency type to
// the framework package it comes from, so `--deps bus:event.Bus` needs no
// second flag to say where event lives. A qualifier outside this set is left
// to the developer's own import, which the compiler will ask for.
var knownDependencyImports = map[string]string{
	"api":         frameworkModule + "/api",
	"cache":       frameworkModule + "/cache",
	"cron":        frameworkModule + "/cron",
	"datasource":  frameworkModule + "/datasource",
	"event":       frameworkModule + "/event",
	"httpx":       frameworkModule + "/httpx",
	"integration": frameworkModule + "/integration",
	"js":          frameworkModule + "/js",
	"lock":        frameworkModule + "/lock",
	"logx":        frameworkModule + "/logx",
	"mold":        frameworkModule + "/mold",
	"orm":         frameworkModule + "/orm",
	"push":        frameworkModule + "/push",
	"schema":      frameworkModule + "/schema",
	"security":    frameworkModule + "/security",
	"sequence":    frameworkModule + "/sequence",
	"storage":     frameworkModule + "/storage",
}

// Dependency is one constructor-injected field of a generated service.
type Dependency struct {
	// Field is the struct field name.
	Field string
	// Type is the field's type as written in Go source.
	Type string
}

// ServiceRequest is one `new service` invocation.
type ServiceRequest struct {
	// Name is the service name, with or without the Service suffix.
	Name string
	// Module is the business module to generate into.
	Module string
	// Deps are the constructor-injected dependencies.
	Deps []Dependency
	// Force overwrites the service file if it already exists.
	Force bool
}

// serviceView is what the service template renders from.
type serviceView struct {
	// Type is the generated struct's name, always suffixed with Service.
	Type string
	// Deps are the injected fields.
	Deps []Dependency
	// Params renders the constructor's parameter list.
	Params string
	// Literal renders the composite literal's field assignments.
	Literal string
}

// GenerateService writes a service skeleton and registers it with its module.
func GenerateService(proj *project.Project, req ServiceRequest, plan *Plan) error {
	typeName, err := serviceTypeName(req.Name)
	if err != nil {
		return err
	}

	view := serviceView{Type: typeName, Deps: req.Deps}

	params := make([]string, 0, len(req.Deps))
	assignments := make([]string, 0, len(req.Deps))

	for _, dep := range req.Deps {
		params = append(params, dep.Field+" "+dep.Type)
		assignments = append(assignments, dep.Field+": "+dep.Field)
	}

	view.Params = strings.Join(params, ", ")
	view.Literal = strings.Join(assignments, ", ")

	groups := gopatch.ImportGroups{Framework: frameworkModule, Local: proj.ModulePath}

	source, err := render("service.go.tmpl", view, dependencyImports(req.Deps), groups)
	if err != nil {
		return err
	}

	file := naming.FileName(naming.SnakeCase(strings.TrimSuffix(typeName, "Service")))
	plan.AddFile(filepath.Join(proj.PackageDir(req.Module, "service"), file), source, req.Force)

	return planModuleRegistration(proj, req.Module, plan, "service", fmt.Sprintf("vef.Provide(service.New%s)", typeName))
}

// serviceTypeName normalizes a requested name into the generated type,
// accepting both Notification and NotificationService.
func serviceTypeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrServiceNameRequired
	}

	// A snake_case or kebab-case name is accepted too, since that is how the
	// entity generator is addressed and the two commands sit side by side.
	if strings.ContainsAny(name, "_- ") {
		name = naming.PascalCase(name)
	}

	return strings.TrimSuffix(name, "Service") + "Service", nil
}

// dependencyImports resolves the framework packages a dependency list needs.
func dependencyImports(deps []Dependency) []gopatch.Import {
	paths := make([]string, 0, len(deps))

	for _, dep := range deps {
		qualifier, _, found := strings.Cut(strings.TrimLeft(dep.Type, "*[]"), ".")
		if !found {
			continue
		}

		if path, known := knownDependencyImports[qualifier]; known {
			paths = append(paths, path)
		}
	}

	return plainImports(paths)
}

// parseDependencies turns --deps entries into constructor dependencies.
func parseDependencies(entries []string) ([]Dependency, error) {
	deps := make([]Dependency, 0, len(entries))

	for _, entry := range entries {
		field, typ, found := strings.Cut(entry, ":")
		field, typ = strings.TrimSpace(field), strings.TrimSpace(typ)

		if !found || field == "" || typ == "" {
			return nil, fmt.Errorf("%w: %s", ErrInvalidDependency, entry)
		}

		deps = append(deps, Dependency{Field: field, Type: typ})
	}

	return deps, nil
}
