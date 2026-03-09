package id

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRandomIDGenerator tests random ID generator functionality.
func TestRandomIDGenerator(t *testing.T) {
	t.Run("CreateWithCustomAlphabetAndLength", func(t *testing.T) {
		alphabet := "0123456789ABCDEF"
		length := 16
		generator := NewRandomIDGenerator(WithAlphabet(alphabet), WithLength(length))
		assert.NotNil(t, generator, "Generator should not be nil")

		id := generator.Generate()
		assert.NotEmpty(t, id, "ID should not be empty")
		assert.Len(t, id, length, "ID should have specified length")

		for _, char := range id {
			assert.True(t, strings.ContainsRune(alphabet, char),
				"ID should contain only alphabet characters: %c", char)
		}
	})

	t.Run("GenerateUniqueIDsWithDefaultAlphabet", func(t *testing.T) {
		alphabet := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-"
		length := 21
		generator := NewRandomIDGenerator(WithAlphabet(alphabet), WithLength(length))

		ids := make(map[string]bool)
		iterations := 10000

		for range iterations {
			id := generator.Generate()
			assert.False(t, ids[id], "ID should be unique: %s", id)
			ids[id] = true
		}

		assert.Len(t, ids, iterations, "All IDs should be unique")
	})

	t.Run("NumericAlphabet", func(t *testing.T) {
		alphabet := "0123456789"
		length := 10
		generator := NewRandomIDGenerator(WithAlphabet(alphabet), WithLength(length))

		id := generator.Generate()
		assert.Len(t, id, length, "ID should have correct length")

		for _, char := range id {
			assert.True(t, char >= '0' && char <= '9',
				"ID should contain only digits: %c", char)
		}
	})

	t.Run("AlphabeticCharacters", func(t *testing.T) {
		alphabet := "abcdefghijklmnopqrstuvwxyz"
		length := 12
		generator := NewRandomIDGenerator(WithAlphabet(alphabet), WithLength(length))

		id := generator.Generate()
		assert.Len(t, id, length, "ID should have correct length")

		for _, char := range id {
			assert.True(t, char >= 'a' && char <= 'z',
				"ID should contain only lowercase letters: %c", char)
		}
	})

	t.Run("ShortIDs", func(t *testing.T) {
		alphabet := "ABCDEF"
		length := 4
		generator := NewRandomIDGenerator(WithAlphabet(alphabet), WithLength(length))

		id := generator.Generate()
		assert.Len(t, id, length, "ID should have correct length")

		for _, char := range id {
			assert.True(t, strings.ContainsRune(alphabet, char),
				"ID should contain only alphabet characters: %c", char)
		}
	})

	t.Run("LongIDs", func(t *testing.T) {
		alphabet := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		length := 128
		generator := NewRandomIDGenerator(WithAlphabet(alphabet), WithLength(length))

		id := generator.Generate()
		assert.Len(t, id, length, "ID should have correct length")

		for _, char := range id {
			assert.True(t, strings.ContainsRune(alphabet, char),
				"ID should contain only alphabet characters: %c", char)
		}
	})

	t.Run("ThreadSafe", func(t *testing.T) {
		alphabet := "0123456789abcdefghijklmnopqrstuvwxyz"
		length := 16
		generator := NewRandomIDGenerator(WithAlphabet(alphabet), WithLength(length))

		const (
			numGoroutines   = 100
			idsPerGoroutine = 100
		)

		idChan := make(chan string, numGoroutines*idsPerGoroutine)

		for range numGoroutines {
			go func() {
				for range idsPerGoroutine {
					idChan <- generator.Generate()
				}
			}()
		}

		ids := make(map[string]bool)

		for range numGoroutines * idsPerGoroutine {
			id := <-idChan
			assert.Len(t, id, length, "Concurrent generation should produce correct length")
			assert.False(t, ids[id], "Concurrent generation should produce unique IDs")
			ids[id] = true
		}

		assert.Len(t, ids, numGoroutines*idsPerGoroutine, "All concurrent IDs should be unique")
	})

	t.Run("SingleCharacterAlphabet", func(t *testing.T) {
		generator := NewRandomIDGenerator(WithAlphabet("A"), WithLength(5))

		id := generator.Generate()
		assert.Equal(t, "AAAAA", id, "Single character alphabet should repeat character")
	})

	t.Run("SpecialCharacters", func(t *testing.T) {
		alphabet := "!@#$%^&*()"
		length := 8
		generator := NewRandomIDGenerator(WithAlphabet(alphabet), WithLength(length))

		id := generator.Generate()
		assert.Len(t, id, length, "ID should have correct length")

		for _, char := range id {
			assert.True(t, strings.ContainsRune(alphabet, char),
				"ID should contain only alphabet characters: %c", char)
		}
	})

	t.Run("DifferentIDsWithSameParameters", func(t *testing.T) {
		alphabet := "0123456789abcdef"
		length := 20
		generator := NewRandomIDGenerator(WithAlphabet(alphabet), WithLength(length))

		ids := make([]string, 100)
		for i := range ids {
			ids[i] = generator.Generate()
		}

		uniqueIDs := make(map[string]bool)
		for _, id := range ids {
			uniqueIDs[id] = true
		}

		assert.GreaterOrEqual(t, len(uniqueIDs), 95,
			"Should generate mostly unique IDs with sufficient entropy")
	})

	t.Run("DefaultValues", func(t *testing.T) {
		generator := NewRandomIDGenerator()

		id := generator.Generate()
		assert.Len(t, id, DefaultRandomIDGeneratorLength, "Default length should be 32")

		for _, char := range id {
			assert.True(t, strings.ContainsRune(DefaultRandomIDGeneratorAlphabet, char),
				"Default alphabet should be alphanumeric")
		}
	})
}
