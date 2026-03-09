package hashx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMD5 tests MD5 functionality.
func TestMD5(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"EmptyString", "", "d41d8cd98f00b204e9800998ecf8427e"},
		{"SimpleText", "hello", "5d41402abc4b2a76b9719d911017c592"},
		{"TextWithPunctuation", "Hello, World!", "65a8e27d8879283831b664bd8b7f0ad4"},
		{"LongText", "The quick brown fox jumps over the lazy dog", "9e107d9d372bb6826bd81d3542a419d6"},
		{"ChineseText", "中文测试", "089b4943ea034acfa445d050c7913e55"},
		{"NumericString", "12345", "827ccb0eea8a706c4c34a16891f84e7b"},
		{"SpecialCharacters", "!@#$%^&*()", "05b28d17a7b6e7024b6e5d8cc43a8bf7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MD5(tt.input)
			assert.Equal(t, tt.expected, result, "Should equal expected value")

			resultBytes := MD5Bytes([]byte(tt.input))
			assert.Equal(t, tt.expected, resultBytes, "Should equal expected value")
		})
	}
}

// TestSHA1 tests SHA1 functionality.
func TestSHA1(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"EmptyString", "", "da39a3ee5e6b4b0d3255bfef95601890afd80709"},
		{"SimpleText", "hello", "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d"},
		{"TextWithPunctuation", "Hello, World!", "0a0a9f2a6772942557ab5355d76af442f8f65e01"},
		{"LongText", "The quick brown fox jumps over the lazy dog", "2fd4e1c67a2d28fced849ee1bb76e7391b93eb12"},
		{"ChineseText", "中文测试", "cf8a8e8f68b4e267920dba0a5f3037180cc1afd9"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SHA1(tt.input)
			assert.Equal(t, tt.expected, result, "Should equal expected value")

			resultBytes := SHA1Bytes([]byte(tt.input))
			assert.Equal(t, tt.expected, resultBytes, "Should equal expected value")
		})
	}
}

// TestSHA256 tests SHA256 functionality.
func TestSHA256(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"EmptyString", "", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"SimpleText", "hello", "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"},
		{"TextWithPunctuation", "Hello, World!", "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f"},
		{"LongText", "The quick brown fox jumps over the lazy dog", "d7a8fbb307d7809469ca9abcb0082e4f8d5651e46d3cdb762d02d0bf37c9e592"},
		{"ChineseText", "中文测试", "e350545d18735c5dd2dec50dcb971f3eb4cdda24b95a79bdb6b553f6a01ceb87"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SHA256(tt.input)
			assert.Equal(t, tt.expected, result, "Should equal expected value")

			resultBytes := SHA256Bytes([]byte(tt.input))
			assert.Equal(t, tt.expected, resultBytes, "Should equal expected value")
		})
	}
}

// TestSHA512 tests SHA512 functionality.
func TestSHA512(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"EmptyString", "", "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e"},
		{"SimpleText", "hello", "9b71d224bd62f3785d96d46ad3ea3d73319bfbc2890caadae2dff72519673ca72323c3d99ba5c11d7c7acc6e14b8c5da0c4663475c2e5c3adef46f73bcdec043"},
		{"TextWithPunctuation", "Hello, World!", "374d794a95cdcfd8b35993185fef9ba368f160d8daf432d08ba9f1ed1e5abe6cc69291e0fa2fe0006a52570ef18c19def4e617c33ce52ef0a6e5fbe318cb0387"},
		{"ChineseText", "中文测试", "1fea9aee07bd0ab66604ef4f079d6b109a0e625c3bc38fe8f850111a9ee6b4a689f3cb454dfd8a16cbd35963382f4ca5d91cdcff2dd473028e6cfee256812eec"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SHA512(tt.input)
			assert.Equal(t, tt.expected, result, "Should equal expected value")

			resultBytes := SHA512Bytes([]byte(tt.input))
			assert.Equal(t, tt.expected, resultBytes, "Should equal expected value")
		})
	}
}

// TestSM3 tests SM3 functionality.
func TestSM3(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"EmptyString", "", "1ab21d8355cfa17f8e61194831e81a8f22bec8c728fefb747ed035eb5082aa2b"},
		{"SimpleText", "abc", "66c7f0f462eeedd9d1f2d46bdc10e4e24167c4875cf2f7a2297da02b8f4ba8e0"},
		{"AnotherSimpleText", "hello", "becbbfaae6548b8bf0cfcad5a27183cd1be6093b1cceccc303d9c61d0a645268"},
		{"TextWithPunctuation", "Hello, World!", "7ed26cbf0bee4ca7d55c1e64714c4aa7d1f163089ef5ceb603cd102c81fbcbc5"},
		{"ChineseText", "中文测试", "ac85a5ef8576c66e75c36f037aaf89bf3cbb3e2745e595bb47b47ea53f30f838"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SM3(tt.input)
			assert.Equal(t, tt.expected, result, "Should equal expected value")

			resultBytes := SM3Bytes([]byte(tt.input))
			assert.Equal(t, tt.expected, resultBytes, "Should equal expected value")
		})
	}
}

