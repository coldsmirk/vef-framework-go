package voiding

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
)

func TestRequestViewPermissions(t *testing.T) {
	vc := &approval.VoidContext{
		Instance: &approval.Instance{
			ID:                      "target",
			InstanceNo:              "N1",
			Title:                   "Private title",
			BusinessKind:            new(approval.BusinessKind("booking")),
			ApplicantID:             "person",
			ApplicantDepartmentID:   new("dept"),
			ApplicantDepartmentName: new("Dept"),
		},
		Flow: &approval.Flow{Name: "Target flow"},
	}
	record := &approval.VoidRecord{InstanceID: "target", Reason: "private reason"}
	render := func(t *testing.T, perms map[string]approval.Permission) (map[string]any, map[string]any) {
		t.Helper()

		raw, err := json.Marshal(makeRequestView(vc, record, &approval.VoidPlan{}, perms))
		require.NoError(t, err, "Should encode business view")

		var v map[string]any
		require.NoError(t, json.Unmarshal(raw, &v), "Should decode rendered payload")
		require.Contains(t, v, "plan", "The plan is always available")

		return v["target"].(map[string]any), v["record"].(map[string]any)
	}
	t.Run("Admin", func(t *testing.T) {
		target, rec := render(t, nil)
		for _, k := range []string{"instanceId", "instanceNo", "title", "flowName", "businessKind", "applicant"} {
			require.Contains(t, target, k, "Admin sees %s", k)
		}

		require.Equal(t, "private reason", rec["reason"], "Admin sees the recorded reason")
		require.Equal(t, "target", rec["instanceId"], "Admin sees the recorded target")
		require.Contains(t, target["applicant"].(map[string]any), "departmentId", "Admin sees the applicant department")
	})

	for _, tc := range []struct {
		field  string
		target []string
		record []string
	}{
		{"targetInstanceId", []string{"instanceId"}, []string{"instanceId"}},
		{"targetInstanceNo", []string{"instanceNo"}, nil},
		{"targetTitle", []string{"title"}, nil},
		{"targetFlowCode", []string{"flowName", "businessKind"}, nil},
		{"targetApplicantId", []string{"applicant"}, nil},
		{"reason", nil, []string{"reason"}},
	} {
		t.Run(tc.field, func(t *testing.T) {
			target, rec := render(t, map[string]approval.Permission{tc.field: approval.PermissionHidden})
			for _, k := range tc.target {
				require.NotContains(t, target, k, "Hidden target values are omitted")
			}

			for _, k := range tc.record {
				require.NotContains(t, rec, k, "The record must not repeat hidden values")
			}
		})
	}

	t.Run("targetApplicantDepartmentId", func(t *testing.T) {
		target, _ := render(t, map[string]approval.Permission{"targetApplicantDepartmentId": approval.PermissionHidden})
		person := target["applicant"].(map[string]any)
		require.Equal(t, "person", person["id"], "The applicant remains visible")
		require.NotContains(t, person, "departmentId", "The department identifier is hidden")
		require.NotContains(t, person, "departmentName", "The department name is hidden")
	})
}
