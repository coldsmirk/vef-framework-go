package id

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGenerate tests generate functionality.
func TestGenerate(t *testing.T) {
	t.Run("GenerateNonEmptyID", func(t *testing.T) {
		id := Generate()
		assert.NotEmpty(t, id, "ID should not be empty")
	})

	t.Run("GenerateUniqueIDs", func(t *testing.T) {
		ids := make(map[string]bool)
		iterations := 1000

		for range iterations {
			id := Generate()
			assert.False(t, ids[id], "ID should be unique: %s", id)
			ids[id] = true
		}

		assert.Len(t, ids, iterations, "All IDs should be unique")
	})

	t.Run("UseXIDGeneratorByDefault", func(t *testing.T) {
		id := Generate()

		assert.Len(t, id, 20, "XID should be 20 characters")

		for _, char := range id {
			assert.True(t,
				(char >= '0' && char <= '9') || (char >= 'a' && char <= 'v'),
				"XID should contain base32 characters (0-9, a-v): %c", char)
		}
	})
}

// TestGenerateUUID tests GenerateUUID functionality.
func TestGenerateUUID(t *testing.T) {
	t.Run("GenerateValidUUIDV7", func(t *testing.T) {
		uuid := GenerateUUID()
		assert.NotEmpty(t, uuid, "UUID should not be empty")
		assert.Len(t, uuid, 36, "UUID should be 36 characters")
		assert.Equal(t, "-", string(uuid[8]), "UUID should have dash at position 8")
		assert.Equal(t, "-", string(uuid[13]), "UUID should have dash at position 13")
		assert.Equal(t, "-", string(uuid[18]), "UUID should have dash at position 18")
		assert.Equal(t, "-", string(uuid[23]), "UUID should have dash at position 23")
		assert.Equal(t, "7", string(uuid[14]), "UUID v7 should have version 7 at position 14")
	})

	t.Run("GenerateUniqueUUIDs", func(t *testing.T) {
		uuids := make(map[string]bool)
		iterations := 1000

		for range iterations {
			uuid := GenerateUUID()
			assert.False(t, uuids[uuid], "UUID should be unique: %s", uuid)
			uuids[uuid] = true
		}

		assert.Len(t, uuids, iterations, "All UUIDs should be unique")
	})
}

// TestDefaultGenerators tests default generators functionality.
func TestDefaultGenerators(t *testing.T) {
	t.Run("Initialized", func(t *testing.T) {
		assert.NotNil(t, DefaultXIDGenerator, "DefaultXIDGenerator should be initialized")
		assert.NotNil(t, DefaultUUIDGenerator, "DefaultUUIDGenerator should be initialized")
		assert.NotNil(t, DefaultSnowflakeIDGenerator, "DefaultSnowflakeGenerator should be initialized")
	})

	t.Run("GenerateIDs", func(t *testing.T) {
		xid := DefaultXIDGenerator.Generate()
		assert.NotEmpty(t, xid, "XID generator should produce ID")

		uuid := DefaultUUIDGenerator.Generate()
		assert.NotEmpty(t, uuid, "UUID generator should produce ID")

		snowflake := DefaultSnowflakeIDGenerator.Generate()
		assert.NotEmpty(t, snowflake, "Snowflake generator should produce ID")
	})
}

// TestConcurrentGeneration tests concurrent generation functionality.
func TestConcurrentGeneration(t *testing.T) {
	t.Run("ThreadSafe", func(t *testing.T) {
		const (
			numGoroutines   = 100
			idsPerGoroutine = 100
		)

		idChan := make(chan string, numGoroutines*idsPerGoroutine)

		for range numGoroutines {
			go func() {
				for range idsPerGoroutine {
					idChan <- Generate()
				}
			}()
		}

		ids := make(map[string]bool)

		for range numGoroutines * idsPerGoroutine {
			id := <-idChan
			assert.False(t, ids[id], "Concurrent generation should produce unique IDs")
			ids[id] = true
		}

		assert.Len(t, ids, numGoroutines*idsPerGoroutine, "All concurrent IDs should be unique")
	})
}
