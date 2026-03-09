package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// TestStartProcessor tests start processor scenarios.
func TestStartProcessor(t *testing.T) {
	process := NewStartProcessor()

	t.Run("NodeKind", func(t *testing.T) {
		assert.Equal(t, approval.NodeStart, process.NodeKind(), "Should return NodeStart kind")
	})

	t.Run("Process", func(t *testing.T) {
		result, err := process.Process(context.Background(), nil)
		require.NoError(t, err, "Should process without error")
		assert.Equal(t, NodeActionContinue, result.Action, "Should return Continue action")
	})
}
