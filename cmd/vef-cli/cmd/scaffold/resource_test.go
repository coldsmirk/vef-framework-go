package scaffold

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/internal/project"
	"github.com/coldsmirk/vef-framework-go/schema"
)

// allowanceItemTable mirrors a real master-data table: the framework's full
// audit column set plus six business columns, one of them a nullable numeric.
func allowanceItemTable() *schema.TableSchema {
	return &schema.TableSchema{
		Name:    "md_allowance_item",
		Comment: "补贴项目",
		Columns: []schema.Column{
			{Name: "id", Type: "varchar(20)", MaxLength: 20, IsPrimaryKey: true, Comment: "主键"},
			{Name: "created_at", Type: "timestamptz", Comment: "创建时间"},
			{Name: "created_by", Type: "varchar(20)", MaxLength: 20, Nullable: true, Comment: "创建人ID"},
			{Name: "updated_at", Type: "timestamptz", Nullable: true, Comment: "更新时间"},
			{Name: "updated_by", Type: "varchar(20)", MaxLength: 20, Nullable: true, Comment: "更新人ID"},
			{Name: "name", Type: "varchar(128)", MaxLength: 128, Comment: "名称"},
			{Name: "scope", Type: "varchar(64)", MaxLength: 64, Nullable: true, Comment: "适用范围"},
			{Name: "unit", Type: "varchar(32)", MaxLength: 32, Nullable: true, Comment: "单位"},
			{Name: "is_active", Type: "bool", Comment: "是否启用"},
			{Name: "amount", Type: "numeric(12,2)", Nullable: true, Comment: "金额"},
			{Name: "remark", Type: "varchar(256)", MaxLength: 256, Nullable: true, Comment: "备注"},
		},
	}
}

func testProject() *project.Project {
	cfg := project.DefaultConfig()
	cfg.Resource.Name = "smp/{module}/{entity}"
	cfg.Resource.AuditUserModel = "acme/internal/sys/model.UserModel"

	return &project.Project{Root: "/tmp/acme", ModulePath: "acme", Config: cfg}
}

func buildTestEntity(t *testing.T, req ResourceRequest) Entity {
	t.Helper()

	entity, err := buildResourceEntity(testProject(), req, allowanceItemTable())
	require.NoError(t, err, "building the entity from a well-formed table must succeed")

	return entity
}

func TestBuildResourceEntity(t *testing.T) {
	entity := buildTestEntity(t, ResourceRequest{Table: "md_allowance_item", Module: "md"})

	t.Run("DerivesNames", func(t *testing.T) {
		require.Equal(t, "allowance_item", entity.Snake, "the module's own table prefix must be dropped")
		require.Equal(t, "AllowanceItem", entity.Name, "the Go type name comes from the entity name")
		require.Equal(t, "mai", entity.Alias, "the table alias follows the initials convention")
		require.Equal(t, "smp/md/allowance_item", entity.ResourceName, "the resource name follows the project template")
	})

	t.Run("RecognizesTheAuditMixin", func(t *testing.T) {
		require.Equal(t, "orm.FullAuditedModel", entity.AuditEmbed,
			"a table carrying id plus both audit pairs must match the widest mixin")
		require.Len(t, entity.Fields, 6, "the five audit columns must not be re-declared as fields")
	})

	t.Run("MapsColumnsToFields", func(t *testing.T) {
		byName := map[string]Field{}
		for _, field := range entity.Fields {
			byName[field.Name] = field
		}

		require.Equal(t, "string", byName["Name"].Type, "a NOT NULL varchar is a plain string")
		require.Equal(t, "*string", byName["Scope"].Type, "a nullable column takes a pointer")
		require.Equal(t, "*float64", byName["Amount"].Type, "numeric maps to float64")
		require.Equal(t, "bool", byName["IsActive"].Type, "bool maps straight through")
		require.Equal(t, "isActive", byName["IsActive"].JSONName, "the json name is the camelCase column")
		require.Equal(t, "名称", byName["Name"].Label, "the column comment becomes the label")
	})

	t.Run("ReportsNoUnmappedColumns", func(t *testing.T) {
		require.Empty(t, entity.UnmappedColumns, "every column in this table has a known type")
	})
}

