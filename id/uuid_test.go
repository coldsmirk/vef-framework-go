package id

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestUUIDGenerator tests UUID generator functionality.
func TestUUIDGenerator(t *testing.T) {
	t.Run("CreateGenerator", func(t *testing.T) {
		generator := NewUUIDGenerator()
		assert.NotNil(t, generator, "Generator should not be nil")
	})

	t.Run("GenerateValidUUIDV7Format", func(t *testing.T) {
		generator := NewUUIDGenerator()
		id := generator.Generate()

		assert.NotEmpty(t, id, "UUID should not be empty")
		assert.Len(t, id, 36, "UUID should be 36 characters")

		uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
		assert.True(t, uuidRegex.MatchString(id), "UUID should match v7 format: %s", id)

		assert.Equal(t, "-", string(id[8]), "UUID should have dash at position 8")
		assert.Equal(t, "-", string(id[13]), "UUID should have dash at position 13")
		assert.Equal(t, "-", string(id[18]), "UUID should have dash at position 18")
		assert.Equal(t, "-", string(id[23]), "UUID should have dash at position 23")
		assert.Equal(t, "7", string(id[14]), "UUID v7 should have version 7 at position 14")

		variantChar := string(id[19])
		assert.Contains(t, []string{"8", "9", "a", "b"}, variantChar,
			"UUID variant should be 8, 9, a, or b, got: %s", variantChar)
	})

	t.Run("GenerateUniqueUUIDs", func(t *testing.T) {
		generator := NewUUIDGenerator()
		uuids := make(map[string]bool)
		iterations := 10000

		for range iterations {
			uuid := generator.Generate()
			assert.False(t, uuids[uuid], "UUID should be unique: %s", uuid)
			uuids[uuid] = true
		}

		assert.Len(t, uuids, iterations, "All UUIDs should be unique")
	})

	t.Run("ThreadSafe", func(t *testing.T) {
		generator := NewUUIDGenerator()

		const (
			numGoroutines     = 100
			uuidsPerGoroutine = 100
		)

		uuidChan := make(chan string, numGoroutines*uuidsPerGoroutine)

		for range numGoroutines {
			go func() {
				for range uuidsPerGoroutine {
					uuidChan <- generator.Generate()
				}
			}()
		}

		uuids := make(map[string]bool)

		for range numGoroutines * uuidsPerGoroutine {
			uuid := <-uuidChan
			assert.False(t, uuids[uuid], "Concurrent generation should produce unique UUIDs")
			uuids[uuid] = true
		}

		assert.Len(t, uuids, numGoroutines*uuidsPerGoroutine, "All concurrent UUIDs should be unique")
	})

	t.Run("TimeOrderedUUIDs", func(t *testing.T) {
		generator := NewUUIDGenerator()

		var uuids []string
		for range 100 {
			uuids = append(uuids, generator.Generate())
		}

		for i := 1; i < len(uuids); i++ {
			assert.True(t, uuids[i] >= uuids[i-1] ||
				uuids[i][0:8] == uuids[i-1][0:8],
				"UUID v7 should maintain rough time ordering")
		}
	})

	t.Run("DefaultGenerator", func(t *testing.T) {
		assert.NotNil(t, DefaultUUIDGenerator, "Default generator should be initialized")

		uuid := DefaultUUIDGenerator.Generate()
		assert.NotEmpty(t, uuid, "Default generator should produce UUIDs")
		assert.Len(t, uuid, 36, "Default generator should produce 36-character UUIDs")

		uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
		assert.True(t, uuidRegex.MatchString(uuid), "Default generator should produce valid UUID v7")
	})

	t.Run("RapidGeneration", func(t *testing.T) {
		generator := NewUUIDGenerator()
		uuids := make(map[string]bool)

		for range 1000 {
			uuid := generator.Generate()
			assert.False(t, uuids[uuid], "Rapid generation should produce unique UUIDs")
			uuids[uuid] = true
		}

		assert.Len(t, uuids, 1000, "All rapidly generated UUIDs should be unique")
	})

	t.Run("ValidHexCharactersOnly", func(t *testing.T) {
		generator := NewUUIDGenerator()
		uuid := generator.Generate()

		hexPart := uuid[0:8] + uuid[9:13] + uuid[14:18] + uuid[19:23] + uuid[24:36]
		for _, char := range hexPart {
			assert.True(t,
				(char >= '0' && char <= '9') || (char >= 'a' && char <= 'f'),
				"UUID should contain only valid hex characters: %c", char)
		}
	})
}
