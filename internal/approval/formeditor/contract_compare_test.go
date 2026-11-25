package formeditor

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
)

func TestCompareFieldContracts(t *testing.T) {
	t.Run("IgnoresDisplayOnlyDifferences", func(t *testing.T) {
		pc := &approval.FormFieldDefinition{
			Key:         "title",
			Kind:        approval.FieldInput,
			Label:       "PC Title",
			Placeholder: "PC placeholder",
			IsRequired:  true,
			Validation:  &approval.ValidationRule{MaxLength: new(50), Message: "PC message"},
			ColumnType:  approval.ColumnString,
			SortOrder:   0,
		}
		mobile := &approval.FormFieldDefinition{
			Key:         "title",
			Kind:        approval.FieldInput,
			Label:       "Mobile Title",
			Placeholder: "Mobile placeholder",
			IsRequired:  true,
			Validation:  &approval.ValidationRule{MaxLength: new(50), Message: "Mobile message"},
			ColumnType:  approval.ColumnString,
			SortOrder:   4,
		}

		require.NoError(t, compareFieldContracts("title", pc, mobile), "display-only differences should not conflict")
	})

	t.Run("RemoteOptionsNormalizeEmptyParamsAndMapping", func(t *testing.T) {
		pc := &approval.FormFieldDefinition{
			Key:  "dept",
			Kind: approval.FieldSelect,
			OptionSource: &approval.FieldOptionSource{
				Kind: approval.OptionSourceRemote,
				Request: &approval.RemoteOptionRequest{
					Resource: "org/dept",
					Action:   "list",
					Params:   map[string]approval.DynamicParam{},
				},
				Mapping: &approval.RemoteOptionMapping{},
			},
		}
		mobile := &approval.FormFieldDefinition{
			Key:  "dept",
			Kind: approval.FieldSelect,
			OptionSource: &approval.FieldOptionSource{
				Kind: approval.OptionSourceRemote,
				Request: &approval.RemoteOptionRequest{
					Resource: "org/dept",
					Action:   "list",
				},
			},
		}

		require.NoError(t, compareFieldContracts("dept", pc, mobile), "empty params and empty mapping should match nil")
	})

	t.Run("RemoteOptionsDetectParamDivergence", func(t *testing.T) {
		pc := &approval.FormFieldDefinition{
			Key:  "dept",
			Kind: approval.FieldSelect,
			OptionSource: &approval.FieldOptionSource{
				Kind: approval.OptionSourceRemote,
				Request: &approval.RemoteOptionRequest{
					Resource: "org/dept",
					Action:   "list",
					Params: map[string]approval.DynamicParam{
						"orgId": {Kind: approval.DynamicParamExpression, Source: "formData.org"},
					},
				},
			},
		}
		mobile := &approval.FormFieldDefinition{
			Key:  "dept",
			Kind: approval.FieldSelect,
			OptionSource: &approval.FieldOptionSource{
				Kind: approval.OptionSourceRemote,
				Request: &approval.RemoteOptionRequest{
					Resource: "org/dept",
					Action:   "list",
					Params: map[string]approval.DynamicParam{
						"orgId": {Kind: approval.DynamicParamExpression, Source: "formData.otherOrg"},
					},
				},
			},
		}

		err := compareFieldContracts("dept", pc, mobile)
		require.ErrorIs(t, err, approval.ErrInvalidFormDesign, "divergent remote params must fail")
		require.Contains(t, err.Error(), "dept", "error should name field key")
	})

	t.Run("RemoteOptionsCompareReferenceParamsByPath", func(t *testing.T) {
		withReference := func(path string) *approval.FormFieldDefinition {
			return &approval.FormFieldDefinition{
				Key:  "dept",
				Kind: approval.FieldSelect,
				OptionSource: &approval.FieldOptionSource{
					Kind: approval.OptionSourceRemote,
					Request: &approval.RemoteOptionRequest{
						Resource: "org/dept",
						Action:   "list",
						Params: map[string]approval.DynamicParam{
							"orgId": {Kind: approval.DynamicParamReference, Path: path},
						},
					},
				},
			}
		}

		require.NoError(t, compareFieldContracts("dept", withReference("org"), withReference("org")), "one path is one contract")
		require.ErrorIs(t, compareFieldContracts("dept", withReference("org"), withReference("$applicant.orgId")), approval.ErrInvalidFormDesign,
			"references reading different paths must fail")
	})

	t.Run("NumericOptionValuesCompareEqualAcrossNumericTypesButNotStrings", func(t *testing.T) {
		pc := &approval.FormFieldDefinition{
			Key:  "grade",
			Kind: approval.FieldSelect,
			Options: []approval.FieldOption{
				{Label: "One", Value: json.Number("1")},
			},
		}
		sameNum := &approval.FormFieldDefinition{
			Key:  "grade",
			Kind: approval.FieldSelect,
			Options: []approval.FieldOption{
				{Label: "One", Value: float64(1)},
			},
		}
		stringVal := &approval.FormFieldDefinition{
			Key:  "grade",
			Kind: approval.FieldSelect,
			Options: []approval.FieldOption{
				{Label: "One", Value: "1"},
			},
		}

		require.NoError(t, compareFieldContracts("grade", pc, sameNum), "equivalent numeric option values should match")
		require.ErrorIs(t, compareFieldContracts("grade", pc, stringVal), approval.ErrInvalidFormDesign, "number and string option values must not match")
	})

	t.Run("ValidationMessageOnlyDiffersFromNilRule", func(t *testing.T) {
		pc := &approval.FormFieldDefinition{
			Key:        "score",
			Kind:       approval.FieldNumber,
			Validation: &approval.ValidationRule{Message: "custom hint"},
		}
		mobile := &approval.FormFieldDefinition{
			Key:  "score",
			Kind: approval.FieldNumber,
		}
		bounded := &approval.FormFieldDefinition{
			Key:        "score",
			Kind:       approval.FieldNumber,
			Validation: &approval.ValidationRule{Min: new(float64(0))},
		}

		require.NoError(t, compareFieldContracts("score", pc, mobile), "message-only rule should equal nil rule")
		require.ErrorIs(t, compareFieldContracts("score", pc, bounded), approval.ErrInvalidFormDesign, "min bound divergence must fail")
	})

	t.Run("FixedTableIgnoresHeaderLabelAndColumnOrder", func(t *testing.T) {
		pc := &approval.FormFieldDefinition{
			Key:        "checks",
			Kind:       approval.FieldTable,
			IsRequired: true,
			FixedRows: &approval.TableFixedRows{
				ItemKey:     "item",
				HeaderLabel: "PC Header",
				Items:       []approval.FieldOption{{Label: "Row 1", Value: "1"}},
			},
			Columns: []approval.FormFieldDefinition{
				{Key: "a", Kind: approval.FieldInput, ColumnType: approval.ColumnText},
				{Key: "b", Kind: approval.FieldNumber, ColumnType: approval.ColumnDecimal, Scale: new(2)},
			},
		}
		mobile := &approval.FormFieldDefinition{
			Key:        "checks",
			Kind:       approval.FieldTable,
			IsRequired: true,
			FixedRows: &approval.TableFixedRows{
				ItemKey:     "item",
				HeaderLabel: "Mobile Header",
				Items:       []approval.FieldOption{{Label: "Row 1", Value: "1"}},
			},
			Columns: []approval.FormFieldDefinition{
				{Key: "b", Kind: approval.FieldNumber, ColumnType: approval.ColumnDecimal, Scale: new(2)},
				{Key: "a", Kind: approval.FieldInput, ColumnType: approval.ColumnText},
			},
		}

		require.NoError(t, compareFieldContracts("checks", pc, mobile), "different headerLabel and column order should match")
	})

	t.Run("StorageMismatchReportsColumnAndScale", func(t *testing.T) {
		pc := &approval.FormFieldDefinition{
			Key:        "files",
			Kind:       approval.FieldUpload,
			ColumnType: approval.ColumnJSON,
		}
		mobile := &approval.FormFieldDefinition{
			Key:        "files",
			Kind:       approval.FieldUpload,
			ColumnType: approval.ColumnText,
		}

		err := compareFieldContracts("files", pc, mobile)
		require.ErrorIs(t, err, approval.ErrInvalidFormDesign, "storage column mismatch must fail")
		require.Contains(t, err.Error(), "files", "error should name field key")
		require.Contains(t, err.Error(), "json", "error should name pc column type")
		require.Contains(t, err.Error(), "text", "error should name mobile column type")
	})
}
