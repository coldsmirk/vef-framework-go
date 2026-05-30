package database

type databaseOptions struct {
	PoolConfig *ConnectionPoolConfig
}

type Option func(*databaseOptions)

func newDefaultOptions() *databaseOptions {
	return &databaseOptions{
		PoolConfig: NewDefaultConnectionPoolConfig(),
	}
}

// WithConnectionPool overrides the default connection pool configuration applied
// to the opened *sql.DB.
func WithConnectionPool(poolConfig *ConnectionPoolConfig) Option {
	return func(opts *databaseOptions) {
		opts.PoolConfig = poolConfig
	}
}

func (opts *databaseOptions) apply(options ...Option) {
	for _, opt := range options {
		opt(opts)
	}
}
