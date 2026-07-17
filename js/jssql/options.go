package jssql

// DefaultMaxRows caps query results unless overridden by WithMaxRows.
const DefaultMaxRows = 1000

// libConfig collects the settings resolved from Options.
type libConfig struct {
	maxRows      int
	allowExecute bool
}

// Option customizes the sql library.
type Option func(*libConfig)

// WithExecute enables sql.execute for mutating statements. Without it the library
// is read-only and execute throws ErrExecuteDisabled.
func WithExecute() Option {
	return func(c *libConfig) {
		c.allowExecute = true
	}
}

// WithMaxRows caps the number of rows a query may return; exceeding the cap
// fails the query instead of silently truncating the result.
func WithMaxRows(n int) Option {
	return func(c *libConfig) {
		c.maxRows = n
	}
}
