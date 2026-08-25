package scaffold

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRenderer is a renderer over one entity, with every optional feature on so
// a case can assert what each one contributes.
var testRenderer = operationRenderer{
	model:          "model.Holiday",
	search:         "payload.HolidaySearch",
	params:         "payload.HolidayParams",
	permission:     func(action string) string { return "md.holiday." + action },
	auditUserModel: "sysmodel.UserModel",
	audit:          true,
}

func TestOperationRender(t *testing.T) {
	cases := []struct {
		name      string
		operation string
		embed     string
		init      string
	}{
		{
			name: "QueryTakesModelAndSearch", operation: "find_page",
			embed: "crud.FindPage[model.Holiday, payload.HolidaySearch]",
			init: "crud.NewFindPage[model.Holiday, payload.HolidaySearch]()" +
				`.RequiredPermission("md.holiday.query").WithAuditUserNames(sysmodel.UserModel)`,
		},
		{
			name: "MutationTakesModelAndParams", operation: "create",
			embed: "crud.Create[model.Holiday, payload.HolidayParams]",
			init:  `crud.NewCreate[model.Holiday, payload.HolidayParams]().RequiredPermission("md.holiday.create").EnableAudit()`,
		},
		{
			name: "ByIDTakesTheModelAlone", operation: "delete",
			embed: "crud.Delete[model.Holiday]",
			init:  `crud.NewDelete[model.Holiday]().RequiredPermission("md.holiday.delete").EnableAudit()`,
		},
		{
			// Gating the options feed is what produces the empty-dropdown class
			// of bug on a screen the user is otherwise allowed to see.
			name: "AnOptionsFeedStaysOpen", operation: "find_options",
			embed: "crud.FindOptions[model.Holiday, payload.HolidaySearch]",
			init:  "crud.NewFindOptions[model.Holiday, payload.HolidaySearch]()",
		},
		{
			// Import reads a file rather than a search payload, so it takes the
			// model alone — the generic arity has to match crud.Import exactly
			// or the generated resource does not compile.
			name: "ImportTakesTheModelAlone", operation: "import",
			embed: "crud.Import[model.Holiday]",
			init:  `crud.NewImport[model.Holiday]().RequiredPermission("md.holiday.import").EnableAudit()`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op, err := testRenderer.render(tc.operation)
			require.NoError(t, err, "%s is in the catalog and must render", tc.operation)

			assert.Equal(t, tc.embed, op.Embed, "the embedded type must carry the operation's generic parameters")
			assert.Equal(t, tc.init, op.Init, "the initializer must carry the full builder chain")
		})
	}

	t.Run("UnknownOperationNamesTheAlternatives", func(t *testing.T) {
		_, err := testRenderer.render("find_tree")

		require.ErrorIs(t, err, ErrUnknownOperation,
			"find_tree takes a tree-building function no generator can supply, so it must be refused")
		assert.Contains(t, err.Error(), "find_page", "the error must list what is available instead")
	})
}

func TestOperationRenderOptionalFeatures(t *testing.T) {
	t.Run("NoPermissionTemplateGeneratesNoRequiredPermission", func(t *testing.T) {
		renderer := testRenderer
		renderer.permission = func(string) string { return "" }

		op, err := renderer.render("create")
		require.NoError(t, err, "rendering must succeed")
		assert.NotContains(t, op.Init, "RequiredPermission", "a project that authorizes elsewhere must get no permission call")
	})

	t.Run("NoAuditUserModelGeneratesNoWithAuditUserNames", func(t *testing.T) {
		renderer := testRenderer
		renderer.auditUserModel = ""

		op, err := renderer.render("find_page")
		require.NoError(t, err, "rendering must succeed")
		assert.NotContains(t, op.Init, "WithAuditUserNames", "without a user model there is nothing to resolve ids against")
	})

	t.Run("AuditOffGeneratesNoEnableAudit", func(t *testing.T) {
		renderer := testRenderer
		renderer.audit = false

		op, err := renderer.render("create")
		require.NoError(t, err, "rendering must succeed")
		assert.NotContains(t, op.Init, "EnableAudit", "audit is a project-level convention and must be honored when off")
	})

	t.Run("AuditNeverReachesAReadOperation", func(t *testing.T) {
		op, err := testRenderer.render("find_page")
		require.NoError(t, err, "rendering must succeed")
		assert.NotContains(t, op.Init, "EnableAudit", "a query writes nothing, so it has no audit entry to make")
	})
}

func TestKnownOperations(t *testing.T) {
	names := KnownOperations()

	require.NotEmpty(t, names, "the catalog must not be empty")
	assert.IsIncreasing(t, names, "the list is used in help text and errors, so it must be sorted")
	assert.NotContains(t, names, "find_tree", "find_tree is deliberately absent and must not be advertised")
	assert.Len(t, names, len(operationCatalog), "every catalog entry must be listed")
}
