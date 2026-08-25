package scaffold

import (
	"strconv"
	"strings"
)

// goType describes the Go type a column maps to, together with the import its
// spelling needs.
type goType struct {
	// name is the value-type spelling, without a pointer.
	name string
	// importPath is the package the type comes from, empty for builtins.
	importPath string
	// bounded reports whether a declared character length should become a
	// max= validation rule.
	bounded bool
}

var (
	goString  = goType{name: "string", bounded: true}
	goInt32   = goType{name: "int32"}
	goInt64   = goType{name: "int64"}
	goFloat64 = goType{name: "float64"}
	goBool    = goType{name: "bool"}
	goTime    = goType{name: "timex.DateTime", importPath: "github.com/coldsmirk/vef-framework-go/timex"}
	goJSON    = goType{name: "json.RawMessage", importPath: "encoding/json"}
	goBytes   = goType{name: "[]byte"}
)

// typesByDeclaration maps a column's declared type, reduced to its bare name,
// onto a Go type. The table is the union of the five dialects the framework
// connects to, because a generator that only knows one of them silently
// produces string fields for the others.
var typesByDeclaration = map[string]goType{
	// Character
	"bpchar": goString, "char": goString, "character": goString,
	"character varying": goString, "citext": goString, "clob": goString,
	"enum": goString, "longtext": goString, "mediumtext": goString,
	"name": goString, "nchar": goString, "ntext": goString, "nvarchar": goString,
	"nvarchar2": goString, "text": goString, "tinytext": goString,
	"uuid": goString, "varchar": goString, "varchar2": goString,

	// Integer
	"int": goInt32, "int2": goInt32, "int4": goInt32, "integer": goInt32,
	"mediumint": goInt32, "serial": goInt32, "serial4": goInt32,
	"smallint": goInt32, "smallserial": goInt32, "tinyint": goInt32,
	"year":   goInt32,
	"bigint": goInt64, "bigserial": goInt64, "int8": goInt64, "serial8": goInt64,

	// Real
	"binary_double": goFloat64, "binary_float": goFloat64, "dec": goFloat64,
	"decimal": goFloat64, "double": goFloat64, "double precision": goFloat64,
	"float": goFloat64, "float4": goFloat64, "float8": goFloat64,
	"money": goFloat64, "number": goFloat64, "numeric": goFloat64, "real": goFloat64,
	"smallmoney": goFloat64,

	// Boolean
	"bool": goBool, "boolean": goBool,

	// Temporal
	"date": goTime, "datetime": goTime, "datetime2": goTime,
	"datetimeoffset": goTime, "smalldatetime": goTime, "time": goTime,
	"timestamp": goTime, "timestamptz": goTime, "timetz": goTime,

	// Structured and binary
	"json": goJSON, "jsonb": goJSON,
	"binary": goBytes, "blob": goBytes, "bytea": goBytes, "image": goBytes,
	"longblob": goBytes, "mediumblob": goBytes, "raw": goBytes,
	"tinyblob": goBytes, "varbinary": goBytes,
}

// resolveGoType maps a declared column type onto a Go type and the character
// bound the declaration carries, reporting whether the declaration was
// recognized. An unrecognized type resolves to string so generation still
// produces a compiling model, and the caller reports it so the developer can
// correct one field instead of losing the whole command to one exotic column.
func resolveGoType(declared string) (typ goType, maxLength int, known bool) {
	base, args := splitDeclaredType(declared)

	// MySQL spells a boolean tinyint(1); every wider tinyint is a real integer.
	if base == "tinyint" && args == "1" {
		return goBool, 0, true
	}

	typ, known = typesByDeclaration[base]
	if !known {
		return goString, 0, false
	}

	if typ.bounded {
		maxLength = parseLength(args)
	}

	return typ, maxLength, true
}

// splitDeclaredType reduces a raw declaration to its bare type name and the
// text between its parentheses: "varchar(128)" is ("varchar", "128"), and
// "int(10) unsigned" is ("int", "10"), because signedness does not change the
// framework's model vocabulary.
func splitDeclaredType(declared string) (base, args string) {
	lowered := strings.ToLower(declared)

	if open := strings.IndexByte(lowered, '('); open >= 0 {
		if end := strings.LastIndexByte(lowered, ')'); end > open {
			args = lowered[open+1 : end]
			lowered = lowered[:open] + " " + lowered[end+1:]
		}
	}

	// Collapse to single-spaced words so "int(10)  unsigned" and "int unsigned"
	// reduce identically.
	base = strings.Join(strings.Fields(lowered), " ")

	for _, modifier := range []string{" unsigned", " zerofill"} {
		base = strings.TrimSuffix(base, modifier)
	}

	// Postgres spells the qualified temporal types out in full.
	base = strings.TrimSuffix(base, " without time zone")
	if trimmed, found := strings.CutSuffix(base, " with time zone"); found {
		base = trimmed + "tz"
	}

	return base, args
}

// parseLength reads a character bound out of a declaration's arguments,
// ignoring a numeric precision pair such as "10,2".
func parseLength(args string) int {
	if args == "" || strings.Contains(args, ",") {
		return 0
	}

	length, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil || length <= 0 {
		return 0
	}

	return length
}
