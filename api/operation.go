package api

import "time"

// OperationSpec defines the specification for an API endpoint.
type OperationSpec struct {
	// Action is the action name for the API endpoint
	Action string
	// EnableAudit indicates whether to enable audit logging for this endpoint
	EnableAudit bool
	// Timeout is the request timeout duration
	Timeout time.Duration
	// Public indicates whether this endpoint is publicly accessible
	Public bool
	// RequiredPermission is the permission token required for access
	RequiredPermission string
	// RateLimit represents the rate limit for an API endpoint
	RateLimit *RateLimitConfig
	// Handler is the business logic handler.
	Handler any
}

// Operation is the runtime operation definition.
// Created by Engine from Resource + OperationSpec.
type Operation struct {
	// Identifier contains the full operation identity.
	Identifier

	// EnableAudit indicates whether audit logging is enabled.
	EnableAudit bool
	// Timeout is the final timeout duration.
	Timeout time.Duration
	// Auth indicates the authentication configuration.
	Auth *AuthConfig
	// RateLimit is the final rate limit configuration.
	RateLimit *RateLimitConfig
	// Handler is the resolved handler (before adaptation).
	Handler any
	// Meta holds additional operation-specific data.
	// For REST: may contain parsed method, path pattern, etc.
	Meta map[string]any
}

// RateLimitConfig defines rate limiting configuration.
type RateLimitConfig struct {
	// Max is the maximum number of requests allowed.
	Max int
	// Period is the time window for rate limiting.
	Period time.Duration
	// Key is a custom rate limit key template.
	// Empty means using the default key.
	Key string
}

// OperationsProvider provides operation specs.
// Embed types implementing this interface in a resource to contribute operations.
type OperationsProvider interface {
	// Provide returns the operation specs for this provider.
	Provide() []OperationSpec
}

// OperationsCollector collects all operations from a resource.
// This includes operations from embedded providers.
type OperationsCollector interface {
	// Collect gathers all operation specs from a resource.
	// Returns specs from embedded OperationsProviders.
	Collect(resource Resource) []OperationSpec
}
