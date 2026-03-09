package api

import "errors"

var (
	// Resource errors.
	ErrEmptyResourceName       = errors.New("empty resource name")
	ErrInvalidResourceName     = errors.New("invalid resource name format")
	ErrResourceNameSlash       = errors.New("resource name cannot start or end with slash")
	ErrResourceNameDoubleSlash = errors.New("resource name cannot contain consecutive slashes")
	ErrInvalidResourceKind     = errors.New("invalid resource kind")

	// Version errors.
	ErrInvalidVersionFormat = errors.New("invalid version format, must match pattern v+digits (e.g., v1, v2, v10)")

	// Action errors.
	ErrEmptyActionName   = errors.New("empty action name")
	ErrInvalidActionName = errors.New("invalid action name format")

	// Decode errors.
	ErrInvalidParamsType = errors.New("invalid params type: must be pointer to struct")
	ErrInvalidMetaType   = errors.New("invalid meta type: must be pointer to struct")
)
