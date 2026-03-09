package sequence

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/internal/log"
	"github.com/coldsmirk/vef-framework-go/timex"
)

var logger = log.Named("sequence")

// Generator provides serial number generation.
type Generator interface {
	// Generate generates a new serial number for the given rule key.
	Generate(ctx context.Context, key string) (string, error)
	// GenerateN generates N serial numbers for the given rule key in a single atomic operation.
	GenerateN(ctx context.Context, key string, count int) ([]string, error)
}

// Store abstracts rule persistence and atomic counter operations.
// Implementations must guarantee atomicity of Reserve (read-modify-write must be serialized per key).
type Store interface {
	// Reserve reserves count sequence values for key based on rule policy at now.
	// It returns the rule snapshot used for generation and the final counter value in the reserved batch.
	Reserve(ctx context.Context, key string, count int, now timex.DateTime) (rule *Rule, newValue int, err error)
}
