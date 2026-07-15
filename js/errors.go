package js

import "errors"

var (
	// ErrInvalidLib is returned by NewEngine when a registered library is nil
	// or reports an empty name.
	ErrInvalidLib = errors.New("js: invalid lib")
	// ErrDuplicateLib is returned by NewEngine when two libraries share one
	// name, or a registered library shadows a standard library.
	ErrDuplicateLib = errors.New("js: duplicate lib name")
	// ErrLibNotFound is returned by Engine.NewRuntime when EnableLibs names a
	// library the engine catalog does not hold.
	ErrLibNotFound = errors.New("js: lib not found")
)
