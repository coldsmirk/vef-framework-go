package id

import (
	"fmt"
	"sync"

	"github.com/bwmarrin/snowflake"
)

// configureSnowflake applies the framework's Snowflake layout exactly once:
//   - Epoch: 1754582400000 (custom start time)
//   - Node bits: 6 (supports 64 nodes: 0-63)
//   - Step bits: 12 (supports 4096 IDs per millisecond per node)
//
// The upstream library only offers package-level configuration, so this is
// deliberately deferred to the first explicit constructor call instead of an
// import-time init: importing this package must never mutate process-global
// state or panic on a misconfigured environment.
var configureSnowflake = sync.OnceFunc(func() {
	snowflake.Epoch = 1754582400000
	snowflake.NodeBits = 6
	snowflake.StepBits = 12
})

// snowflakeIDGenerator implements IDGenerator using the Snowflake algorithm.
type snowflakeIDGenerator struct {
	node *snowflake.Node
}

// Generate creates a new Snowflake ID encoded as a Base36 string.
func (g *snowflakeIDGenerator) Generate() string {
	return g.node.Generate().Base36()
}

// NewSnowflakeIDGenerator creates a new Snowflake ID generator for the
// specified node. The nodeID must be between 0 and 63 (6-bit layout); an
// out-of-range id returns an error rather than panicking, so callers decide
// how a misconfigured VEF_NODE_ID surfaces. Each node in a distributed
// system should have a unique nodeID to ensure global uniqueness.
func NewSnowflakeIDGenerator(nodeID int64) (_ IDGenerator, err error) {
	configureSnowflake()

	var node *snowflake.Node
	if node, err = snowflake.NewNode(nodeID); err != nil {
		return nil, fmt.Errorf("failed to create snowflake node: %w", err)
	}

	return &snowflakeIDGenerator{
		node: node,
	}, nil
}
