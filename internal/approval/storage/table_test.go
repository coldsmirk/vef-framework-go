package storage

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
)

func columnsFixture() []approval.FormTableColumn {
	return []approval.FormTableColumn{
		{ColumnName: "id", SortOrder: 0},
		{ColumnName: "instance_id", SortOrder: 1},
		{ColumnName: "reason", SourceFieldKey: new("reason"), SortOrder: 2},
		{ColumnName: "amount", SourceFieldKey: new("amount"), SortOrder: 3},
		{ColumnName: "tags", SourceFieldKey: new("tags"), SortOrder: 4},
		{ColumnName: "created_at", SortOrder: 5},
	}
}

func TestBuildInsertBindsValuesAndSkipsCreatedAt(t *testing.T) {
	formData := map[string]any{
		"reason": "needs review",
		"amount": 42.5,
		"tags":   []any{"a", "b"},
	}

	sql, args, err := buildInsert("apv_form_demo_v1", "inst-123", columnsFixture(), formData)
	require.NoError(t, err)

	// created_at is omitted (database default); every other column is present
	// with a single positional placeholder.
	assert.Equal(t,
		"INSERT INTO apv_form_demo_v1 (id, instance_id, reason, amount, tags) VALUES (?, ?, ?, ?, ?)",
		sql,
	)
	require.Len(t, args, 5)

	// id is a generated identifier (non-empty), instance_id is bound verbatim.
	assert.NotEmpty(t, args[0])
	assert.Equal(t, "inst-123", args[1])
	assert.Equal(t, "needs review", args[2])
	assert.Equal(t, 42.5, args[3])
	// Composite values are JSON-encoded to a string for the TEXT column.
	assert.Equal(t, `["a","b"]`, args[4])
}

func TestBuildInsertMissingFieldBindsNil(t *testing.T) {
	// "amount" and "tags" absent from form data -> NULL binds, not skipped
	// columns, so the row shape matches the column metadata.
	sql, args, err := buildInsert("apv_form_demo_v1", "inst-9", columnsFixture(), map[string]any{"reason": "x"})
	require.NoError(t, err)

	assert.Equal(t,
		"INSERT INTO apv_form_demo_v1 (id, instance_id, reason, amount, tags) VALUES (?, ?, ?, ?, ?)",
		sql,
	)
	require.Len(t, args, 5)
	assert.Nil(t, args[3], "missing amount binds NULL")
	assert.Nil(t, args[4], "missing tags binds NULL")
}

func TestBuildInsertEmptyStringNullsNonTextColumns(t *testing.T) {
	cols := []approval.FormTableColumn{
		{ColumnName: "id", SortOrder: 0},
		{ColumnName: "instance_id", SortOrder: 1},
		{ColumnName: "when", ColumnType: "DATE", SourceFieldKey: new("when"), SortOrder: 2},
		{ColumnName: "note", ColumnType: "TEXT", SourceFieldKey: new("note"), SortOrder: 3},
	}

	// Both present but empty: the DATE column nulls out (a real DATE rejects ""),
	// the TEXT column keeps "" as a value distinct from NULL.
	_, args, err := buildInsert("apv_form_demo_v1", "inst-1", cols, map[string]any{"when": "", "note": ""})
	require.NoError(t, err)
	require.Len(t, args, 4)
	assert.Nil(t, args[2], "empty date binds NULL, not empty string")
	assert.Equal(t, "", args[3], "empty text keeps empty string")
}

func TestBuildInsertRejectsUnsafeTableName(t *testing.T) {
	_, _, err := buildInsert("apv_form_demo; DROP TABLE x", "inst-1", columnsFixture(), nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidGeneratedIdentifier))
}

func TestBuildInsertRejectsUnsafeColumnName(t *testing.T) {
	cols := []approval.FormTableColumn{
		{ColumnName: "id", SortOrder: 0},
		{ColumnName: "instance_id", SortOrder: 1},
		{ColumnName: "bad name; --", SourceFieldKey: new("x"), SortOrder: 2},
	}

	_, _, err := buildInsert("apv_form_demo_v1", "inst-1", cols, map[string]any{"x": "y"})
	require.Error(t, err, "a column smuggled in from metadata must still be validated at render")
	assert.True(t, errors.Is(err, ErrInvalidGeneratedIdentifier))
}

func TestBuildDelete(t *testing.T) {
	sql, err := buildDelete("apv_form_demo_v1")
	require.NoError(t, err)
	assert.Equal(t, "DELETE FROM apv_form_demo_v1 WHERE instance_id = ?", sql)
}

func TestBuildDeleteRejectsUnsafeTableName(t *testing.T) {
	_, err := buildDelete("apv_form_demo; DROP TABLE x")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidGeneratedIdentifier))
}

func TestCoerceValue(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want any
	}{
		{"string", "hi", "hi"},
		{"int", 7, 7},
		{"float", 1.5, 1.5},
		{"bool", true, true},
		{"nil", nil, nil},
		{"string slice", []string{"a", "b"}, `["a","b"]`},
		{"any slice", []any{"a", 1.0}, `["a",1]`},
		{"map", map[string]any{"k": "v"}, `{"k":"v"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := coerceValue(tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
