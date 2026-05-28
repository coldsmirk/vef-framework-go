package orm

import "errors"

var (
	// ErrDataSourceNotFound is returned by DataSources.Get/Update/Unregister/Kind
	// when no data source is registered under the requested name.
	ErrDataSourceNotFound = errors.New("orm: data source not found")

	// ErrDataSourceExists is returned by DataSources.Register when a data source
	// with the same name is already registered. Use Update to replace an existing
	// configuration, or Unregister + Register to fully recycle the entry.
	ErrDataSourceExists = errors.New("orm: data source already registered")

	// ErrDataSourceClosed is returned by DataSources.Get when the requested data
	// source was Unregister'd but the caller still holds the name reference.
	ErrDataSourceClosed = errors.New("orm: data source closed")

	// ErrPrimaryReserved is returned by Register/Update/Unregister when the
	// caller attempts to mutate the primary data source through the dynamic API.
	// The primary source is owned by the TOML configuration and lives for the
	// lifetime of the application.
	ErrPrimaryReserved = errors.New("orm: primary data source is reserved")

	// ErrDataSourceNameInvalid is returned when a Register/Update name is empty
	// or otherwise rejected (whitespace, control characters).
	ErrDataSourceNameInvalid = errors.New("orm: data source name invalid")
)
