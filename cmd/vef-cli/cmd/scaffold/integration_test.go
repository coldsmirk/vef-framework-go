package scaffold

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/muesli/termenv"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/internal/project"
	"github.com/coldsmirk/vef-framework-go/internal/database"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

// createTableDDL is the shape a generator must handle end to end: the
// framework's audit columns, a NOT NULL bounded string, nullable columns of
// three different kinds, and comments on both the table and every column.
const createTableDDL = `
CREATE TABLE md_allowance_item (
	id          VARCHAR(20) PRIMARY KEY,
	created_at  TIMESTAMPTZ NOT NULL,
	created_by  VARCHAR(20),
	updated_at  TIMESTAMPTZ,
	updated_by  VARCHAR(20),
	name        VARCHAR(128) NOT NULL,
	scope       VARCHAR(64),
	is_active   BOOLEAN NOT NULL,
	amount      NUMERIC(12,2),
	remark      VARCHAR(256)
);
COMMENT ON TABLE md_allowance_item IS '补贴项目';
COMMENT ON COLUMN md_allowance_item.name IS '名称';
COMMENT ON COLUMN md_allowance_item.scope IS '适用范围';
COMMENT ON COLUMN md_allowance_item.is_active IS '是否启用';
COMMENT ON COLUMN md_allowance_item.amount IS '金额';
COMMENT ON COLUMN md_allowance_item.remark IS '备注';
`

// writeProject lays out a minimal project on disk for the generators to target
// and returns its resolved context.
func writeProject(t *testing.T, dataSource string) *project.Project {
	t.Helper()

	root := t.TempDir()

	files := map[string]string{
		"go.mod":                      "module acme\n\ngo 1.26\n",
		"vef.yml":                     "resource:\n  name: \"smp/{module}/{entity}\"\n",
		"configs/application.toml":    dataSource,
		"internal/md/module.go":       "package md\n\nvar Module = vef.Module(\n\t\"app:md\",\n)\n",
		"internal/md/model/models.go": "package model\n\nvar (\n)\n",
	}

	for path, content := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755), "creating %s must succeed", path)
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644), "writing %s must succeed", path)
	}

	proj, err := project.Locate(root)
	require.NoError(t, err, "the written project must resolve")

	return proj
}

// dataSourceTOML renders an application.toml pointing at the container.
func dataSourceTOML(container *testx.PostgresContainer) string {
	ds := container.DataSource

	return fmt.Sprintf(`[vef.data_sources.primary]
type = "postgres"
host = %q
port = %d
user = %q
password = %q
database = %q
`, ds.Host, ds.Port, ds.User, ds.Password, ds.Database)
}

func TestGenerateResourceAgainstPostgres(t *testing.T) {
	ctx := context.Background()
	container := testx.NewPostgresContainer(ctx, t)

	db, err := database.Open(*container.DataSource)
	require.NoError(t, err, "connecting to the container must succeed")

	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(ctx, createTableDDL)
	require.NoError(t, err, "creating the fixture table must succeed")

	proj := writeProject(t, dataSourceTOML(container))
	plan := NewPlan(proj)

	require.NoError(t, GenerateResource(ctx, proj, ResourceRequest{
		Table:  "md_allowance_item",
		Module: "md",
		Ops:    []string{"find_page", "create", "delete"},
	}, plan), "generating from a real table must succeed")

	require.NoError(t, plan.Apply(os.Stdout, termenv.DefaultOutput()), "applying the plan must succeed")

	t.Run("WritesTheModelWithTheAuditMixin", func(t *testing.T) {
		model := readFile(t, proj, "internal/md/model/allowance_item.go")

		require.Contains(t, model, "orm.BaseModel `bun:\"table:md_allowance_item,alias:mai\"`", "the bun tag must carry the table and derived alias")
		require.Contains(t, model, "orm.FullAuditedModel", "the audit columns must resolve to the framework mixin")
		require.NotContains(t, model, "CreatedAt", "an audit column must not be re-declared as a field")
		require.Contains(t, model, "// AllowanceItem 补贴项目.", "the table comment must become the doc comment")
		// A live PostgreSQL varchar(128) NOT NULL must round-trip into the
		// framework's full tag set — the bound included, which Atlas reports
		// separately from the type name.
		require.Contains(t, model, "Name     string   `json:\"name\" validate:\"required,max=128\" label:\"名称\"`",
			"the declared bound must reach the validate tag")
		require.Contains(t, model, "Amount   *float64", "a nullable numeric must be a float64 pointer")
	})

	t.Run("WritesThePayload", func(t *testing.T) {
		payload := readFile(t, proj, "internal/md/payload/allowance_item.go")

		require.Contains(t, payload, "search:\"contains\"", "string criteria must default to substring matching")
		require.Contains(t, payload, "search:\"eq\"", "non-string criteria must default to equality")
		require.Contains(t, payload, "ID       string   `json:\"id\"`", "an id-bearing entity's params must carry the id update needs")
	})

	t.Run("WritesTheResource", func(t *testing.T) {
		resource := readFile(t, proj, "internal/md/resource/allowance_item.go")

		require.Contains(t, resource, `api.NewRPCResource("smp/md/allowance_item")`, "the resource name must follow the project template")
		require.Contains(t, resource, `RequiredPermission("md.allowance_item.query")`, "a read must be gated by the query permission")
		require.Contains(t, resource, `RequiredPermission("md.allowance_item.delete")`, "a delete must be gated by its own permission")
		require.Contains(t, resource, ".EnableAudit()", "mutations must be audited")
	})

	t.Run("RegistersInBothRegistries", func(t *testing.T) {
		require.Contains(t, readFile(t, proj, "internal/md/model/models.go"),
			"AllowanceItemModel = new(AllowanceItem)", "the model registry must gain the entity")

		module := readFile(t, proj, "internal/md/module.go")
		require.Contains(t, module, "vef.ProvideAPIResource(resource.NewAllowanceItemResource)", "the module must gain the resource")
		require.Contains(t, module, `"acme/internal/md/resource"`, "the module must gain the import its registration needs")
	})

	t.Run("IsIdempotent", func(t *testing.T) {
		second := NewPlan(proj)

		require.NoError(t, GenerateResource(ctx, proj, ResourceRequest{
			Table:  "md_allowance_item",
			Module: "md",
			Ops:    []string{"find_page", "create", "delete"},
		}, second), "re-generating must succeed")

		require.True(t, second.Empty(), "re-running a generator over its own output must change nothing")
	})
}