// TestHmacMD5 tests HMAC MD5 functionality.
func TestHmacMD5(t *testing.T) {
	tests := []struct {
		name string
		key  []byte
		data []byte
	}{
		{"BasicHmac", []byte("secret-key"), []byte("test message")},
		{"EmptyKey", []byte{}, []byte("test message")},
		{"EmptyData", []byte("secret-key"), []byte{}},
		{"BothEmpty", []byte{}, []byte{}},
		{"LongKey", []byte("this-is-a-very-long-secret-key-for-testing-purposes"), []byte("test message")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HmacMD5(tt.key, tt.data)
			assert.Len(t, result, 32, "Length should be 32")
			assert.Regexp(t, "^[0-9a-f]+$", result, "Should contain only lowercase hex characters")

			result2 := HmacMD5(tt.key, tt.data)
			assert.Equal(t, result, result2, "Should produce consistent results")
		})
	}

	t.Run("DifferentKeysProduceDifferentResults", func(t *testing.T) {
		data := []byte("test message")
		result1 := HmacMD5([]byte("key1"), data)
		result2 := HmacMD5([]byte("key2"), data)
		assert.NotEqual(t, result1, result2, "Should not equal")
	})
}

// TestHmacSHA1 tests HMAC SHA1 functionality.
func TestHmacSHA1(t *testing.T) {
	tests := []struct {
		name string
		key  []byte
		data []byte
	}{
		{"BasicHmac", []byte("secret-key"), []byte("test message")},
		{"EmptyKey", []byte{}, []byte("test message")},
		{"EmptyData", []byte("secret-key"), []byte{}},
		{"BothEmpty", []byte{}, []byte{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HmacSHA1(tt.key, tt.data)
			assert.Len(t, result, 40, "Length should be 40")
			assert.Regexp(t, "^[0-9a-f]+$", result, "Should contain only lowercase hex characters")
		})
	}
}

// TestHmacSHA256 tests HMAC SHA256 functionality.
func TestHmacSHA256(t *testing.T) {
	tests := []struct {
		name     string
		key      []byte
		data     []byte
		expected string
	}{
		{"BasicHmac", []byte("secret-key"), []byte("test message"), ""},
		{"StandardTestVector", []byte("key"), []byte("The quick brown fox jumps over the lazy dog"), "f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8"},
		{"EmptyKey", []byte{}, []byte("data"), ""},
		{"EmptyData", []byte("key"), []byte{}, ""},
		{"BothEmpty", []byte{}, []byte{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HmacSHA256(tt.key, tt.data)
			assert.Len(t, result, 64, "Length should be 64")
			assert.Regexp(t, "^[0-9a-f]+$", result, "Should contain only lowercase hex characters")

			if tt.expected != "" {
				assert.Equal(t, tt.expected, result, "Should equal expected value")
			}
		})
	}
}

// TestHmacSHA512 tests HMAC SHA512 functionality.
func TestHmacSHA512(t *testing.T) {
	tests := []struct {
		name string
		key  []byte
		data []byte
	}{
		{"BasicHmac", []byte("secret-key"), []byte("test message")},
		{"EmptyKey", []byte{}, []byte("test message")},
		{"EmptyData", []byte("secret-key"), []byte{}},
		{"BothEmpty", []byte{}, []byte{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HmacSHA512(tt.key, tt.data)
			assert.Len(t, result, 128, "Length should be 128")
			assert.Regexp(t, "^[0-9a-f]+$", result, "Should contain only lowercase hex characters")
		})
	}
}

// TestHmacSM3 tests HMAC SM3 functionality.
func TestHmacSM3(t *testing.T) {
	tests := []struct {
		name string
		key  []byte
		data []byte
	}{
		{"BasicHmac", []byte("secret-key"), []byte("test message")},
		{"EmptyKey", []byte{}, []byte("test message")},
		{"EmptyData", []byte("secret-key"), []byte{}},
		{"BothEmpty", []byte{}, []byte{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HmacSM3(tt.key, tt.data)
			assert.Len(t, result, 64, "Length should be 64")
			assert.Regexp(t, "^[0-9a-f]+$", result, "Should contain only lowercase hex characters")

			result2 := HmacSM3(tt.key, tt.data)
			assert.Equal(t, result, result2, "Should produce consistent results")
		})
	}
}

// TestHashFunctionsNilInput tests Hash Functions nil input scenarios.
func TestHashFunctionsNilInput(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func() string
	}{
		{"MD5Bytes", func() string { return MD5Bytes(nil) }},
		{"SHA1Bytes", func() string { return SHA1Bytes(nil) }},
		{"SHA256Bytes", func() string { return SHA256Bytes(nil) }},
		{"SHA512Bytes", func() string { return SHA512Bytes(nil) }},
		{"SM3Bytes", func() string { return SM3Bytes(nil) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.testFunc()
			assert.NotEmpty(t, result, "Should not be empty")
			assert.Regexp(t, "^[0-9a-f]+$", result, "Should contain only lowercase hex characters")
		})
	}
}

// TestHashOutputFormat tests hash output format functionality.
func TestHashOutputFormat(t *testing.T) {
	testData := "test"

	tests := []struct {
		name     string
		testFunc func(string) string
	}{
		{"MD5", MD5},
		{"SHA1", SHA1},
		{"SHA256", SHA256},
		{"SHA512", SHA512},
		{"SM3", SM3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.testFunc(testData)
			assert.Regexp(t, "^[0-9a-f]+$", result, "Should contain only lowercase hex characters")
			assert.NotEmpty(t, result, "Should not be empty")
		})
	}
}

func BenchmarkMD5(b *testing.B) {
	data := "benchmark test data"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			MD5(data)
		}
	})
}

func BenchmarkSHA256(b *testing.B) {
	data := "benchmark test data"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			SHA256(data)
		}
	})
}

func BenchmarkSHA512(b *testing.B) {
	data := "benchmark test data"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			SHA512(data)
		}
	})
}

func BenchmarkSM3(b *testing.B) {
	data := "benchmark test data"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			SM3(data)
		}
	})
}

func BenchmarkHmacSHA256(b *testing.B) {
	key := []byte("secret-key")
	data := []byte("benchmark test data")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			HmacSHA256(key, data)
		}
	})
}
