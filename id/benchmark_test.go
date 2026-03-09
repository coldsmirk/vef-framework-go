package id

import (
	"testing"
)

func BenchmarkGenerate(b *testing.B) {
	for b.Loop() {
		_ = Generate()
	}
}

func BenchmarkGenerateUUID(b *testing.B) {
	for b.Loop() {
		_ = GenerateUUID()
	}
}

func BenchmarkSnowflakeIdGenerator(b *testing.B) {
	generator, err := NewSnowflakeIDGenerator(1)
	if err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		_ = generator.Generate()
	}
}

func BenchmarkSnowflakeIDGenerator_Parallel(b *testing.B) {
	generator, err := NewSnowflakeIDGenerator(1)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = generator.Generate()
		}
	})
}

func BenchmarkXIDGenerator(b *testing.B) {
	generator := NewXIDGenerator()

	for b.Loop() {
		_ = generator.Generate()
	}
}

func BenchmarkXIDGenerator_Parallel(b *testing.B) {
	generator := NewXIDGenerator()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = generator.Generate()
		}
	})
}

func BenchmarkUUIDGenerator(b *testing.B) {
	generator := NewUUIDGenerator()

	for b.Loop() {
		_ = generator.Generate()
	}
}

func BenchmarkUUIDGenerator_Parallel(b *testing.B) {
	generator := NewUUIDGenerator()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = generator.Generate()
		}
	})
}

func BenchmarkRandomIDGenerator_Short(b *testing.B) {
	generator := NewRandomIDGenerator(WithAlphabet("0123456789abcdef"), WithLength(8))

	for b.Loop() {
		_ = generator.Generate()
	}
}

func BenchmarkRandomIDGenerator_Medium(b *testing.B) {
	generator := NewRandomIDGenerator(WithAlphabet("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"), WithLength(21))

	for b.Loop() {
		_ = generator.Generate()
	}
}

func BenchmarkRandomIDGenerator_Long(b *testing.B) {
	generator := NewRandomIDGenerator(WithAlphabet("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_-"), WithLength(64))

	for b.Loop() {
		_ = generator.Generate()
	}
}

func BenchmarkRandomIDGenerator_Parallel(b *testing.B) {
	generator := NewRandomIDGenerator(WithAlphabet("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"), WithLength(21))

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = generator.Generate()
		}
	})
}

func BenchmarkDefaultGenerators(b *testing.B) {
	b.Run("DefaultXIDGenerator", func(b *testing.B) {
		b.ResetTimer()

		for b.Loop() {
			_ = DefaultXIDGenerator.Generate()
		}
	})

	b.Run("DefaultUUIDGenerator", func(b *testing.B) {
		b.ResetTimer()

		for b.Loop() {
			_ = DefaultUUIDGenerator.Generate()
		}
	})

	b.Run("DefaultSnowflakeIDGenerator", func(b *testing.B) {
		b.ResetTimer()

		for b.Loop() {
			_ = DefaultSnowflakeIDGenerator.Generate()
		}
	})
}

func BenchmarkMemoryAllocation(b *testing.B) {
	b.Run("SnowflakeID", func(b *testing.B) {
		generator, _ := NewSnowflakeIDGenerator(1)

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			_ = generator.Generate()
		}
	})

	b.Run("XID", func(b *testing.B) {
		generator := NewXIDGenerator()

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			_ = generator.Generate()
		}
	})

	b.Run("UUID", func(b *testing.B) {
		generator := NewUUIDGenerator()

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			_ = generator.Generate()
		}
	})

	b.Run("RandomID", func(b *testing.B) {
		generator := NewRandomIDGenerator(WithAlphabet("0123456789abcdefghijklmnopqrstuvwxyz"), WithLength(21))

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			_ = generator.Generate()
		}
	})
}

func BenchmarkConcurrentPerformance(b *testing.B) {
	b.Run("SnowflakeIDConcurrent", func(b *testing.B) {
		generator, _ := NewSnowflakeIDGenerator(1)

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = generator.Generate()
			}
		})
	})

	b.Run("XIDConcurrent", func(b *testing.B) {
		generator := NewXIDGenerator()

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = generator.Generate()
			}
		})
	})

	b.Run("UUIDConcurrent", func(b *testing.B) {
		generator := NewUUIDGenerator()

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = generator.Generate()
			}
		})
	})

	b.Run("RandomIDConcurrent", func(b *testing.B) {
		generator := NewRandomIDGenerator(WithAlphabet("0123456789abcdefghijklmnopqrstuvwxyz"), WithLength(21))

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = generator.Generate()
			}
		})
	})
}
