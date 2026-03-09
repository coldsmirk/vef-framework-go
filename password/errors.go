package password

import "errors"

var (
	// ErrInvalidCost is returned when bcrypt cost is outside valid range (4-31).
	ErrInvalidCost = errors.New("invalid bcrypt cost: must be between 4 and 31")
	// ErrInvalidMemory is returned when argon2 memory parameter is too small.
	ErrInvalidMemory = errors.New("invalid argon2 memory: must be at least 8 KiB")
	// ErrInvalidIterations is returned when iteration count is less than 1.
	ErrInvalidIterations = errors.New("invalid iterations: must be at least 1")
	// ErrInvalidParallelism is returned when parallelism is less than 1.
	ErrInvalidParallelism = errors.New("invalid parallelism: must be at least 1")
	// ErrInvalidEncoderID is returned when CompositeEncoder receives an unknown encoder ID.
	ErrInvalidEncoderID = errors.New("invalid encoder id: encoder not found")
	// ErrInvalidHashFormat is returned when encoded password has unexpected format.
	ErrInvalidHashFormat = errors.New("invalid hash format")
	// ErrDefaultEncoderNotFound is returned when the default encoder ID is not registered in CompositeEncoder.
	ErrDefaultEncoderNotFound = errors.New("default encoder not found in registered encoders")
	// ErrCipherRequired is returned when cipher is not provided to CipherEncoder.
	ErrCipherRequired = errors.New("cipher is required")
	// ErrEncoderRequired is returned when encoder is not provided to CipherEncoder.
	ErrEncoderRequired = errors.New("encoder is required")
)
