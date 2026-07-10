package formeditor_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
//
// Both suites enumerate testdata/ at runtime (mirroring the TS twin's glob) so a
// fixture pair added for the TS side but not hand-registered here still runs: a
// <name>.schema.json WITH a sibling <name>.expected.json is a golden case, one
// WITHOUT is an error case. The error tokens each error case must surface stay
// registered in errorTokens, and a testdata error fixture missing from that map
// fails loudly — enumeration stays automatic while token coverage stays
// intentional.

// errorTokens maps each error-case fixture (a *.schema.json with no sibling
// *.expected.json) to the language-independent tokens (paths / widget types) its
// message must name. A nil slice registers a fixture whose message carries no
// stable token to assert (malformed JSON). Every error fixture in testdata/ must
// have an entry here; enumerateSchemas fails the error test otherwise.
var errorTokens = map[string][]string{
	"unmappable_switch":           {"flag", "switch"},
	"unmappable_daterange":        {"span", "daterange"},
	"unknown_type":                {"score", "rating"},
	"nested_subform":              {"items.parts"},
	"table_columns_empty":         {"items"},
	"unmappable_column":           {"items.flag", "switch"},
	"cross_device_kind_conflict":  {"amount", "input", "number"},
	"cross_device_table_conflict": {"items"},
	// A key valid on pc (textfield) but unmappable on mobile (switch) must fail
	// regardless of sighting order — the second sighting is classified, not
	// silently deduped.
	"cross_device_unmappable": {"a", "switch"},
	"malformed_json":          nil,
}

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

// enumerateSchemas returns every fixture base name (the part before
// ".schema.json") under testdata/, split by whether it has a sibling
// ".expected.json": golden cases carry an expected file, error cases do not.
func enumerateSchemas(t *testing.T) (golden, errorCases []string) {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join("testdata", "*.schema.json"))
	require.NoError(t, err, "should glob testdata schema fixtures")
	require.NotEmpty(t, matches, "testdata must contain at least one schema fixture")

	for _, schemaPath := range matches {
		name := strings.TrimSuffix(filepath.Base(schemaPath), ".schema.json")

		if _, err := os.Stat(filepath.Join("testdata", name+".expected.json")); err == nil {
			golden = append(golden, name)
		} else {
			require.True(t, os.IsNotExist(err), "should stat expected fixture for %q: %v", name, err)
			errorCases = append(errorCases, name)
		}
	}

	return golden, errorCases
}

func TestParseFormFieldsGolden(t *testing.T) {
	parser := formeditor.NewParser()

	golden, _ := enumerateSchemas(t)

	// Every golden case mirrors a scenario in project.test.ts; the expected
	// fixture is the TypeScript projector's asserted output.
	for _, name := range golden {
		t.Run(name, func(t *testing.T) {
			schema := loadFixture(t, name+".schema.json")
			want := loadExpected(t, name+".expected.json")

			got, err := parser.ParseFormFields(t.Context(), schema)
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

	_, errorCases := enumerateSchemas(t)

	for _, name := range errorCases {
		t.Run(name, func(t *testing.T) {
			tokens, registered := errorTokens[name]
			require.Truef(t, registered,
				"error fixture %q has no sibling %q.expected.json and is not registered in errorTokens; "+
					"register its expected error tokens (or add an expected file to make it a golden case)",
				name, name)

			got, err := parser.ParseFormFields(t.Context(), loadFixture(t, name+".schema.json"))

			require.Error(t, err, "fixture %q must abort the deploy", name)
			require.Nil(t, got, "fixture %q must project no fields on error", name)
			require.ErrorIs(t, err, shared.ErrInvalidFormDesign, "fixture %q must surface as an invalid form design", name)

			for _, token := range tokens {
				require.Contains(t, err.Error(), token, "fixture %q error must name %q", name, token)
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
			got, err := parser.ParseFormFields(t.Context(), tc.input)

			require.NoError(t, err, "a blank schema must not error")
			require.Nil(t, got, "a blank schema must project no fields")
		})
	}
}
