package oracle

import "errors"

var (
	// ErrOracleServiceRequired is returned when the data source omits the
	// database field, which for Oracle carries the service name — there is no
	// sensible default to fall back to.
	ErrOracleServiceRequired = errors.New("oracle data source requires a database (service name)")

	// ErrOracleRootCertUnsupported is returned when a verify mode supplies
	// ssl_root_cert: go-ora verifies against an Oracle wallet or the system
	// pool only, so a PEM root bundle cannot be honored and silently ignoring
	// it would fake the verification the operator asked for.
	ErrOracleRootCertUnsupported = errors.New("oracle data source does not support ssl_root_cert; use a system-trusted certificate")
)
