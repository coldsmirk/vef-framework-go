package password

import (
	"fmt"
	"testing"
)

const benchmarkPassword = "MySecurePassword123!"

// Benchmark Encoding

func BenchmarkPlaintextEncode(b *testing.B) {
	encoder := NewPlaintextEncoder()

	b.ResetTimer()

	for b.Loop() {
		_, _ = encoder.Encode(benchmarkPassword)
	}
}

func BenchmarkMd5Encode(b *testing.B) {
	encoder := NewMd5Encoder(WithMd5Salt("secret"))

	b.ResetTimer()

	for b.Loop() {
		_, _ = encoder.Encode(benchmarkPassword)
	}
}

func BenchmarkSha256Encode(b *testing.B) {
	encoder := NewSha256Encoder(WithSha256Salt("secret"))

	b.ResetTimer()

	for b.Loop() {
		_, _ = encoder.Encode(benchmarkPassword)
	}
}

func BenchmarkBcryptEncode_Cost4(b *testing.B) {
	encoder := NewBcryptEncoder(WithBcryptCost(4))

	b.ResetTimer()

	for b.Loop() {
		_, _ = encoder.Encode(benchmarkPassword)
	}
}

func BenchmarkBcryptEncode_Cost10(b *testing.B) {
	encoder := NewBcryptEncoder(WithBcryptCost(10))

	b.ResetTimer()

	for b.Loop() {
		_, _ = encoder.Encode(benchmarkPassword)
	}
}

func BenchmarkPbkdf2Encode_1000Iterations(b *testing.B) {
	encoder := NewPbkdf2Encoder(WithPbkdf2Iterations(1000))

	b.ResetTimer()

	for b.Loop() {
		_, _ = encoder.Encode(benchmarkPassword)
	}
}

func BenchmarkPbkdf2Encode_10000Iterations(b *testing.B) {
	encoder := NewPbkdf2Encoder(WithPbkdf2Iterations(10000))

	b.ResetTimer()

	for b.Loop() {
		_, _ = encoder.Encode(benchmarkPassword)
	}
}

func BenchmarkScryptEncode_N1024(b *testing.B) {
	encoder := NewScryptEncoder(WithScryptN(1024), WithScryptR(8), WithScryptP(1))

	b.ResetTimer()

	for b.Loop() {
		_, _ = encoder.Encode(benchmarkPassword)
	}
}

func BenchmarkScryptEncode_N16384(b *testing.B) {
	encoder := NewScryptEncoder(WithScryptN(16384), WithScryptR(8), WithScryptP(1))

	b.ResetTimer()

	for b.Loop() {
		_, _ = encoder.Encode(benchmarkPassword)
	}
}

func BenchmarkArgon2Encode_Light(b *testing.B) {
	encoder := NewArgon2Encoder(
		WithArgon2Memory(16*1024),
		WithArgon2Iterations(1),
		WithArgon2Parallelism(1),
	)

	b.ResetTimer()

	for b.Loop() {
		_, _ = encoder.Encode(benchmarkPassword)
	}
}

func BenchmarkArgon2Encode_Default(b *testing.B) {
	encoder := NewArgon2Encoder()

	b.ResetTimer()

	for b.Loop() {
		_, _ = encoder.Encode(benchmarkPassword)
	}
}

// Benchmark Matching/Verification

func BenchmarkPlaintextMatches(b *testing.B) {
	encoder := NewPlaintextEncoder()
	encoded, _ := encoder.Encode(benchmarkPassword)

	b.ResetTimer()

	for b.Loop() {
		_ = encoder.Matches(benchmarkPassword, encoded)
	}
}

func BenchmarkMd5Matches(b *testing.B) {
	encoder := NewMd5Encoder(WithMd5Salt("secret"))
	encoded, _ := encoder.Encode(benchmarkPassword)

	b.ResetTimer()

	for b.Loop() {
		_ = encoder.Matches(benchmarkPassword, encoded)
	}
}

func BenchmarkSha256Matches(b *testing.B) {
	encoder := NewSha256Encoder(WithSha256Salt("secret"))
	encoded, _ := encoder.Encode(benchmarkPassword)

	b.ResetTimer()

	for b.Loop() {
		_ = encoder.Matches(benchmarkPassword, encoded)
	}
}

func BenchmarkBcryptMatches_Cost4(b *testing.B) {
	encoder := NewBcryptEncoder(WithBcryptCost(4))
	encoded, _ := encoder.Encode(benchmarkPassword)

	b.ResetTimer()

	for b.Loop() {
		_ = encoder.Matches(benchmarkPassword, encoded)
	}
}

func BenchmarkBcryptMatches_Cost10(b *testing.B) {
	encoder := NewBcryptEncoder(WithBcryptCost(10))
	encoded, _ := encoder.Encode(benchmarkPassword)

	b.ResetTimer()

	for b.Loop() {
		_ = encoder.Matches(benchmarkPassword, encoded)
	}
}

func BenchmarkPbkdf2Matches_1000Iterations(b *testing.B) {
	encoder := NewPbkdf2Encoder(WithPbkdf2Iterations(1000))
	encoded, _ := encoder.Encode(benchmarkPassword)

	b.ResetTimer()

	for b.Loop() {
		_ = encoder.Matches(benchmarkPassword, encoded)
	}
}

