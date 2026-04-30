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
