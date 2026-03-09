package id

import (
	"fmt"
	"os"

	"github.com/bwmarrin/snowflake"
	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/config"
)

// DefaultSnowflakeIDGenerator is the default Snowflake ID generator instance.
var DefaultSnowflakeIDGenerator IDGenerator

// init initializes the Snowflake algorithm with custom configuration and creates the default generator.
// Configuration:
//   - Epoch: 1754582400000 (custom start time)
//   - Node bits: 6 (supports 64 nodes: 0-63)
//   - Step bits: 12 (supports 4096 IDs per millisecond per node)
func init() {
	snowflake.Epoch = 1754582400000
	snowflake.NodeBits = 6
	snowflake.StepBits = 12

	var nodeID int64
	if nodeIDStr := os.Getenv(config.EnvNodeID); nodeIDStr != "" {
		var err error
		if nodeID, err = cast.ToInt64E(nodeIDStr); err != nil {
			panic(fmt.Errorf("failed to convert node ID to int: %w", err))
		}
	}

	var err error
	if DefaultSnowflakeIDGenerator, err = NewSnowflakeIDGenerator(nodeID); err != nil {
		panic(err)
	}
}

// snowflakeIDGenerator implements IDGenerator using the Snowflake algorithm.
type snowflakeIDGenerator struct {
	node *snowflake.Node
}

// Generate creates a new Snowflake ID encoded as a Base36 string.
func (g *snowflakeIDGenerator) Generate() string {
	return g.node.Generate().Base36()
}

// NewSnowflakeIDGenerator creates a new Snowflake ID generator for the specified node.
// The nodeID must be between 0 and 63 (6-bit limit as configured in init).
// Each node in a distributed system should have a unique nodeID to ensure global uniqueness.
func NewSnowflakeIDGenerator(nodeID int64) (_ IDGenerator, err error) {
	var node *snowflake.Node
	if node, err = snowflake.NewNode(nodeID); err != nil {
		return nil, fmt.Errorf("failed to create snowflake node: %w", err)
	}

	return &snowflakeIDGenerator{
		node: node,
	}, nil
}
