package project

import (
	"errors"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

var (
	// ErrNotAGoModule reports that no go.mod was found in the starting directory
	// or any of its parents, so there is no project to generate into.
	ErrNotAGoModule = errors.New("no go.mod found in this directory or any parent")
	// ErrInvalidModule reports a business module path the generators could not
	// build valid Go from.
	ErrInvalidModule = errors.New("invalid module")
)

// CleanModule normalizes a business module path and rejects one that cannot
// become Go.
//
// Every segment is a directory holding Go packages and the last one is also a
// package name, so each has to be a Go identifier. Without this check a stray
// flag value — an empty string, a trailing slash, a hyphen — is only caught
// when the rendered file fails to parse, which reports a syntax error inside a
// file the generator itself just wrote and reads as a bug in the tool.
func CleanModule(module string) (string, error) {
	module = strings.Trim(strings.TrimSpace(filepath.ToSlash(module)), "/")
	if module == "" {
		return "", fmt.Errorf("%w: a module name is required", ErrInvalidModule)
	}

	for segment := range strings.SplitSeq(module, "/") {
		if !token.IsIdentifier(segment) {
			return "", fmt.Errorf("%w: %q is not usable as a Go package name", ErrInvalidModule, segment)
		}
	}

	return module, nil
}

// Project is the resolved context every generator runs in: where the project
// lives, what its Go module is called, and the conventions it generates under.
type Project struct {
	// Root is the absolute path of the directory holding go.mod.
	Root string
	// ModulePath is the go.mod module path, the prefix of every import the
	// generators emit.
	ModulePath string
	// Config holds vef.yml with defaults applied.
	Config Config
}

// Locate walks up from start looking for go.mod and loads the project's
// generation config. start may be relative; the resolved Root is absolute.
func Locate(start string) (*Project, error) {
	root, err := findModuleRoot(start)
	if err != nil {
		return nil, err
	}

	modulePath, err := readModulePath(root)
	if err != nil {
		return nil, err
	}

	cfg, err := LoadConfig(root)
	if err != nil {
		return nil, err
	}

	return &Project{Root: root, ModulePath: modulePath, Config: cfg}, nil
}

// ModuleDir returns the absolute directory of a business module. The module
// may be nested (hr/emp), which is how the framework's own consumers group a
// large domain into sub-modules.
func (p *Project) ModuleDir(module string) string {
	return filepath.Join(p.Root, filepath.FromSlash(p.Config.ModuleRoot), filepath.FromSlash(module))
}

// PackageDir returns the absolute directory of one package inside a module,
// for example the model or resource package.
func (p *Project) PackageDir(module, pkg string) string {
	return filepath.Join(p.ModuleDir(module), pkg)
}

// ImportPath returns the Go import path of one package inside a module.
//
// Empty segments are dropped rather than joined: a project whose modules live
// at its root has no module root to name, and an import path may not carry an
// empty element.
func (p *Project) ImportPath(module, pkg string) string {
	parts := make([]string, 0, 4)

	for _, part := range []string{p.ModulePath, p.Config.ModuleRoot, module, pkg} {
		if part != "" {
			parts = append(parts, part)
		}
	}

	return strings.Join(parts, "/")
}

// ModuleFile returns the absolute path of a module's module.go, the file both
// resource and service registration is appended to.
func (p *Project) ModuleFile(module string) string {
	return filepath.Join(p.ModuleDir(module), "module.go")
}

// ModulePackageName returns the Go package name of a module's own package,
// which is the last segment of a possibly nested module path.
func (*Project) ModulePackageName(module string) string {
	segments := strings.Split(module, "/")

	return segments[len(segments)-1]
}

// Domain returns the module's top-level segment. Permission tokens are scoped
// to it rather than to the full nested path: hr/emp and hr/ctr both issue
// hr.<entity>.<action>.
func (*Project) Domain(module string) string {
	module, _, _ = strings.Cut(module, "/")

	return module
}

// Rel renders an absolute path inside the project relative to its root, for
// readable command output. A path outside the project is returned unchanged.
func (p *Project) Rel(path string) string {
	rel, err := filepath.Rel(p.Root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}

	return filepath.ToSlash(rel)
}

// ResourceName renders the API resource name for an entity from the configured
// template.
func (p *Project) ResourceName(module, entity string) string {
	return p.expand(p.Config.Resource.Name, module, entity, "")
}

// PermissionToken renders the permission token for one operation from the
// configured template. An empty template means the project does not scope this
// resource by permission.
func (p *Project) PermissionToken(module, entity, action string) string {
	if p.Config.Resource.Permission == "" {
		return ""
	}

	return p.expand(p.Config.Resource.Permission, module, entity, action)
}

// expand substitutes the template placeholders shared by the resource-name and
// permission-token patterns.
func (p *Project) expand(template, module, entity, action string) string {
	return strings.NewReplacer(
		"{module}", module,
		"{domain}", p.Domain(module),
		"{entity}", entity,
		"{action}", action,
	).Replace(template)
}

// findModuleRoot walks up from start until it finds a directory holding go.mod.
func findModuleRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", start, err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("%w: started at %s", ErrNotAGoModule, start)
		}

		dir = parent
	}
}

// readModulePath extracts the module path declared by the project's go.mod.
func readModulePath(root string) (string, error) {
	path := filepath.Join(root, "go.mod")

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}

	modulePath := modfile.ModulePath(data)
	if modulePath == "" {
		return "", fmt.Errorf("%w: %s declares no module path", ErrNotAGoModule, path)
	}

	return modulePath, nil
}
