package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/mold"
	"github.com/coldsmirk/vef-framework-go/result"
)

// recordingLoader serves fixed dictionaries and counts the loads of each.
type recordingLoader struct {
	dictionaries map[string]map[string]string
	loads        map[string]int
	err          error
}

func (l *recordingLoader) Load(_ context.Context, codeSet string) (map[string]string, error) {
	l.loads[codeSet]++

	if l.err != nil {
		return nil, l.err
	}

	return l.dictionaries[codeSet], nil
}

func newRecordingLoader() *recordingLoader {
	return &recordingLoader{
		dictionaries: map[string]map[string]string{
			"sys.gender": {"M": "男", "F": "女"},
			"hr.skill":   {"go": "Go", "ts": "TypeScript"},
			"common.region": {
				"east": "华东大区",
				"zj":   "浙江省",
				"hz":   "杭州市",
				"sh":   "上海市",
			},
		},
		loads: make(map[string]int),
	}
}

func codeSetSelect(key, codeSet string) approval.FormFieldDefinition {
	return approval.FormFieldDefinition{
		Key: key, Kind: approval.FieldSelect, Label: key,
		OptionSource: &approval.FieldOptionSource{Kind: approval.OptionSourceCodeSet, CodeSet: codeSet},
	}
}

func requireFormValidationError(t *testing.T, err error) {
	t.Helper()

	var re result.Error
	require.ErrorAs(t, err, &re, "Should return business error")
	assert.Equal(t, approval.ErrCodeFormValidationFailed, re.Code, "Should return form validation error code")
}

func TestValidateFormDataCodeSet(t *testing.T) {
	fields := []approval.FormFieldDefinition{codeSetSelect("gender", "sys.gender"), codeSetSelect("skills", "hr.skill")}

	newService := func(loader mold.CodeSetLoader) *ValidationService {
		return NewValidationService(mustInitiatorCompositeInternal(nil), WithCodeSetLoader(loader))
	}

	t.Run("AcceptsCodesTheDictionaryHolds", func(t *testing.T) {
		err := newService(newRecordingLoader()).ValidateFormData(t.Context(), fields, map[string]any{
			"gender": "M",
			"skills": []any{"go", "ts"},
		})
		assert.NoError(t, err, "Should accept codes of the field's dictionary")
	})

	t.Run("RejectsACodeTheDictionaryDoesNotHold", func(t *testing.T) {
		err := newService(newRecordingLoader()).ValidateFormData(t.Context(), fields, map[string]any{"gender": "X"})
		requireFormValidationError(t, err)
	})

	t.Run("ChecksEveryItemOfAMultipleSelection", func(t *testing.T) {
		err := newService(newRecordingLoader()).ValidateFormData(t.Context(), fields, map[string]any{"skills": []string{"go", "rust"}})
		requireFormValidationError(t, err)
	})

	t.Run("ChecksADetailTableColumn", func(t *testing.T) {
		table := []approval.FormFieldDefinition{{
			Key: "members", Kind: approval.FieldTable, Label: "members",
			Columns: []approval.FormFieldDefinition{codeSetSelect("gender", "sys.gender")},
		}}

		err := newService(newRecordingLoader()).ValidateFormData(t.Context(), table, map[string]any{
			"members": []any{map[string]any{"gender": "M"}, map[string]any{"gender": "X"}},
		})
		requireFormValidationError(t, err)
	})

	t.Run("LoadsEachDictionaryOnce", func(t *testing.T) {
		loader := newRecordingLoader()
		twice := []approval.FormFieldDefinition{codeSetSelect("gender", "sys.gender"), codeSetSelect("spouseGender", "sys.gender")}

		err := newService(loader).ValidateFormData(t.Context(), twice, map[string]any{"gender": "M", "spouseGender": "F"})
		require.NoError(t, err, "Should accept codes of the field's dictionary")
		assert.Equal(t, 1, loader.loads["sys.gender"], "Should load a dictionary shared by two fields once")
	})

	t.Run("SkipsEmptyValues", func(t *testing.T) {
		loader := newRecordingLoader()

		err := newService(loader).ValidateFormData(t.Context(), fields, map[string]any{"gender": ""})
		require.NoError(t, err, "An empty value is the required check's concern, not the dictionary's")
		assert.Empty(t, loader.loads, "Should load no dictionary for an empty value")
	})

	t.Run("AcceptsAnyValueWithoutALoader", func(t *testing.T) {
		err := newService(nil).ValidateFormData(t.Context(), fields, map[string]any{"gender": "X"})
		assert.NoError(t, err, "Without a host loader a code set value is accepted as a remote one is")
	})

	t.Run("SurfacesALoaderFailure", func(t *testing.T) {
		loader := newRecordingLoader()
		loader.err = errors.New("dictionary store unavailable")

		err := newService(loader).ValidateFormData(t.Context(), fields, map[string]any{"gender": "M"})
		require.ErrorIs(t, err, loader.err, "Should fail the submission when the dictionary cannot be read")

		var re result.Error
		assert.NotErrorAs(t, err, &re, "A load failure is not the submitter's validation error")
	})

	t.Run("AcceptsHierarchicalCodeSetScalarAndMultiWithoutEnforcingLeafOnly", func(t *testing.T) {
		treeFields := []approval.FormFieldDefinition{
			codeSetSelect("region", "common.region"),
			{
				Key:          "regions",
				Kind:         approval.FieldSelect,
				Label:        "覆盖区域",
				IsMultiple:   true,
				OptionSource: &approval.FieldOptionSource{Kind: approval.OptionSourceCodeSet, CodeSet: "common.region"},
			},
		}

		// Both branch ("east") and leaf ("hz", "sh") codes are valid because
		// backend validation checks against the whole dictionary without leafOnly.
		err := newService(newRecordingLoader()).ValidateFormData(t.Context(), treeFields, map[string]any{
			"region":  "east",
			"regions": []any{"east", "hz", "sh"},
		})
		require.NoError(t, err, "Should accept both branch and leaf codes from a hierarchical code set")

		err = newService(newRecordingLoader()).ValidateFormData(t.Context(), treeFields, map[string]any{
			"regions": []any{"hz", "unknown_region"},
		})
		requireFormValidationError(t, err)
	})
}