func TestRenderEntityFiles(t *testing.T) {
	entity := buildTestEntity(t, ResourceRequest{
		Table:  "md_allowance_item",
		Module: "md",
		Ops:    []string{"find_page", "find_options", "create", "update", "delete"},
	})

	rendered, err := renderEntityFiles(testProject(), entity)
	require.NoError(t, err, "rendering an entity's files must succeed")

	t.Run("Model", func(t *testing.T) {
		require.Equal(t, `package model

import (
	"github.com/coldsmirk/vef-framework-go/orm"
)

// AllowanceItem 补贴项目.
type AllowanceItem struct {
	orm.BaseModel `+"`"+`bun:"table:md_allowance_item,alias:mai"`+"`"+`
	orm.FullAuditedModel

	Name     string   `+"`"+`json:"name" validate:"required,max=128" label:"名称"`+"`"+`
	Scope    *string  `+"`"+`json:"scope,omitempty" validate:"omitempty,max=64" label:"适用范围"`+"`"+`
	Unit     *string  `+"`"+`json:"unit,omitempty" validate:"omitempty,max=32" label:"单位"`+"`"+`
	IsActive bool     `+"`"+`json:"isActive" label:"是否启用"`+"`"+`
	Amount   *float64 `+"`"+`json:"amount,omitempty" validate:"omitempty" label:"金额"`+"`"+`
	Remark   *string  `+"`"+`json:"remark,omitempty" validate:"omitempty,max=256" label:"备注"`+"`"+`
}
`, string(rendered["model"]), "the generated model must read as hand-written framework code")
	})

	t.Run("Payload", func(t *testing.T) {
		require.Equal(t, `package payload

import (
	"github.com/coldsmirk/vef-framework-go/api"
)

// AllowanceItemSearch 补贴项目查询条件.
type AllowanceItemSearch struct {
	api.P

	Name     *string  `+"`"+`json:"name" search:"contains" label:"名称"`+"`"+`
	Scope    *string  `+"`"+`json:"scope" search:"contains" label:"适用范围"`+"`"+`
	Unit     *string  `+"`"+`json:"unit" search:"contains" label:"单位"`+"`"+`
	IsActive *bool    `+"`"+`json:"isActive" search:"eq" label:"是否启用"`+"`"+`
	Amount   *float64 `+"`"+`json:"amount" search:"eq" label:"金额"`+"`"+`
	Remark   *string  `+"`"+`json:"remark" search:"contains" label:"备注"`+"`"+`
}

// AllowanceItemParams 补贴项目创建和更新参数.
type AllowanceItemParams struct {
	api.P

	ID       string   `+"`"+`json:"id"`+"`"+`
	Name     string   `+"`"+`json:"name" validate:"required,max=128" label:"名称"`+"`"+`
	Scope    *string  `+"`"+`json:"scope" validate:"omitempty,max=64" label:"适用范围"`+"`"+`
	Unit     *string  `+"`"+`json:"unit" validate:"omitempty,max=32" label:"单位"`+"`"+`
	IsActive bool     `+"`"+`json:"isActive" label:"是否启用"`+"`"+`
	Amount   *float64 `+"`"+`json:"amount" validate:"omitempty" label:"金额"`+"`"+`
	Remark   *string  `+"`"+`json:"remark" validate:"omitempty,max=256" label:"备注"`+"`"+`
}
`, string(rendered["payload"]), "the generated payload must carry search tags and an id for update")
	})

	t.Run("Resource", func(t *testing.T) {
		require.Equal(t, `package resource

import (
	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/crud"

	"acme/internal/md/model"
	"acme/internal/md/payload"
	sysmodel "acme/internal/sys/model"
)

// AllowanceItemResource 补贴项目资源.
type AllowanceItemResource struct {
	api.Resource
	crud.FindPage[model.AllowanceItem, payload.AllowanceItemSearch]
	crud.FindOptions[model.AllowanceItem, payload.AllowanceItemSearch]
	crud.Create[model.AllowanceItem, payload.AllowanceItemParams]
	crud.Update[model.AllowanceItem, payload.AllowanceItemParams]
	crud.Delete[model.AllowanceItem]
}

// NewAllowanceItemResource constructs the AllowanceItem API resource.
func NewAllowanceItemResource() api.Resource {
	return &AllowanceItemResource{
		Resource:    api.NewRPCResource("smp/md/allowance_item"),
		FindPage:    crud.NewFindPage[model.AllowanceItem, payload.AllowanceItemSearch]().RequiredPermission("md.allowance_item.query").WithAuditUserNames(sysmodel.UserModel),
		FindOptions: crud.NewFindOptions[model.AllowanceItem, payload.AllowanceItemSearch](),
		Create:      crud.NewCreate[model.AllowanceItem, payload.AllowanceItemParams]().RequiredPermission("md.allowance_item.create").EnableAudit(),
		Update:      crud.NewUpdate[model.AllowanceItem, payload.AllowanceItemParams]().RequiredPermission("md.allowance_item.update").EnableAudit(),
		Delete:      crud.NewDelete[model.AllowanceItem]().RequiredPermission("md.allowance_item.delete").EnableAudit(),
	}
}
`, string(rendered["resource"]), "the generated resource must alias the foreign model package it collides with")
	})
}

