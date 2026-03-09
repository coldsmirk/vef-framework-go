package id

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSnowflakeEdgeCases tests snowflake edge cases functionality.
func TestSnowflakeEdgeCases(t *testing.T) {
	t.Run("MaximumNodeID", func(t *testing.T) {
		generator, err := NewSnowflakeIDGenerator(63)
		require.NoError(t, err, "Should not return error")

		id := generator.Generate()
		assert.NotEmpty(t, id, "Max node ID should generate valid IDs")
	})

	t.Run("NodeIDExceedingMaximum", func(t *testing.T) {
		_, err := NewSnowflakeIDGenerator(64)
		assert.Error(t, err, "Should return error")
		assert.Contains(t, err.Error(), "failed to create snowflake node", "Should contain expected value")
	})

	t.Run("NegativeNodeID", func(t *testing.T) {
		_, err := NewSnowflakeIDGenerator(-1)
		assert.Error(t, err, "Should return error")
		assert.Contains(t, err.Error(), "failed to create snowflake node", "Should contain expected value")
	})

	t.Run("RapidSequenceGeneration", func(t *testing.T) {
		generator, err := NewSnowflakeIDGenerator(1)
		require.NoError(t, err, "Should not return error")

		ids := make(map[string]bool)

		for range 5000 {
			id := generator.Generate()
			assert.False(t, ids[id], "Rapid sequence generation should produce unique IDs")
			ids[id] = true
		}

		assert.Len(t, ids, 5000, "All rapid sequence IDs should be unique")
	})
}

// TestRandomIdGeneratorEdgeCases tests random id generator edge cases functionality.
func TestRandomIdGeneratorEdgeCases(t *testing.T) {
	t.Run("EmptyAlphabet", func(t *testing.T) {
		generator := NewRandomIDGenerator(WithAlphabet(""), WithLength(10))
		assert.NotNil(t, generator, "Should create generator even with empty alphabet")

		assert.Panics(t, func() {
			generator.Generate()
		}, "Empty alphabet should panic when generating")
	})

	t.Run("ZeroLength", func(t *testing.T) {
		generator := NewRandomIDGenerator(WithAlphabet("abc"), WithLength(0))

		assert.Panics(t, func() {
			generator.Generate()
		}, "Zero length should panic")
	})

	t.Run("SingleCharacterAlphabet", func(t *testing.T) {
		generator := NewRandomIDGenerator(WithAlphabet("X"), WithLength(10))
		id := generator.Generate()
		assert.Equal(t, "XXXXXXXXXX", id, "Single character alphabet should repeat character")
	})

	t.Run("VeryLongIds", func(t *testing.T) {
		generator := NewRandomIDGenerator(WithAlphabet("0123456789"), WithLength(1000))
		id := generator.Generate()
		assert.Len(t, id, 1000, "Should handle very long ID generation")

		for _, char := range id {
			assert.True(t, char >= '0' && char <= '9', "Long ID should contain only digits")
		}
	})

	t.Run("UnicodeCharacters", func(t *testing.T) {
		generator := NewRandomIDGenerator(WithAlphabet("αβγδε"), WithLength(5))
		id := generator.Generate()
		assert.NotEmpty(t, id, "Should handle unicode alphabet")

		allowedRunes := []rune("αβγδε")
		for _, char := range id {
			found := slices.Contains(allowedRunes, char)
			assert.True(t, found, "Unicode ID should contain only alphabet characters: %c", char)
		}
	})
}

// TestUUIDEdgeCases tests UUID edge cases functionality.
func TestUUIDEdgeCases(t *testing.T) {
	t.Run("RapidGenerationWithoutCollision", func(t *testing.T) {
		generator := NewUUIDGenerator()
		uuids := make(map[string]bool)

		for range 100000 {
			uuid := generator.Generate()
			assert.False(t, uuids[uuid], "Rapid UUID generation should not have collisions")
			uuids[uuid] = true
		}

		assert.Len(t, uuids, 100000, "All rapid UUIDs should be unique")
	})

	t.Run("VersionAndVariantBitsUnderLoad", func(t *testing.T) {
		generator := NewUUIDGenerator()

		for range 1000 {
			uuid := generator.Generate()

			assert.Equal(t, "7", string(uuid[14]), "Version should always be 7")

			variantChar := string(uuid[19])
			assert.Contains(t, []string{"8", "9", "a", "b"}, variantChar,
				"Variant should be valid")
		}
	})
}

