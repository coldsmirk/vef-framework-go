package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
)

func TestFixedCountAppearsInInstanceViews(t *testing.T) {
	bundle := linearBundle()
	bundle.FlowNodes[1].PassRule = approval.PassFixedCount
	bundle.FlowNodes[1].PassCount = 3
	bundle.Visits = []approval.NodeVisit{visit("v1", "na", 1, approval.NodeVisitActive)}

	graph := buildInstanceFlowGraph(bundle)
	approvalNode := nodesByKey(graph)["kappr"]
	require.NotNil(t, approvalNode.Data.PassCount, "Flow graph should show the fixed threshold")
	assert.Equal(t, 3, *approvalNode.Data.PassCount, "Flow graph should show the configured count")
	assert.Nil(t, approvalNode.Data.PassRatio, "Fixed-count node should not show a ratio")

	timeline := buildInstanceTimeline(bundle)
	require.Len(t, timeline, 1, "The active approval visit should appear in the timeline")
	require.NotNil(t, timeline[0].PassCount, "Timeline should show the fixed threshold")
	assert.Equal(t, 3, *timeline[0].PassCount, "Timeline should show the configured count")
	assert.Nil(t, timeline[0].PassRatio, "Fixed-count visit should not show a ratio")
}