func BenchmarkPbkdf2Matches_10000Iterations(b *testing.B) {
	encoder := NewPbkdf2Encoder(WithPbkdf2Iterations(10000))
	encoded, _ := encoder.Encode(benchmarkPassword)

	b.ResetTimer()

	for b.Loop() {
		_ = encoder.Matches(benchmarkPassword, encoded)
	}
}

func BenchmarkScryptMatches_N1024(b *testing.B) {
	encoder := NewScryptEncoder(WithScryptN(1024), WithScryptR(8), WithScryptP(1))
	encoded, _ := encoder.Encode(benchmarkPassword)

	b.ResetTimer()

	for b.Loop() {
		_ = encoder.Matches(benchmarkPassword, encoded)
	}
}

func BenchmarkScryptMatches_N16384(b *testing.B) {
	encoder := NewScryptEncoder(WithScryptN(16384), WithScryptR(8), WithScryptP(1))
	encoded, _ := encoder.Encode(benchmarkPassword)

	b.ResetTimer()

	for b.Loop() {
		_ = encoder.Matches(benchmarkPassword, encoded)
	}
}

func BenchmarkArgon2Matches_Light(b *testing.B) {
	encoder := NewArgon2Encoder(
		WithArgon2Memory(16*1024),
		WithArgon2Iterations(1),
		WithArgon2Parallelism(1),
	)
	encoded, _ := encoder.Encode(benchmarkPassword)

	b.ResetTimer()

	for b.Loop() {
		_ = encoder.Matches(benchmarkPassword, encoded)
	}
}

func BenchmarkArgon2Matches_Default(b *testing.B) {
	encoder := NewArgon2Encoder()
	encoded, _ := encoder.Encode(benchmarkPassword)

	b.ResetTimer()

	for b.Loop() {
		_ = encoder.Matches(benchmarkPassword, encoded)
	}
}

// TestEncoderPerformanceComparison runs a comparison test that prints a formatted table.
func TestEncoderPerformanceComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance comparison in short mode")
	}

	type encoderConfig struct {
		name    string
		encoder Encoder
	}

	encoders := []encoderConfig{
		{"Plaintext", NewPlaintextEncoder()},
		{"MD5 (with salt)", NewMd5Encoder(WithMd5Salt("secret"))},
		{"SHA256 (with salt)", NewSha256Encoder(WithSha256Salt("secret"))},
		{"Bcrypt (cost=4)", NewBcryptEncoder(WithBcryptCost(4))},
		{"Bcrypt (cost=10)", NewBcryptEncoder(WithBcryptCost(10))},
		{"PBKDF2 (iter=1000)", NewPbkdf2Encoder(WithPbkdf2Iterations(1000))},
		{"PBKDF2 (iter=10000)", NewPbkdf2Encoder(WithPbkdf2Iterations(10000))},
		{"Scrypt (N=1024)", NewScryptEncoder(WithScryptN(1024), WithScryptR(8), WithScryptP(1))},
		{"Scrypt (N=16384)", NewScryptEncoder(WithScryptN(16384), WithScryptR(8), WithScryptP(1))},
		{"Argon2 (light)", NewArgon2Encoder(WithArgon2Memory(16*1024), WithArgon2Iterations(1), WithArgon2Parallelism(1))},
		{"Argon2 (default)", NewArgon2Encoder()},
	}

	fmt.Println("\n=== Password Encoder Performance Comparison ===")
	fmt.Println("Password:", benchmarkPassword)
	fmt.Println()
	fmt.Printf("%-25s | %-15s | %-15s\n", "Encoder", "Encode Time", "Matches Time")
	fmt.Println("------------------------------------------------------------")

	for _, ec := range encoders {
		// Benchmark encoding
		encodeResult := testing.Benchmark(func(b *testing.B) {
			for b.Loop() {
				_, _ = ec.encoder.Encode(benchmarkPassword)
			}
		})

		// Encode once for matching benchmark
		encoded, err := ec.encoder.Encode(benchmarkPassword)
		if err != nil {
			t.Errorf("%s: Encode failed: %v", ec.name, err)

			continue
		}

		// Benchmark matching
		matchResult := testing.Benchmark(func(b *testing.B) {
			for b.Loop() {
				_ = ec.encoder.Matches(benchmarkPassword, encoded)
			}
		})

		fmt.Printf("%-25s | %-15s | %-15s\n",
			ec.name,
			formatDuration(encodeResult),
			formatDuration(matchResult),
		)
	}

	fmt.Println()
}

func formatDuration(r testing.BenchmarkResult) string {
	nsPerOp := r.NsPerOp()
	switch {
	case nsPerOp < 1000:
		return fmt.Sprintf("%d ns/op", nsPerOp)
	case nsPerOp < 1_000_000:
		return fmt.Sprintf("%.2f us/op", float64(nsPerOp)/1000)
	case nsPerOp < 1_000_000_000:
		return fmt.Sprintf("%.2f ms/op", float64(nsPerOp)/1_000_000)
	default:
		return fmt.Sprintf("%.2f s/op", float64(nsPerOp)/1_000_000_000)
	}
}
