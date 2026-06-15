package storage

import "errors"

// ErrEventRouteNotTransactional indicates the framework's event bus is
// not configured to deliver a storage domain event through a
// transactional transport. Storage publishes with event.WithTx; without
// such a route the first publish would fail at runtime, so the module
// fails fast at start-up instead. The wrapped formatted error names the
// offending event type and points operators at the configuration that
// must be set.
var ErrEventRouteNotTransactional = errors.New("storage: event must route to a transactional transport")

// ErrUnsupportedStorageProvider is returned by NewService when the
// configured provider does not match any of the known backends.
var ErrUnsupportedStorageProvider = errors.New("unsupported storage provider")
