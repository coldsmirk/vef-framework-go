package approval_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// TestNodeDefinitionParseData_HappyPath verifies that ParseData dispatches
// correctly to the right NodeData type for every supported NodeKind.
func TestNodeDefinitionParseDataHappyPath(t *testing.T) {
	tests := []struct {
		kind     approval.NodeKind
		data     map[string]any
		wantKind approval.NodeKind
	}{
		{
			kind:     approval.NodeStart,
			data:     map[string]any{"name": "Start"},
			wantKind: approval.NodeStart,
		},
		{
			kind:     approval.NodeEnd,
			data:     map[string]any{"name": "End"},
			wantKind: approval.NodeEnd,
		},
		{
			kind:     approval.NodeApproval,
			data:     map[string]any{"name": "Review", "approvalMethod": "parallel"},
			wantKind: approval.NodeApproval,
		},
		{
			kind:     approval.NodeHandle,
			data:     map[string]any{"name": "Handle"},
			wantKind: approval.NodeHandle,
		},
		{
			kind:     approval.NodeCC,
			data:     map[string]any{"name": "CC"},
			wantKind: approval.NodeCC,
		},
		{
			kind:     approval.NodeCondition,
			data:     map[string]any{"name": "Branch"},
			wantKind: approval.NodeCondition,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			raw, err := json.Marshal(tt.data)
			require.NoError(t, err, "test setup: failed to marshal node data")

			nd := approval.NodeDefinition{
				ID:   "n1",
				Kind: tt.kind,
				Data: raw,
			}
			got, err := nd.ParseData()

			require.NoError(t, err, "ParseData should succeed for kind %q", tt.kind)
			require.NotNil(t, got, "ParseData should return non-nil NodeData for kind %q", tt.kind)
			assert.Equal(t, tt.wantKind, got.Kind(), "Returned NodeData should report its Kind as %q", tt.wantKind)
		})
	}
}

// TestNodeDefinitionParseDataEmptyData verifies that missing Data is handled
// gracefully — the typed struct is returned with zero values.
func TestNodeDefinitionParseDataEmptyData(t *testing.T) {
	nd := approval.NodeDefinition{
		ID:   "n1",
		Kind: approval.NodeStart,
		Data: nil,
	}
	got, err := nd.ParseData()

	require.NoError(t, err, "ParseData with nil Data should not error")
	require.NotNil(t, got, "ParseData with nil Data should return non-nil NodeData")
	assert.Equal(t, approval.NodeStart, got.Kind(), "Returned NodeData kind should match")
	assert.Empty(t, got.GetName(), "Name should be empty when Data is nil")
}

// TestNodeDefinitionParseDataUnknownKind verifies the default-branch sentinel.
func TestNodeDefinitionParseDataUnknownKind(t *testing.T) {
	nd := approval.NodeDefinition{
		ID:   "n1",
		Kind: approval.NodeKind("unknown_kind"),
	}
	got, err := nd.ParseData()

	require.Error(t, err, "ParseData should error for unknown kind")
	assert.Nil(t, got, "ParseData should return nil NodeData for unknown kind")
	assert.True(t, errors.Is(err, approval.ErrUnknownNodeKind),
		"Error should wrap ErrUnknownNodeKind for unknown kind")
}

// TestNodeDefinitionParseDataMalformedJSON verifies the unmarshal error path.
func TestNodeDefinitionParseDataMalformedJSON(t *testing.T) {
	nd := approval.NodeDefinition{
		ID:   "n1",
		Kind: approval.NodeApproval,
		Data: json.RawMessage(`{"approvalMethod": 123}`), // number where string expected
	}
	got, err := nd.ParseData()

	require.Error(t, err, "ParseData should error on malformed node data")
	assert.Nil(t, got, "ParseData should return nil on unmarshal error")
	assert.True(t, errors.Is(err, approval.ErrNodeDataUnmarshal),
		"Error should wrap ErrNodeDataUnmarshal for bad payload")
}