func TestValidateFormDataFixedRowsCodeSet(t *testing.T) {
	newService := func(loader mold.CodeSetLoader) *ValidationService {
		return NewValidationService(mustInitiatorCompositeInternal(nil), WithCodeSetLoader(loader))
	}

	fields := []approval.FormFieldDefinition{{
		Key: "checks", Kind: approval.FieldTable, Label: "自查表", IsRequired: true,
		Columns: []approval.FormFieldDefinition{
			codeSetSelect("gender", "sys.gender"),
			{Key: "note", Kind: approval.FieldInput, Label: "说明"},
		},
		FixedRows: &approval.TableFixedRows{
			ItemKey:     "item",
			HeaderLabel: "自查内容",
			Source:      &approval.FieldOptionSource{Kind: approval.OptionSourceCodeSet, CodeSet: "hr.skill"},
		},
	}}

	t.Run("AcceptsTopLevelDictionaryCodesAndChecksColumns", func(t *testing.T) {
		loader := newRecordingLoader()
		err := newService(loader).ValidateFormData(t.Context(), fields, map[string]any{
			"checks": []any{
				map[string]any{"item": map[string]any{"value": "go", "label": "Go"}, "gender": "M", "note": "ok"},
				map[string]any{"item": map[string]any{"value": "ts", "label": "TypeScript"}, "gender": "F", "note": "ok"},
			},
		})
		require.NoError(t, err, "top-level dictionary codes in fixed rows must pass")
		assert.Equal(t, 1, loader.loads["hr.skill"], "dictionary should be loaded once per call")
		assert.Equal(t, 1, loader.loads["sys.gender"], "column dictionary should be loaded once per call")
	})

	t.Run("RejectsCodeNotInDictionary", func(t *testing.T) {
		err := newService(newRecordingLoader()).ValidateFormData(t.Context(), fields, map[string]any{
			"checks": []any{
				map[string]any{"item": map[string]any{"value": "go", "label": "Go"}},
				map[string]any{"item": map[string]any{"value": "rust", "label": "Rust"}},
			},
		})
		requireFormValidationError(t, err)
	})

	t.Run("RejectsUnknownEmptyDictionary", func(t *testing.T) {
		unknownFields := []approval.FormFieldDefinition{{
			Key: "checks", Kind: approval.FieldTable, Label: "自查表",
			Columns: []approval.FormFieldDefinition{{Key: "note", Kind: approval.FieldInput}},
			FixedRows: &approval.TableFixedRows{
				ItemKey: "item",
				Source:  &approval.FieldOptionSource{Kind: approval.OptionSourceCodeSet, CodeSet: "missing.dict"},
			},
		}}
		err := newService(newRecordingLoader()).ValidateFormData(t.Context(), unknownFields, map[string]any{
			"checks": []any{map[string]any{"item": map[string]any{"value": "a", "label": "A"}}},
		})
		requireFormValidationError(t, err)
	})

	t.Run("RejectsDuplicateDictionaryValues", func(t *testing.T) {
		err := newService(newRecordingLoader()).ValidateFormData(t.Context(), fields, map[string]any{
			"checks": []any{
				map[string]any{"item": map[string]any{"value": "go", "label": "Go"}},
				map[string]any{"item": map[string]any{"value": "go", "label": "Go"}},
			},
		})
		requireFormValidationError(t, err)
	})

	t.Run("AcceptsWithoutLoader", func(t *testing.T) {
		err := newService(nil).ValidateFormData(t.Context(), fields, map[string]any{
			"checks": []any{
				map[string]any{"item": map[string]any{"value": "anything", "label": "Any"}},
			},
		})
		assert.NoError(t, err, "without a CodeSetLoader any unique valid item shape is accepted")
	})

	t.Run("SurfacesLoaderFailure", func(t *testing.T) {
		loader := newRecordingLoader()
		loader.err = errors.New("dictionary store unavailable")

		err := newService(loader).ValidateFormData(t.Context(), fields, map[string]any{
			"checks": []any{map[string]any{"item": map[string]any{"value": "go", "label": "Go"}}},
		})
		require.ErrorIs(t, err, loader.err, "loader errors must surface as system errors")

		var re result.Error
		assert.NotErrorAs(t, err, &re, "a load failure is not the submitter's validation error")
	})
}
