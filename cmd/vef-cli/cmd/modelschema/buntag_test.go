package modelschema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBunStructTag(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		tag := parseBunStructTag("")
		assert.Empty(t, tag.Name, "Empty tag has no name")
		assert.Nil(t, tag.Options, "Empty tag has no options")
	})

	t.Run("BareName", func(t *testing.T) {
		tag := parseBunStructTag("users")
		assert.Equal(t, "users", tag.Name, "Bare segment is the name")
		assert.False(t, tag.hasOption("users"), "Bare name is not an option")
	})

	t.Run("NameThenFlags", func(t *testing.T) {
		tag := parseBunStructTag("col,notnull,pk")
		assert.Equal(t, "col", tag.Name, "First bare segment is the name")
		assert.True(t, tag.hasOption("notnull"), "notnull is a flag option")
		assert.True(t, tag.hasOption("pk"), "pk is a flag option")
	})

	t.Run("LeadingCommaGivesEmptyName", func(t *testing.T) {
		tag := parseBunStructTag(",scanonly")
		assert.Empty(t, tag.Name, "Leading comma leaves the name empty")
		assert.True(t, tag.hasOption("scanonly"), "scanonly is the option")
	})

	t.Run("KeyValueOptions", func(t *testing.T) {
		tag := parseBunStructTag("table:users,alias:u")
		assert.Empty(t, tag.Name, "A leading key:value segment is an option, not the name")
		table, ok := tag.option("table")
		require.True(t, ok, "table option present")
		assert.Equal(t, "users", table, "table option value")

		alias, ok := tag.option("alias")
		require.True(t, ok, "alias option present")
		assert.Equal(t, "u", alias, "alias option value")
	})

	t.Run("NoWhitespaceTrimming", func(t *testing.T) {
		// bun's parser does not trim: the space becomes part of the option key, so
		// "scanonly" is NOT recognized — the single most important fidelity property.
		tag := parseBunStructTag("col, scanonly")
		assert.Equal(t, "col", tag.Name, "Name segment is untouched")
		assert.False(t, tag.hasOption("scanonly"), "A space-prefixed option key must not match the trimmed name")
		assert.True(t, tag.hasOption(" scanonly"), "The key retains its leading space verbatim")
	})

	t.Run("RepeatedOptionLastWins", func(t *testing.T) {
		tag := parseBunStructTag("column:a,column:b")
		got, ok := tag.option("column")
		require.True(t, ok, "column option present")
		assert.Equal(t, "b", got, "option() returns the last value for a repeated key")
	})

	t.Run("EmptyOptionValue", func(t *testing.T) {
		tag := parseBunStructTag("embed:")
		got, ok := tag.option("embed")
		require.True(t, ok, "An empty-value option is still present")
		assert.Empty(t, got, "Its value is the empty string")
	})

	t.Run("QuotedValueKeepsComma", func(t *testing.T) {
		// A quoted value may contain commas; they must not split the option.
		tag := parseBunStructTag(`msg:"hello,world",notnull`)
		got, ok := tag.option("msg")
		require.True(t, ok, "msg option present")
		assert.Equal(t, "hello,world", got, "Comma inside quotes stays part of the value")
		assert.True(t, tag.hasOption("notnull"), "The option after the quoted value is still parsed")
	})
}
