package push

import "errors"

var (
	errHubClosed          = errors.New("push: hub is shut down")
	errTooManyConnections = errors.New("push: per-user connection limit reached")
)
