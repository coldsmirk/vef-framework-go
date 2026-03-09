package sqlguard

import "context"

type contextKey int

const (
	keyWhitelisted contextKey = iota
)

// WithWhitelist marks the context to skip SQL guard checks.
func WithWhitelist(ctx context.Context) context.Context {
	return context.WithValue(ctx, keyWhitelisted, true)
}

// IsWhitelisted returns true if the context is marked to skip SQL guard checks.
func IsWhitelisted(ctx context.Context) bool {
	whitelisted, _ := ctx.Value(keyWhitelisted).(bool)

	return whitelisted
}
