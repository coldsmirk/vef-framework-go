package event

import "github.com/coldsmirk/vef-framework-go/id"

// newEventID returns a fresh message identifier for an Envelope. It is
// distinct from any business identifier on the payload and stable across
// retries within a single publish.
func newEventID() string { return id.GenerateUUID() }
