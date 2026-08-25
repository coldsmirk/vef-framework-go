package project

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"golang.org/x/mod/module"
)

// ErrInvalidModuleRoot reports a module_root that cannot form elements of a Go
// import path, so every import the generators emit under it would be malformed.
var ErrInvalidModuleRoot = errors.New("invalid module_root")

// ConfigFileName is the project-level generation config vef-cli reads from the
// module root. Every key has a default, so the file is optional; `new project`
// writes one so the conventions are visible rather than implied.
const ConfigFileName = "vef.yml"

// DefaultOps is the operation set `new resource` generates when the project
// does not narrow it — the combination the framework's CRUD resources use for
// an ordinary master-data entity.
var DefaultOps = []string{"find_page", "find_all", "find_options", "create", "update", "delete"}

// Config is vef.yml with defaults applied.
type Config struct {
	// ModuleRoot is the directory business modules live under, relative to the
	// project root.
	ModuleRoot string `mapstructure:"module_root"`
	// Resource holds the conventions `new resource` generates against.
	Resource ResourceConfig `mapstructure:"resource"`
}

// ResourceConfig captures the project's API resource conventions. Name and
// Permission are templates over {module} (the full, possibly nested module
// path), {domain} (its first segment), {entity} and {action}.
type ResourceConfig struct {
	// Name is the api.NewRPCResource name template.
	Name string `mapstructure:"name"`
	// Permission is the RequiredPermission token template. Empty generates no
	// permission at all, which suits a project that authorizes elsewhere.
	Permission string `mapstructure:"permission"`
	// Ops lists the CRUD operations a generated resource embeds.
	Ops []string `mapstructure:"ops"`
	// Audit enables EnableAudit on the generated mutating operations.
	Audit bool `mapstructure:"audit"`
	// AuditUserModel is the qualified model variable passed to
	// WithAuditUserNames on read operations, written as
	// <import path>.<identifier> (for example acme/internal/sys/model.UserModel).
	// Empty generates no WithAuditUserNames call.
	AuditUserModel string `mapstructure:"audit_user_model"`
}

// DefaultConfig returns the configuration a project with no vef.yml generates
// under.
func DefaultConfig() Config {
	return Config{
		ModuleRoot: "internal",
		Resource: ResourceConfig{
			Name:       "{module}/{entity}",
			Permission: "{domain}.{entity}.{action}",
			Ops:        DefaultOps,
			Audit:      true,
		},
	}
}

// LoadConfig reads root/vef.yml over the defaults. A missing file is not an
// error; a malformed one is, because silently generating against defaults the
// project deliberately overrode would produce code that looks right and is
// wrong everywhere.
func LoadConfig(root string) (Config, error) {
	cfg := DefaultConfig()

	v := viper.New()
	v.SetConfigFile(filepath.Join(root, ConfigFileName))
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) || isNotExist(err) {
			return cfg, nil
		}

		return Config{}, fmt.Errorf("read %s: %w", ConfigFileName, err)
	}

	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", ConfigFileName, err)
	}

	return normalize(cfg)
}

// normalize restores any value the file blanked out to its default, so a
// partially written config cannot generate a resource with no name template or
// no operations.
func normalize(cfg Config) (Config, error) {
	defaults := DefaultConfig()

	if strings.TrimSpace(cfg.ModuleRoot) == "" {
		cfg.ModuleRoot = defaults.ModuleRoot
	}

	if strings.TrimSpace(cfg.Resource.Name) == "" {
		cfg.Resource.Name = defaults.Resource.Name
	}

	if len(cfg.Resource.Ops) == 0 {
		cfg.Resource.Ops = defaults.Resource.Ops
	}

	root, err := cleanModuleRoot(cfg.ModuleRoot)
	if err != nil {
		return Config{}, err
	}

	cfg.ModuleRoot = root

	return cfg, nil
}

// cleanModuleRoot normalizes module_root into a form both of its consumers can
// use. It is a directory, so it is cleaned like a path; it is also spliced into
// every generated import, where the rules are stricter than the filesystem's.
// A dot segment is the case that matters: filepath.Join swallows it, so
// module_root: "." produced the right directories and the import path
// "acme/./md/model", which the compiler rejects. It normalizes to the empty
// string — modules living at the project root — and ImportPath skips it.
func cleanModuleRoot(value string) (string, error) {
	value = strings.Trim(path.Clean(filepath.ToSlash(strings.TrimSpace(value))), "/")
	if value == "" || value == "." {
		return "", nil
	}

	// The synthetic prefix makes this a whole import path, which is what the
	// checker validates; the elements after it are the ones being judged.
	if err := module.CheckImportPath("m/" + value); err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidModuleRoot, err)
	}

	return value, nil
}

// isNotExist reports whether a viper read failure is just a missing file.
// viper surfaces an explicitly set config file's absence as a *fs.PathError
// rather than as ConfigFileNotFoundError, which only covers search-path lookups.
func isNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}
