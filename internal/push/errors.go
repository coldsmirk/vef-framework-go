package push

import "errors"

var (
	errHubClosed          = errors.New("push: hub is shut down")
	errTooManyConnections = errors.New("push: per-user connection limit reached")
	// errRelayRequiresAppName fails the boot when the Redis relay would come up
	// without the channel namespace that keeps co-tenant applications apart.
	errRelayRequiresAppName = errors.New("push: the redis relay requires a non-empty vef.app.name to namespace its pub/sub channel")
)
