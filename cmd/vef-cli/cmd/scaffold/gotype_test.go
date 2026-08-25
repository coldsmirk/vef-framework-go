package scaffold

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveGoType(t *testing.T) {
	cases := []struct {
		name      string
		declared  string
		wantType  string
		wantMax   int
		wantKnown bool
	}{
		{name: "BoundedVarchar", declared: "varchar(128)", wantType: "string", wantMax: 128, wantKnown: true},
		{name: "UnboundedText", declared: "text", wantType: "string", wantKnown: true},
		{name: "PostgresBigint", declared: "int8", wantType: "int64", wantKnown: true},
		{name: "Integer", declared: "int4", wantType: "int32", wantKnown: true},
		{name: "NumericPrecisionIsNotALength", declared: "numeric(10,2)", wantType: "float64", wantKnown: true},
		{name: "Boolean", declared: "bool", wantType: "bool", wantKnown: true},
		{name: "MySQLBooleanIsTinyintOne", declared: "tinyint(1)", wantType: "bool", wantKnown: true},
		{name: "WiderTinyintIsAnInteger", declared: "tinyint(4)", wantType: "int32", wantKnown: true},
		{name: "MySQLUnsigned", declared: "int(10) unsigned", wantType: "int32", wantKnown: true},
		{name: "BareUnsigned", declared: "bigint unsigned", wantType: "int64", wantKnown: true},
		{name: "Timestamp", declared: "timestamptz", wantType: "timex.DateTime", wantKnown: true},
		{name: "PostgresSpelledOutTimestamp", declared: "timestamp without time zone", wantType: "timex.DateTime", wantKnown: true},
		{name: "PostgresSpelledOutTimestampTZ", declared: "timestamp with time zone", wantType: "timex.DateTime", wantKnown: true},
		{name: "JSONB", declared: "jsonb", wantType: "json.RawMessage", wantKnown: true},
		{name: "Bytes", declared: "bytea", wantType: "[]byte", wantKnown: true},
		{name: "UppercaseDeclaration", declared: "VARCHAR(64)", wantType: "string", wantMax: 64, wantKnown: true},
		{name: "UnknownFallsBackToStringAndReportsIt", declared: "geography", wantType: "string", wantKnown: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			typ, maxLength, known := resolveGoType(tc.declared)

			assert.Equal(t, tc.wantType, typ.name, "%q must map to the framework's model vocabulary", tc.declared)
			assert.Equal(t, tc.wantMax, maxLength, "%q must yield the character bound the declaration carries", tc.declared)
			assert.Equal(t, tc.wantKnown, known, "%q must report whether the declaration was recognized", tc.declared)
		})
	}
}
