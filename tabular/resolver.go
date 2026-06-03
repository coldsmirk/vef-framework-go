package tabular

// ResolveFormatter selects the Formatter to use for a column.
//
// Precedence:
//  1. Column.FormatterFn (direct instance override)
//  2. Column.Formatter (name lookup in registry)
//  3. NewDefaultFormatter(Column.Format)
//
// If the name is set but not present in the registry, it is logged and the
// default formatter is returned so exports keep working.
func ResolveFormatter(col *Column, registry map[string]Formatter) Formatter {
	if col.FormatterFn != nil {
		return col.FormatterFn
	}

	if col.Formatter != "" {
		if formatter, ok := registry[col.Formatter]; ok {
			return formatter
		}

		logger.Warnf("Formatter %s not found, using default formatter", col.Formatter)
	}

	return NewDefaultFormatter(col.Format)
}

// ResolveParser selects the ValueParser to use for a column using the same
// precedence rules as ResolveFormatter.
func ResolveParser(col *Column, registry map[string]ValueParser) ValueParser {
	if col.ParserFn != nil {
		return col.ParserFn
	}

	if col.Parser != "" {
		if parser, ok := registry[col.Parser]; ok {
			return parser
		}

		logger.Warnf("Parser %s not found, using default parser", col.Parser)
	}

	return NewDefaultParser(col.Format)
}

// IsDefaultFormatter reports whether a column resolves to the built-in default
// formatter (i.e. no FormatterFn and no registered named Formatter). It applies
// the same precedence as ResolveFormatter so callers can branch on typed vs
// string output without re-implementing the lookup.
func IsDefaultFormatter(col *Column, registry map[string]Formatter) bool {
	if col.FormatterFn != nil {
		return false
	}

	if col.Formatter != "" {
		if _, ok := registry[col.Formatter]; ok {
			return false
		}
	}

	return true
}

// ResolveFormatters resolves a Formatter for every column once, returning a
// slice aligned with schema.Columns(). Resolution is loop-invariant, so callers
// resolve here before the per-row loop instead of per cell.
func ResolveFormatters(schema *Schema, registry map[string]Formatter) []Formatter {
	columns := schema.Columns()

	formatters := make([]Formatter, len(columns))
	for i, column := range columns {
		formatters[i] = ResolveFormatter(column, registry)
	}

	return formatters
}

// ResolveParsers resolves a ValueParser for every column once, returning a
// slice aligned with schema.Columns(). Resolution is loop-invariant, so callers
// resolve here before the per-row loop instead of per cell.
func ResolveParsers(schema *Schema, registry map[string]ValueParser) []ValueParser {
	columns := schema.Columns()

	parsers := make([]ValueParser, len(columns))
	for i, column := range columns {
		parsers[i] = ResolveParser(column, registry)
	}

	return parsers
}
