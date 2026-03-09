package id

// IDGenerator defines the interface for all ID generation strategies.
// All generators must implement this interface to ensure consistency.
type IDGenerator interface {
	// Generate creates a new unique identifier as a string.
	// The format and characteristics depend on the specific implementation.
	Generate() string
}

// Generate creates a new unique identifier using the default XID generator.
// XID is chosen as the default because it offers the best performance with good uniqueness guarantees.
// The generated ID is a 20-character string using base32 encoding (0-9, a-v).
//
// Example:
//
//	id := Generate()
//	// Returns something like: "9m4e2mr0ui3e8a215n4g"
func Generate() string {
	return DefaultXIDGenerator.Generate()
}

// GenerateUUID creates a new UUID v7 identifier using the default UUID generator.
// UUID v7 provides time-based ordering and follows RFC 4122 standards.
// The generated UUID is a 36-character string in the format: xxxxxxxx-xxxx-7xxx-xxxx-xxxxxxxxxxxx
//
// Example:
//
//	uuid := GenerateUUID()
//	// Returns something like: "018f4e42-832a-7123-9abc-def012345678"
func GenerateUUID() string {
	return DefaultUUIDGenerator.Generate()
}
