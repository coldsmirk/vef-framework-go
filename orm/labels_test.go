package orm_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/orm"
)

// TestValidateLabels pins the shared label rules every LabelsEqual writer
// relies on: JSON-path-safe keys, size bounds, and empty values as legitimate
// presence-style flags.
func TestValidateLabels(t *testing.T) {
	tests := []struct {
		name    string
		labels  map[string]string
		wantErr bool
	}{
		{name: "NilLabelsPass", labels: nil},
		{name: "ValidLabelsPass", labels: map[string]string{"scene": "inspection", "app-1": "smp_web"}},
		{name: "SingleCharKeyPasses", labels: map[string]string{"a": "x"}},
		{name: "EmptyValuePasses", labels: map[string]string{"mobile": ""}},
		{name: "MaxLengthsPass", labels: map[string]string{strings.Repeat("k", 63): strings.Repeat("值", 256)}},
		{name: "EmptyKeyFails", labels: map[string]string{"": "x"}, wantErr: true},
		{name: "DottedKeyFails", labels: map[string]string{"a.b": "x"}, wantErr: true},
		{name: "NonASCIIKeyFails", labels: map[string]string{"场景": "x"}, wantErr: true},
		{name: "EdgeDashKeyFails", labels: map[string]string{"-lead": "x"}, wantErr: true},
		{name: "OverlongKeyFails", labels: map[string]string{strings.Repeat("k", 64): "x"}, wantErr: true},
		{name: "OverlongValueFails", labels: map[string]string{"scene": strings.Repeat("值", 257)}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := orm.ValidateLabels(tt.labels)

			if tt.wantErr {
				assert.ErrorIs(t, err, orm.ErrInvalidLabel, "Invalid labels should be rejected with the sentinel")

				return
			}

			assert.NoError(t, err, "Valid labels should pass")
		})
	}
}
