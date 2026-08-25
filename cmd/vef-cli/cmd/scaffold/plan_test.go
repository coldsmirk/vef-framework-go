package scaffold

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/internal/gopatch"
	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/internal/project"
)

// newTestPlan returns an empty plan rooted at a fresh temporary directory.
func newTestPlan(t *testing.T) (*Plan, string) {
	t.Helper()

	root := t.TempDir()

	return NewPlan(&project.Project{Root: root, ModulePath: "acme", Config: project.DefaultConfig()}), root
}

// applyPlan runs the plan and returns what it reported.
func applyPlan(t *testing.T, plan *Plan) string {
	t.Helper()

	var report bytes.Buffer
	require.NoError(t, plan.Apply(&report, termenv.DefaultOutput()), "applying the plan must succeed")

	return report.String()
}

func TestPlanAddFile(t *testing.T) {
	t.Run("CreatesAMissingFileAndItsParents", func(t *testing.T) {
		plan, root := newTestPlan(t)
		path := filepath.Join(root, "internal", "md", "model", "holiday.go")

		plan.AddFile(path, []byte("package model\n"), false)
		require.False(t, plan.Empty(), "a planned create must count as a change")

		report := applyPlan(t, plan)
		assert.Contains(t, report, "create internal/md/model/holiday.go", "the report must name what was created")

		written, err := os.ReadFile(path)
		require.NoError(t, err, "the file must exist after Apply")
		assert.Equal(t, "package model\n", string(written), "the planned content must land verbatim")
	})

	t.Run("SkipsAnIdenticalFileWithoutForce", func(t *testing.T) {
		plan, root := newTestPlan(t)
		path := filepath.Join(root, "holiday.go")
		require.NoError(t, os.WriteFile(path, []byte("package model\n"), 0o644), "seeding the existing file must succeed")

		plan.AddFile(path, []byte("package model\n"), false)

		assert.True(t, plan.Empty(), "re-generating identical content must report as no change rather than touch the file")
		assert.Contains(t, applyPlan(t, plan), "already up to date", "the skip must say why")
	})

	t.Run("RefusesToOverwriteWithoutForce", func(t *testing.T) {
		plan, root := newTestPlan(t)
		path := filepath.Join(root, "holiday.go")
		require.NoError(t, os.WriteFile(path, []byte("// hand-edited\n"), 0o644), "seeding the existing file must succeed")

		plan.AddFile(path, []byte("package model\n"), false)
		report := applyPlan(t, plan)

		assert.Contains(t, report, ErrFileExists.Error(), "scaffolding hands a file over, so overwriting must be asked for")

		kept, err := os.ReadFile(path)
		require.NoError(t, err, "reading the file back must succeed")
		assert.Equal(t, "// hand-edited\n", string(kept), "the developer's edits must survive a re-run")
	})

	t.Run("OverwritesWithForce", func(t *testing.T) {
		plan, root := newTestPlan(t)
		path := filepath.Join(root, "holiday.go")
		require.NoError(t, os.WriteFile(path, []byte("// hand-edited\n"), 0o644), "seeding the existing file must succeed")

		plan.AddFile(path, []byte("package model\n"), true)
		assert.Contains(t, applyPlan(t, plan), "update holiday.go", "a forced overwrite must report as an update, not a create")

		written, err := os.ReadFile(path)
		require.NoError(t, err, "reading the file back must succeed")
		assert.Equal(t, "package model\n", string(written), "--force must replace the content")
	})
}

func TestPlanAddPatchOrCreate(t *testing.T) {
	patch := func(file *gopatch.File) (bool, error) {
		return file.AppendVarSpec("HolidayModel", "new(Holiday)")
	}

	t.Run("CreatesTheTargetFromItsInitialContent", func(t *testing.T) {
		plan, root := newTestPlan(t)
		path := filepath.Join(root, "models.go")

		require.NoError(t, plan.AddPatchOrCreate(path, "package model\n", patch), "creating the registry on demand must succeed")
		assert.Contains(t, applyPlan(t, plan), "create models.go", "a file that had to be created must report as a create")

		written, err := os.ReadFile(path)
		require.NoError(t, err, "the file must exist after Apply")
		assert.Contains(t, string(written), "HolidayModel = new(Holiday)", "the patch must have run against the initial content")
	})

	t.Run("SkipsAnAlreadyRegisteredEntry", func(t *testing.T) {
		plan, root := newTestPlan(t)
		path := filepath.Join(root, "models.go")
		require.NoError(t, os.WriteFile(path, []byte("package model\n\nvar (\n\tHolidayModel = new(Holiday)\n)\n"), 0o644),
			"seeding the registry must succeed")

		require.NoError(t, plan.AddPatchOrCreate(path, "package model\n", patch), "patching must succeed")

		assert.True(t, plan.Empty(), "re-registering an entry that is already there must change nothing")
		assert.Contains(t, applyPlan(t, plan), "already registered", "the skip must say why")
	})

	t.Run("ReportsAMalformedTargetDuringPlanning", func(t *testing.T) {
		plan, root := newTestPlan(t)
		path := filepath.Join(root, "models.go")
		require.NoError(t, os.WriteFile(path, []byte("package model\n\nfunc broken( {\n"), 0o644),
			"seeding the malformed target must succeed")

		err := plan.AddPatchOrCreate(path, "package model\n", patch)
		require.Error(t, err, "a target that cannot be parsed must surface while planning, before anything is written")
	})
}

func TestPlanPreviewWritesNothing(t *testing.T) {
	plan, root := newTestPlan(t)
	path := filepath.Join(root, "holiday.go")

	plan.AddFile(path, []byte("package model\n"), false)

	var preview bytes.Buffer
	plan.Preview(&preview, termenv.DefaultOutput())

	assert.Contains(t, preview.String(), "package model", "--dry-run must show what would land, not only where")

	_, err := os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist, "--dry-run must not touch the filesystem")
}

func TestPlanNote(t *testing.T) {
	plan, _ := newTestPlan(t)
	plan.Note("add md.Module to your application's vef.Run call")

	assert.True(t, plan.Empty(), "a note writes nothing, so it must not count as a change")
	assert.Contains(t, applyPlan(t, plan), "TODO   add md.Module", "a step the generator could not take must still be reported")
}