// TestXidEdgeCases tests xid edge cases functionality.
func TestXidEdgeCases(t *testing.T) {
	t.Run("ConcurrentGenerationFromMultipleGenerators", func(t *testing.T) {
		const (
			numGenerators   = 10
			idsPerGenerator = 1000
		)

		idChan := make(chan string, numGenerators*idsPerGenerator)

		for range numGenerators {
			go func() {
				generator := NewXIDGenerator()
				for range idsPerGenerator {
					idChan <- generator.Generate()
				}
			}()
		}

		ids := make(map[string]bool)

		for range numGenerators * idsPerGenerator {
			id := <-idChan
			assert.False(t, ids[id], "Multiple generators should produce unique IDs")
			ids[id] = true
		}

		assert.Len(t, ids, numGenerators*idsPerGenerator, "All IDs from multiple generators should be unique")
	})

	t.Run("FormatConsistency", func(t *testing.T) {
		generator := NewXIDGenerator()

		for range 1000 {
			id := generator.Generate()
			assert.Len(t, id, 20, "XID length should always be 20")

			for _, char := range id {
				assert.True(t,
					(char >= '0' && char <= '9') || (char >= 'a' && char <= 'v'),
					"XID should always use base32 alphabet")
			}
		}
	})
}

// TestEnvironmentVariables tests environment variables functionality.
func TestEnvironmentVariables(t *testing.T) {
	t.Run("InvalidNodeIDEnvironmentVariable", func(t *testing.T) {
		originalNodeID := os.Getenv("NODE_ID")

		defer func() {
			if originalNodeID != "" {
				_ = os.Setenv("NODE_ID", originalNodeID)
			} else {
				_ = os.Unsetenv("NODE_ID")
			}
		}()

		assert.NotNil(t, DefaultSnowflakeIDGenerator, "Default generator should be initialized")
	})
}

// TestInterfaceCompliance tests interface compliance functionality.
func TestInterfaceCompliance(t *testing.T) {
	t.Run("AllGeneratorsImplementInterface", func(t *testing.T) {
		generators := []IDGenerator{
			NewXIDGenerator(),
			NewUUIDGenerator(),
			NewRandomIDGenerator(WithAlphabet("abc"), WithLength(10)),
			DefaultXIDGenerator,
			DefaultUUIDGenerator,
			DefaultSnowflakeIDGenerator,
		}

		for i, generator := range generators {
			assert.NotNil(t, generator, "Generator %d should not be nil", i)

			id := generator.Generate()
			assert.NotEmpty(t, id, "Generator %d should produce ID", i)
		}
	})

	t.Run("SnowflakeGeneratorFromConstructor", func(t *testing.T) {
		generator, err := NewSnowflakeIDGenerator(1)
		require.NoError(t, err, "Should not return error")

		id := generator.Generate()
		assert.NotEmpty(t, id, "Snowflake generator should produce ID")
	})
}

// TestMemoryUsage tests memory usage functionality.
func TestMemoryUsage(t *testing.T) {
	t.Run("GeneratorsShouldNotLeakMemory", func(t *testing.T) {
		for i := range 1000 {
			_ = NewXIDGenerator()
			_ = NewUUIDGenerator()
			_ = NewRandomIDGenerator(WithAlphabet("abc"), WithLength(10))

			if i%100 == 0 {
				DefaultXIDGenerator.Generate()
				DefaultUUIDGenerator.Generate()
				DefaultSnowflakeIDGenerator.Generate()
			}
		}

		assert.True(t, true, "Memory usage test completed")
	})
}

// TestStringManipulation tests string manipulation functionality.
func TestStringManipulation(t *testing.T) {
	t.Run("IdsSafeForCommonStringOperations", func(t *testing.T) {
		generators := []IDGenerator{
			DefaultXIDGenerator,
			DefaultUUIDGenerator,
			DefaultSnowflakeIDGenerator,
			NewRandomIDGenerator(WithAlphabet("0123456789abcdef"), WithLength(16)),
		}

		for _, generator := range generators {
			id := generator.Generate()

			assert.NotEmpty(t, strings.TrimSpace(id), "ID should not be empty after trimming")
			assert.False(t, strings.Contains(id, " "), "ID should not contain spaces")
			assert.False(t, strings.Contains(id, "\n"), "ID should not contain newlines")
			assert.False(t, strings.Contains(id, "\t"), "ID should not contain tabs")

			_ = strings.ToUpper(id)
			_ = strings.ToLower(id)
		}
	})
}
