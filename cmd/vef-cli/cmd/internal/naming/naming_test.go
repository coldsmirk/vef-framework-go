package naming

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPascalCase(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "SingleWord", input: "name", want: "Name"},
		{name: "MultipleWords", input: "allowance_item", want: "AllowanceItem"},
		{name: "LeadingAcronym", input: "id", want: "ID"},
		{name: "EmbeddedAcronym", input: "api_key", want: "APIKey"},
		{name: "TrailingAcronym", input: "callback_url", want: "CallbackURL"},
		{name: "AcronymBetweenWords", input: "user_id_hash", want: "UserIDHash"},
		{name: "ConsecutiveAcronymsKeepOnlyTheFirstUppercase", input: "json_api", want: "JSONApi"},
		{name: "DigitsStayWithTheirWord", input: "sha256_sum", want: "Sha256Sum"},
		{name: "Empty", input: "", want: ""},
		{name: "DoubledSeparator", input: "created__at", want: "CreatedAt"},
		{name: "TrailingSeparator", input: "created_at_", want: "CreatedAt"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, PascalCase(tc.input), "PascalCase(%q) must produce the Go exported form", tc.input)
		})
	}
}

// TestPascalCaseRoundTrip pins the property the generated code depends on:
// bun derives a column name back from the field name, so a field this package
// names must snake_case back to the column it came from. A stacked acronym
// (JSONAPI) is what breaks it, which is why PascalCase refuses to produce one.
func TestPascalCaseRoundTrip(t *testing.T) {
	columns := []string{
		"id", "created_at", "created_by", "is_active", "api_key", "callback_url",
		"allowance_item", "user_id_hash", "json_api", "sha256_sum", "remark",
	}

	for _, column := range columns {
		t.Run(column, func(t *testing.T) {
			assert.Equal(t, column, SnakeCase(PascalCase(column)),
				"a field named %q must snake_case back to its own column", PascalCase(column))
		})
	}
}

func TestCamelCase(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "SingleWord", input: "name", want: "name"},
		{name: "MultipleWords", input: "allowance_item", want: "allowanceItem"},
		{name: "LeadingAcronym", input: "id", want: "id"},
		{name: "LeadingAcronymBeforeWord", input: "api_key", want: "apiKey"},
		// The wire name is plain camelCase even where the Go field name keeps
		// the acronym: the framework's models carry tenantId and baseUrl, never
		// tenantID or baseURL, and the JSON name is a client-facing contract.
		{name: "TrailingAcronymIsNotKeptUppercase", input: "tenant_id", want: "tenantId"},
		{name: "TrailingAcronymUrl", input: "callback_url", want: "callbackUrl"},
		{name: "InteriorAcronym", input: "user_id_hash", want: "userIdHash"},
		{name: "Empty", input: "", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, CamelCase(tc.input), "CamelCase(%q) must produce the json tag form", tc.input)
		})
	}
}

func TestEntityFromTable(t *testing.T) {
	cases := []struct {
		name   string
		table  string
		module string
		want   string
	}{
		{name: "StripsTheModulePrefix", table: "md_allowance_item", module: "md", want: "allowance_item"},
		{name: "KeepsAnotherModulesPrefix", table: "sys_user", module: "md", want: "sys_user"},
		{name: "NoModuleKeepsTheTableName", table: "md_holiday", module: "", want: "md_holiday"},
		{name: "PrefixOnlyTableIsNotStrippedToEmpty", table: "md_", module: "md", want: "md_"},
		{name: "UppercaseTableNormalizes", table: "MD_HOLIDAY", module: "md", want: "holiday"},
		{name: "SurroundingSpace", table: "  md_holiday ", module: "md", want: "holiday"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, EntityFromTable(tc.table, tc.module),
				"EntityFromTable(%q, %q) must drop only the module's own prefix", tc.table, tc.module)
		})
	}
}

func TestTableAlias(t *testing.T) {
	cases := []struct {
		name  string
		table string
		want  string
	}{
		{name: "InitialsOfEveryWord", table: "md_allowance_item", want: "mai"},
		{name: "TwoWords", table: "sys_user", want: "su"},
		{name: "SingleWordKeepsTwoLetters", table: "holiday", want: "ho"},
		{name: "SingleLetterWord", table: "x", want: "x"},
		{name: "Empty", table: "", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, TableAlias(tc.table), "TableAlias(%q) must follow the initials convention", tc.table)
		})
	}
}

func TestFileName(t *testing.T) {
	assert.Equal(t, "allowance_item.go", FileName("allowance_item"), "FileName must append the Go extension")
}

func TestSnakeCase(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "PascalWords", input: "AllowanceItem", want: "allowance_item"},
		{name: "Acronym", input: "ID", want: "id"},
		{name: "AcronymBeforeWord", input: "APIKey", want: "api_key"},
		{name: "DigitsStayWithTheirWord", input: "Sha256Sum", want: "sha256_sum"},
		{name: "Empty", input: "", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, SnakeCase(tc.input), "SnakeCase(%q) must mirror bun's underscore rule", tc.input)
		})
	}
}
