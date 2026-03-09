package contract

import "context"

// Initializer defines the interface for components that require initialization.
// This method should ONLY be called during application startup and MUST NOT be called at runtime.
type Initializer interface {
	// Init initializes the component (e.g., creating buckets, directories, establishing connections).
	Init(ctx context.Context) error
}
