package formeditor_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/formeditor"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
)

// The golden fixtures under testdata/ are the shared TS/Go parity corpus: each
// <name>.schema.json is the form-editor rich schema and each
// <name>.expected.json is the projected field list both this parser and
// @vef-framework-react/approval-form-bridge `projectFormSchema` must produce.
// A change to either projector must update the fixtures and land in both repos.

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err, "fixture %q must be readable", name)

	return data
}

func loadExpected(t *testing.T, name string) []approval.FormFieldDefinition {
	t.Helper()

	var fields []approval.FormFieldDefinition
	require.NoError(t, json.Unmarshal(loadFixture(t, name), &fields), "expected fixture %q must be valid JSON", name)

	return fields
}

func TestParseFormFieldsGolden(t *testing.T) {
	parser := formeditor.NewParser()

	// Every case mirrors a scenario in project.test.ts; the expected fixture is
	// the TypeScript projector's asserted output.
	names := []string{
		"scalar_full_textfield",
		"all_widget_kinds",
		"number_precision",
		"column_type_override",
		"decimal_no_precision",
		"label_fallback",
		"nested_layout_containers",
		"datasource_options",
		"subform_table",
		"subform_unbounded",
		"subform_layout_flatten",
		"subform_duplicate_column",
		"subform_column_ref",
		"cross_device_dedupe",
		"cross_device_table_match",
		"non_keyed_skipped",
		"linkage_ignored",
		"empty_presentations",
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			schema := loadFixture(t, name+".schema.json")
			want := loadExpected(t, name+".expected.json")

			got, err := parser.ParseFormFields(schema)
			require.NoError(t, err, "fixture %q must project without error", name)

			if len(want) == 0 {
				require.Empty(t, got, "fixture %q must project to no fields", name)

				return
			}

			require.Equal(t, want, got, "fixture %q projection must match its golden expected fields", name)
		})
	}
}

func TestParseFormFieldsErrors(t *testing.T) {
	parser := formeditor.NewParser()

	cases := []struct {
		name     string
		contains []string // language-independent tokens (paths / widget types) the message must name
	}{
		{"unmappable_switch", []string{"flag", "switch"}},
		{"unmappable_daterange", []string{"span", "daterange"}},
		{"unknown_type", []string{"score", "rating"}},
		{"nested_subform", []string{"items.parts"}},
		{"table_columns_empty", []string{"items"}},
		{"unmappable_column", []string{"items.flag", "switch"}},
		{"cross_device_kind_conflict", []string{"amount", "input", "number"}},
		{"cross_device_table_conflict", []string{"items"}},
		{"malformed_json", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parser.ParseFormFields(loadFixture(t, tc.name+".schema.json"))

			require.Error(t, err, "fixture %q must abort the deploy", tc.name)
			require.Nil(t, got, "fixture %q must project no fields on error", tc.name)
			require.ErrorIs(t, err, shared.ErrInvalidFormDesign, "fixture %q must surface as an invalid form design", tc.name)

			for _, token := range tc.contains {
				require.Contains(t, err.Error(), token, "fixture %q error must name %q", tc.name, token)
			}
		})
	}
}

func TestParseFormFieldsBlankSchema(t *testing.T) {
	parser := formeditor.NewParser()

	cases := []struct {
		name  string
		input json.RawMessage
	}{
		{"nil", nil},
		{"empty", json.RawMessage("")},
		{"whitespace", json.RawMessage("  \n\t ")},
		{"json null", json.RawMessage("null")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parser.ParseFormFields(tc.input)

			require.NoError(t, err, "a blank schema must not error")
			require.Nil(t, got, "a blank schema must project no fields")
		})
	}
}