func TestBuildResourceEntityRejectsAnUnknownOperation(t *testing.T) {
	_, err := buildResourceEntity(testProject(), ResourceRequest{
		Table:  "md_allowance_item",
		Module: "md",
		Ops:    []string{"find_tree"},
	}, allowanceItemTable())

	require.ErrorIs(t, err, ErrUnknownOperation,
		"find_tree needs a caller-supplied tree builder, so it must be refused rather than rendered broken")
}

// TestPrimaryKeyHandling covers the tables whose primary key the audit mixins
// do not supply. Every case here reaches bun through the generated model, and
// a model bun sees no primary key on fails every crud Update and Delete at
// runtime with ErrModelNoPrimaryKey — a failure no compiler catches.
func TestPrimaryKeyHandling(t *testing.T) {
	t.Run("CompositeKeyGetsPkTagsAndTheTrackedMixin", func(t *testing.T) {
		table := &schema.TableSchema{
			Name:       "md_ledger_entry",
			Comment:    "台账分录",
			PrimaryKey: &schema.PrimaryKey{Columns: []string{"tenant_id", "entry_id"}},
			Columns: []schema.Column{
				{Name: "tenant_id", Type: "varchar(20)", MaxLength: 20, IsPrimaryKey: true, Comment: "租户"},
				{Name: "entry_id", Type: "varchar(20)", MaxLength: 20, IsPrimaryKey: true, Comment: "分录号"},
				{Name: "created_at", Type: "timestamptz"},
				{Name: "created_by", Type: "varchar(20)"},
				{Name: "updated_at", Type: "timestamptz", Nullable: true},
				{Name: "updated_by", Type: "varchar(20)", Nullable: true},
				{Name: "amount", Type: "numeric(12,2)", Nullable: true, Comment: "金额"},
			},
		}

		entity, err := buildResourceEntity(testProject(), ResourceRequest{Module: "md", Entity: "ledger_entry"}, table)
		require.NoError(t, err, "building a composite-keyed entity must succeed")

		require.Equal(t, "orm.FullTrackedModel", entity.AuditEmbed,
			"a table with audit columns but no id column must take the mixin that carries no ID")
		require.False(t, entity.HasIDField, "the tracked mixin declares no ID field")

		rendered, err := renderEntityFiles(testProject(), entity)
		require.NoError(t, err, "rendering must succeed")

		model := string(rendered["model"])
		require.Contains(t, model, "TenantID string   `json:\"tenantId\" bun:\",pk\" validate:\"required,max=20\" label:\"租户\"`",
			"a key column the mixin does not supply must declare itself as a primary key")
		require.Contains(t, model, "EntryID  string   `json:\"entryId\" bun:\",pk\" validate:\"required,max=20\" label:\"分录号\"`",
			"every key column must carry the pk tag, not just the first")
		require.NotContains(t, model, "Amount   *float64 `json:\"amount,omitempty\" bun:\",pk\"",
			"a non-key column must not be marked as a primary key")

		require.NotContains(t, string(rendered["payload"]), "ID string `json:\"id\"`",
			"a model with no ID field must not advertise an id parameter its update silently discards")
	})

	t.Run("SingleKeyNotNamedIDGetsAPkTag", func(t *testing.T) {
		table := &schema.TableSchema{
			Name:       "md_region",
			PrimaryKey: &schema.PrimaryKey{Columns: []string{"code"}},
			Columns: []schema.Column{
				{Name: "code", Type: "varchar(12)", MaxLength: 12, IsPrimaryKey: true, Comment: "编码"},
				{Name: "name", Type: "varchar(64)", MaxLength: 64, Comment: "名称"},
			},
		}

		entity, err := buildResourceEntity(testProject(), ResourceRequest{Module: "md", Entity: "region"}, table)
		require.NoError(t, err, "building a code-keyed entity must succeed")

		require.Empty(t, entity.AuditEmbed, "a table with no audit columns matches no mixin")
		require.False(t, entity.KeylessTable, "the table does declare a primary key")

		require.Contains(t, string(mustRender(t, entity)["model"]), "bun:\",pk\"",
			"the sole key column must be marked even though nothing is named id")
	})

	t.Run("NonStringIDDoesNotTakeTheStringIDMixin", func(t *testing.T) {
		table := &schema.TableSchema{
			Name:       "md_legacy",
			PrimaryKey: &schema.PrimaryKey{Columns: []string{"id"}},
			Columns: []schema.Column{
				{Name: "id", Type: "bigint", IsPrimaryKey: true, IsAutoIncrement: true, Comment: "主键"},
				{Name: "name", Type: "varchar(50)", MaxLength: 50, Comment: "名称"},
			},
		}

		entity, err := buildResourceEntity(testProject(), ResourceRequest{Module: "md", Entity: "legacy"}, table)
		require.NoError(t, err, "building a legacy integer-keyed entity must succeed")

		require.Empty(t, entity.AuditEmbed,
			"orm.Model declares ID string, so a bigint identity column must not match it — the ORM would generate a 20-character XID into a numeric column on every insert")
		require.False(t, entity.HasIDField, "no mixin supplied an ID field")

		model := string(mustRender(t, entity)["model"])
		require.Contains(t, model, "ID   int64  `json:\"id\" bun:\",pk\" label:\"主键\"`",
			"the real column type must survive, carrying the pk tag itself")
		require.NotContains(t, model, "orm.Model", "the string-ID mixin must not be embedded")
	})

	t.Run("KeylessTableIsReported", func(t *testing.T) {
		table := &schema.TableSchema{
			Name: "md_audit_trail",
			Columns: []schema.Column{
				{Name: "occurred_at", Type: "timestamptz"},
				{Name: "message", Type: "varchar(200)", MaxLength: 200},
			},
		}

		entity, err := buildResourceEntity(testProject(), ResourceRequest{Module: "md", Entity: "audit_trail"}, table)
		require.NoError(t, err, "building a keyless entity must succeed")

		require.True(t, entity.KeylessTable,
			"a table with no primary key must be flagged, because nothing the generator writes can give crud one")
	})
}

