package sqlx

import "database/sql/driver"

// Bool wraps the built-in bool type and implements driver.Valuer.
// It converts boolean values to database-compatible integers (1 for true, 0 for false),
// useful for databases without native boolean support.
type Bool bool

// Value implements driver.Valuer, converting true to 1 and false to 0.
func (b Bool) Value() (driver.Value, error) {
	if b {
		return int16(1), nil
	}

	return int16(0), nil
}
