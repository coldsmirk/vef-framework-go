package gopatch

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const moduleSource = `package md

import (
	"github.com/coldsmirk/vef-framework-go"

	"acme/internal/md/resource"
)

var Module = vef.Module(
	"app:md",
	vef.ProvideAPIResource(resource.NewHolidayResource),
)
`

const registrySource = `package model

var (
	HolidayModel = new(Holiday)
	DeviceModel  = new(Device)
)
`

func TestAppendCallArg(t *testing.T) {
	t.Run("AppendsAsItsOwnLine", func(t *testing.T) {
		file := New("module.go", []byte(moduleSource))

		changed, err := file.AppendCallArg("Module", "vef.ProvideAPIResource(resource.NewDeviceResource)")
		require.NoError(t, err, "Appending to an existing module call must succeed")
		require.True(t, changed, "A missing registration must be reported as a change")
		require.NoError(t, file.Format(), "The patched file must still be valid Go")

		require.Equal(t, `package md

import (
	"github.com/coldsmirk/vef-framework-go"

	"acme/internal/md/resource"
)

var Module = vef.Module(
	"app:md",
	vef.ProvideAPIResource(resource.NewHolidayResource),
	vef.ProvideAPIResource(resource.NewDeviceResource),
)
`, string(file.Source()), "The new registration must be appended as the last argument")
	})

	t.Run("IsIdempotent", func(t *testing.T) {
		file := New("module.go", []byte(moduleSource))

		changed, err := file.AppendCallArg("Module", "vef.ProvideAPIResource(resource.NewHolidayResource)")
		require.NoError(t, err, "Re-appending an existing registration must not error")
		require.False(t, changed, "An existing registration must not be reported as a change")
		require.Equal(t, moduleSource, string(file.Source()), "An existing registration must leave the file untouched")
	})

	t.Run("MatchesRegardlessOfWhitespace", func(t *testing.T) {
		file := New("module.go", []byte(moduleSource))

		changed, err := file.AppendCallArg("Module", "vef.ProvideAPIResource( resource.NewHolidayResource )")
		require.NoError(t, err, "Whitespace-different arguments must still compare")
		require.False(t, changed, "A registration differing only in whitespace is already present")
	})

	t.Run("AppendsInlineToASingleLineCall", func(t *testing.T) {
		file := New("module.go", []byte("package md\n\nvar Module = vef.Module(\"app:md\")\n"))

		changed, err := file.AppendCallArg("Module", "vef.Provide(service.NewX)")
		require.NoError(t, err, "A single-line call must accept an argument")
		require.True(t, changed, "The argument must be reported as a change")
		require.Equal(t, "package md\n\nvar Module = vef.Module(\"app:md\", vef.Provide(service.NewX))\n",
			string(file.Source()), "A single-line call must stay on one line")
	})

	t.Run("UnknownVariableFails", func(t *testing.T) {
		file := New("module.go", []byte(moduleSource))

		_, err := file.AppendCallArg("Missing", "vef.Provide(x)")
		require.ErrorIs(t, err, ErrVarNotFound, "A missing variable must be named rather than silently skipped")
	})

	t.Run("NonCallVariableFails", func(t *testing.T) {
		file := New("module.go", []byte("package md\n\nvar Module = 1\n"))

		_, err := file.AppendCallArg("Module", "vef.Provide(x)")
		require.ErrorIs(t, err, ErrNotACall, "A variable with no argument list must fail loudly")
	})
}

const entryPointSource = `package main

import (
	"github.com/coldsmirk/vef-framework-go"

	ivef "acme/internal/vef"
)

func main() {
	vef.Run(
		ivef.Module,
	)
}
`

