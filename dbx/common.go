package dbx

// ColumnWithAlias returns the column prefixed with the table alias if provided.
// For example: ColumnWithAlias("name", "u") returns "u.name".
func ColumnWithAlias(column string, alias ...string) string {
	if len(alias) == 0 || alias[0] == "" {
		return column
	}

	return alias[0] + "." + column
}
