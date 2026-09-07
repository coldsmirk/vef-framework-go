package approval_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
)

func TestRemoteOptionRequestHasBoundParams(t *testing.T) {
	literal := func(value any) approval.DynamicParam {
		return approval.DynamicParam{Kind: approval.DynamicParamLiteral, Value: value}
	}

	tests := []struct {
		name     string
		request  *approval.RemoteOptionRequest
		expected bool
	}{
		{
			// A parser can emit a descriptor with no request (a broken designer
			// document that deploy validation then rejects), and a host calling
			// the parser directly reaches this before that check runs.
			name:     "NilRequest",
			request:  nil,
			expected: false,
		},
		{
			name:     "NoParams",
			request:  &approval.RemoteOptionRequest{Resource: "city", Action: "list"},
			expected: false,
		},
		{
			name: "OnlyLiterals",
			request: &approval.RemoteOptionRequest{
				Resource: "city", Action: "list",
				Params: map[string]approval.DynamicParam{"level": literal(2), "enabled": literal(true)},
			},
			expected: false,
		},
		{
			name: "OneExpressionAmongLiterals",
			request: &approval.RemoteOptionRequest{
				Resource: "ward", Action: "list",
				Params: map[string]approval.DynamicParam{
					"active": literal(true),
					"deptId": {Kind: approval.DynamicParamExpression, Source: "formData.dept"},
				},
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.request.HasBoundParams(),
				"HasBoundParams decides whether one lookup can translate a whole column")
		})
	}
}

func TestDynamicParamRoundTrip(t *testing.T) {
	// Params are persisted onto apv_flow_version.form_fields, so a literal the
	// designer chose must survive the marshal — an omitempty on Value would drop
	// exactly the falsy ones and silently change the request.
	falsy := []any{false, float64(0), ""}

	for _, value := range falsy {
		params := map[string]approval.DynamicParam{
			"p": {Kind: approval.DynamicParamLiteral, Value: value},
		}

		encoded, err := json.Marshal(params)
		require.NoError(t, err, "a literal parameter must marshal")

		var decoded map[string]approval.DynamicParam
		require.NoError(t, json.Unmarshal(encoded, &decoded), "a literal parameter must unmarshal")

		assert.Equal(t, value, decoded["p"].Value, "a falsy literal %#v must survive the round trip", value)
	}
}
