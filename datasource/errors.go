package datasource

import "errors"

var (
	// ErrNotFound is returned by Registry.Get/Update/Unregister/Kind when no data
	// source is currently registered under the requested name.
	ErrNotFound = errors.New("datasource: data source not found")

	// ErrExists is returned by Registry.Register when a data source with the same
	// name is already registered. Use Update to replace an existing
	// configuration, or Unregister + Register to fully recycle the entry.
	ErrExists = errors.New("datasource: data source already registered")

	// ErrPrimaryReserved is returned by Register/Update/Unregister when the caller
	// attempts to mutate the primary data source through the dynamic API. The
	// primary source is owned by the TOML configuration and lives for the lifetime
	// of the application.
	ErrPrimaryReserved = errors.New("datasource: primary data source is reserved")

	// ErrNameInvalid is returned by Register/Update when a name is empty or
	// contains whitespace or control characters.
	ErrNameInvalid = errors.New("datasource: data source name invalid")
)