func TestGenerateResourceIntoAnAbsentModule(t *testing.T) {
	ctx := context.Background()
	container := testx.NewPostgresContainer(ctx, t)

	db, err := database.Open(*container.DataSource)
	require.NoError(t, err, "connecting to the container must succeed")

	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(ctx, createTableDDL)
	require.NoError(t, err, "creating the fixture table must succeed")

	proj := writeProject(t, dataSourceTOML(container))
	plan := NewPlan(proj)

	// "hr" does not exist in the fixture project: the generator has to open the
	// module, because the files it must register into are the module's own.
	require.NoError(t, GenerateResource(ctx, proj, ResourceRequest{
		Table:  "md_allowance_item",
		Module: "hr",
		Entity: "allowance_item",
		Ops:    []string{"find_page"},
	}, plan), "generating into a module that does not exist yet must succeed")

	require.NoError(t, plan.Apply(os.Stdout, termenv.DefaultOutput()), "applying the plan must succeed")

	module := readFile(t, proj, "internal/hr/module.go")
	require.Contains(t, module, `var Module = vef.Module(`, "the new module must get its fx option list")
	require.Contains(t, module, `"app:hr"`, "the new module must be named after its directory")
	require.Contains(t, module, "vef.ProvideAPIResource(resource.NewAllowanceItemResource)", "the resource must be registered in it")
	require.Contains(t, module, "//go:generate vef-cli generate-model-schema", "the new module must carry the schema generation directive")
}

func TestGenerateServiceRegistersWithItsModule(t *testing.T) {
	proj := writeProject(t, "")
	plan := NewPlan(proj)

	require.NoError(t, GenerateService(proj, ServiceRequest{
		Name:   "Notification",
		Module: "md",
		Deps:   []Dependency{{Field: "bus", Type: "event.Bus"}},
	}, plan), "generating a service must succeed")

	require.NoError(t, plan.Apply(os.Stdout, termenv.DefaultOutput()), "applying the plan must succeed")

	service := readFile(t, proj, "internal/md/service/notification.go")
	require.Contains(t, service, "type NotificationService struct {", "the Service suffix must be added once")
	require.Contains(t, service, "bus event.Bus", "a declared dependency must become a field")
	require.Contains(t, service, `"github.com/coldsmirk/vef-framework-go/event"`, "a framework dependency's import must be resolved")
	require.Contains(t, service, "func NewNotificationService(bus event.Bus) *NotificationService {", "the constructor must take the dependency")

	require.Contains(t, readFile(t, proj, "internal/md/module.go"),
		"vef.Provide(service.NewNotificationService)", "the service must be registered with its module")
}

func TestGenerateServiceWithoutDependencies(t *testing.T) {
	proj := writeProject(t, "")
	plan := NewPlan(proj)

	require.NoError(t, GenerateService(proj, ServiceRequest{Name: "RosterService", Module: "md"}, plan),
		"generating a dependency-free service must succeed")
	require.NoError(t, plan.Apply(os.Stdout, termenv.DefaultOutput()), "applying the plan must succeed")

	service := readFile(t, proj, "internal/md/service/roster.go")
	require.Contains(t, service, "type RosterService struct {\n}", "an empty service must still declare its struct")
	require.Contains(t, service, "return new(RosterService)", "an empty struct pointer must use new(T), matching the repository convention")
	require.NotContains(t, service, "RosterServiceService", "an already-suffixed name must not be suffixed twice")
}

// readFile returns a generated file's contents.
func readFile(t *testing.T, proj *project.Project, relative string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(proj.Root, filepath.FromSlash(relative)))
	require.NoError(t, err, "%s must have been generated", relative)

	return string(content)
}
