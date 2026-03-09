package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// TestEndProcessor tests end processor scenarios.
func TestEndProcessor(t *testing.T) {
	processor := NewEndProcessor()

	t.Run("NodeKind", func(t *testing.T) {
		assert.Equal(t, approval.NodeEnd, processor.NodeKind(), "Should return NodeEnd kind")
	})

	t.Run("Process", func(t *testing.T) {
		result, err := processor.Process(context.Background(), nil)
		require.NoError(t, err, "Should process without error")
		assert.Equal(t, NodeActionComplete, result.Action, "Should return Complete action")
		require.NotNil(t, result.FinalStatus, "Should set final status")
		assert.Equal(t, approval.InstanceApproved, *result.FinalStatus, "Should set final status to Approved")
	})
}
