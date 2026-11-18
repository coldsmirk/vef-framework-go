package command

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
)

func TestValidateTitleFields(t *testing.T) {
	fields := []approval.FormFieldDefinition{{Key: "items"}}
	for _, tc := range []struct {
		name, source string
		valid        bool
	}{
		{"Known", "{{.formData.items}}", true},
		{"Missing", "{{.formData.unknown}}", false},
		{"Condition", "{{if eq .formData.missing 5}}yes{{end}}", false},
		{"Quoted", "{{printf \".formData.unknown\"}}", true},
		{"ItemScope", "{{range .formData.items}}{{.formData.itemProperty}}{{end}}", true},
		{"RootInRange", "{{range .formData.items}}{{$.formData.unknown}}{{end}}", false},
		{"CalledTemplate", "{{define \"field\"}}{{.formData.unknown}}{{end}}{{template \"field\" .}}", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTitleFields(tc.source, fields)
			if tc.valid {
				require.NoError(t, err, "Only static root form references should be checked")
			} else {
				require.ErrorIs(t, err, approval.ErrInvalidTitleTemplate, "Unknown root fields must fail deployment")
			}
		})
	}
}