func TestAppendCallArgIn(t *testing.T) {
	t.Run("AppendsToACallInsideAFunction", func(t *testing.T) {
		file := New("main.go", []byte(entryPointSource))

		appended, err := file.AppendCallArgIn("main", "vef.Run", "md.Module")
		require.NoError(t, err, "appending to the entry point must succeed")
		require.True(t, appended, "a missing module must be reported as a change")
		require.NoError(t, file.Format(), "the patched entry point must still be valid Go")

		require.Contains(t, string(file.Source()), "\t\tivef.Module,\n\t\tmd.Module,\n",
			"the module must be appended to the run call's argument list")
	})

	t.Run("IsIdempotent", func(t *testing.T) {
		file := New("main.go", []byte(entryPointSource))

		appended, err := file.AppendCallArgIn("main", "vef.Run", "ivef.Module")
		require.NoError(t, err, "re-appending an existing module must not error")
		require.False(t, appended, "an existing module must not be reported as a change")
	})

	t.Run("ReportsAMissingFunctionWithoutFailing", func(t *testing.T) {
		file := New("main.go", []byte("package main\n\nfunc other() {}\n"))

		appended, err := file.AppendCallArgIn("main", "vef.Run", "md.Module")
		require.NoError(t, err, "an entry point shaped differently is not an error")
		require.False(t, appended, "a project that wires itself differently must be reported, not failed")
	})

	t.Run("ReportsAMissingCallWithoutFailing", func(t *testing.T) {
		file := New("main.go", []byte("package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n"))

		appended, err := file.AppendCallArgIn("main", "vef.Run", "md.Module")
		require.NoError(t, err, "a main that does not call vef.Run is not an error")
		require.False(t, appended, "a main without the expected call must be reported, not failed")
	})
}

func TestAppendVarSpec(t *testing.T) {
	t.Run("AppendsAndRealigns", func(t *testing.T) {
		file := New("models.go", []byte(registrySource))

		changed, err := file.AppendVarSpec("AllowanceItemModel", "new(AllowanceItem)")
		require.NoError(t, err, "Appending a model registry entry must succeed")
		require.True(t, changed, "A missing entry must be reported as a change")
		require.NoError(t, file.Format(), "The patched registry must still be valid Go")

		require.Equal(t, `package model

var (
	HolidayModel       = new(Holiday)
	DeviceModel        = new(Device)
	AllowanceItemModel = new(AllowanceItem)
)
`, string(file.Source()), "gofmt must realign the whole block after the insertion")
	})

	t.Run("IsIdempotent", func(t *testing.T) {
		file := New("models.go", []byte(registrySource))

		changed, err := file.AppendVarSpec("DeviceModel", "new(Device)")
		require.NoError(t, err, "Re-appending an existing entry must not error")
		require.False(t, changed, "An existing entry must not be reported as a change")
	})

	t.Run("CreatesTheBlockWhenAbsent", func(t *testing.T) {
		file := New("models.go", []byte("package model\n"))

		changed, err := file.AppendVarSpec("DeviceModel", "new(Device)")
		require.NoError(t, err, "A registry file with no block must still accept an entry")
		require.True(t, changed, "Creating the block must be reported as a change")
		require.NoError(t, file.Format(), "The created block must be valid Go")

		require.Equal(t, "package model\n\nvar (\n\tDeviceModel = new(Device)\n)\n",
			string(file.Source()), "A missing block must be created rather than reported as an error")
	})
}

// TestAppendPreservesAComment covers a comment written on the closing
// parenthesis's own line. The insertion is zero-width, so reusing that line's
// text as indentation would leave the comment behind AND copy it — producing
// valid Go that gofmt cannot object to, written straight to the developer's file.
func TestAppendPreservesAComment(t *testing.T) {
	file := New("module.go", []byte(`package md

var Module = vef.Module(
	"app:md",
	vef.ProvideAPIResource(resource.NewHolidayResource),
	/* keep me */)
`))

	changed, err := file.AppendCallArg("Module", "vef.ProvideAPIResource(resource.NewDeviceResource)")
	require.NoError(t, err, "appending beside a comment must succeed")
	require.True(t, changed, "the registration must be reported as a change")
	require.NoError(t, file.Format(), "the result must still be valid Go")

	source := string(file.Source())
	require.Equal(t, 1, strings.Count(source, "/* keep me */"), "the comment must survive exactly once")
	require.Contains(t, source, "vef.ProvideAPIResource(resource.NewDeviceResource),", "the registration must be added")
}
