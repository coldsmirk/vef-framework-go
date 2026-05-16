package memory

import "github.com/coldsmirk/vef-framework-go/id"

// newSubID returns a subscription identifier. Backed by the framework's
// UUID generator so cross-package test fixtures collide negligibly.
func newSubID() string { return id.GenerateUUID() }
