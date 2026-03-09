package crud

import (
	"github.com/gofiber/fiber/v3"
)

// contextKey is a custom type for context keys to avoid collisions with user-defined keys.
type contextKey int

const (
	// KeyQueryError is the context key for storing query errors during recursive CTE building.
	// Used internally by FindTree and FindTreeOptions operations to propagate errors from closures.
	keyQueryError contextKey = iota
)

// QueryError retrieves the query error stored in the context during CTE building.
// Returns nil if no error has been stored.
func QueryError(ctx fiber.Ctx) error {
	err, _ := ctx.Locals(keyQueryError).(error)

	return err
}

// SetQueryError stores a query error in the context during CTE building.
// This is used by tree APIs to propagate errors from within WithRecursive closures.
func SetQueryError(ctx fiber.Ctx, err error) {
	ctx.Locals(keyQueryError, err)
}