// mustRender renders an entity's files or fails the test.
func mustRender(t *testing.T, entity Entity) map[string][]byte {
	t.Helper()

	rendered, err := renderEntityFiles(testProject(), entity)
	require.NoError(t, err, "rendering the entity's files must succeed")

	return rendered
}

// TestColumnNamesThatDoNotDeriveBack covers columns whose Go field name does
// not underscore back to the column. bun re-derives a column from the field
// name, so an untagged field would address a column that does not exist and
// every query against it would fail at runtime.
func TestColumnNamesThatDoNotDeriveBack(t *testing.T) {
	table := &schema.TableSchema{
		Name:       "md_endpoint",
		PrimaryKey: &schema.PrimaryKey{Columns: []string{"api_v1"}},
		Columns: []schema.Column{
			{Name: "api_v1", Type: "varchar(20)", MaxLength: 20, IsPrimaryKey: true, Comment: "接口"},
			{Name: "ip_v4", Type: "varchar(15)", MaxLength: 15, Nullable: true, Comment: "地址"},
			{Name: "2fa_enabled", Type: "bool", Comment: "双因素"},
			{Name: "remark", Type: "varchar(64)", MaxLength: 64, Nullable: true, Comment: "备注"},
		},
	}

	entity, err := buildResourceEntity(testProject(), ResourceRequest{Module: "md", Entity: "endpoint"}, table)
	require.NoError(t, err, "building an entity with awkward column names must succeed")

	model := string(mustRender(t, entity)["model"])

	t.Run("StatesTheColumnWhenTheNameWouldNotDeriveBack", func(t *testing.T) {
		require.Contains(t, model, `bun:"api_v1,pk"`,
			"APIV1 underscores back to apiv1, so the real column must be named explicitly alongside pk")
		require.Contains(t, model, `bun:"ip_v4"`,
			"IPV4 underscores back to ipv4, so the real column must be named explicitly")
	})

	t.Run("EscapesAColumnThatIsNotALegalIdentifier", func(t *testing.T) {
		require.Contains(t, model, `Col2faEnabled bool`,
			"a column starting with a digit must be escaped into a legal exported identifier")
		require.Contains(t, model, `bun:"2fa_enabled"`,
			"the escaped field must state the column it really addresses")
	})

	t.Run("LeavesAWellBehavedColumnUntagged", func(t *testing.T) {
		require.Contains(t, model, "Remark        *string `json:\"remark,omitempty\" validate:\"omitempty,max=64\" label:\"备注\"`",
			"a name that derives back needs no bun tag, and adding one everywhere would be noise")
	})

	t.Run("StaysValidGo", func(t *testing.T) {
		require.NotEmpty(t, model, "rendering runs the result through gofmt, which parses it")
	})
}

