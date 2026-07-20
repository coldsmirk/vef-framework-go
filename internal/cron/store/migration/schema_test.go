package migration

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/config"
)

func TestNormalizeColumnTypeRejectsUnsignedMySQLIntegers(t *testing.T) {
	tests := []struct {
		name       string
		dataType   string
		columnType string
		signedKind schemaColumnKind
	}{
		{name: "BigInt", dataType: "bigint", columnType: "bigint unsigned", signedKind: columnInt64},
		{name: "Int", dataType: "int", columnType: "int unsigned", signedKind: columnInt32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, _ := normalizeColumnType(config.MySQL, schemaColumnRow{
				DataType:   tt.dataType,
				ColumnType: tt.columnType,
			})
			assert.NotEqual(t, tt.signedKind, kind,
				"An unsigned MySQL integer must not normalize to its signed schema kind")
		})
	}
}
