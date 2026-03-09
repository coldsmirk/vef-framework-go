package id

import (
	"fmt"

	"github.com/google/uuid"
)

// DefaultUUIDGenerator is the default UUID v7 generator instance.
// UUID v7 embeds a timestamp for natural ordering and includes random bits for uniqueness.
var DefaultUUIDGenerator = NewUUIDGenerator()

type uuidGenerator struct{}

// Generate creates a new UUID v7 as a 36-character hyphenated string.
func (*uuidGenerator) Generate() string {
	id, err := uuid.NewV7()
	if err != nil {
		panic(fmt.Errorf("failed to generate UUID: %w", err))
	}

	return id.String()
}

// NewUUIDGenerator creates a new UUID v7 generator instance.
func NewUUIDGenerator() IDGenerator {
	return &uuidGenerator{}
}