// TestModuleRegistrationCountsEveryStep pins that the patch closure reports a
// change when any of its steps made one.
//
// It runs three edits — two imports and the fx option — and used to return only
// the last one's flag. AddPatchOrCreate discards the buffer of a patch that
// reports no change, so a module already carrying the option but missing an
// import was recorded as "already registered" and the import was dropped,
// leaving a file that does not compile and a run that said it did nothing.
func TestModuleRegistrationCountsEveryStep(t *testing.T) {
	proj := writeProject(t, "")

	moduleFile := proj.ModuleFile("md")
	require.NoError(t, os.WriteFile(moduleFile, []byte(
		"package md\n\nvar Module = vef.Module(\n\t\"app:md\",\n\tvef.Provide(service.NewNotificationService),\n)\n"), 0o644),
		"seeding a module that registers the service without importing it must succeed")

	plan := NewPlan(proj)
	require.NoError(t, GenerateService(proj, ServiceRequest{Name: "Notification", Module: "md"}, plan),
		"generating the already-registered service must succeed")
	require.NoError(t, plan.Apply(io.Discard, termenv.DefaultOutput()), "applying the plan must succeed")

	module := readFile(t, proj, "internal/md/module.go")

	assert.Contains(t, module, `"acme/internal/md/service"`, "the missing import must be added even though the option was already there")
	assert.Contains(t, module, `"github.com/coldsmirk/vef-framework-go"`, "the framework import must be added too")
	assert.Equal(t, 1, strings.Count(module, "vef.Provide(service.NewNotificationService)"),
		"the option must not be registered a second time")
}
