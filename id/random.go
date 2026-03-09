package id

import nid "github.com/matoous/go-nanoid/v2"

const (
	DefaultRandomIDGeneratorAlphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	DefaultRandomIDGeneratorLength   = 32
)

type randomIDGenerator struct {
	alphabet string
	length   int
}

// Generate creates a new random ID using the configured alphabet and length.
func (g *randomIDGenerator) Generate() string {
	return nid.MustGenerate(g.alphabet, g.length)
}

// RandomIDGeneratorOption configures a randomIDGenerator instance.
type RandomIDGeneratorOption func(*randomIDGenerator)

// WithAlphabet sets the character set for random ID generation.
func WithAlphabet(alphabet string) RandomIDGeneratorOption {
	return func(g *randomIDGenerator) {
		g.alphabet = alphabet
	}
}

// WithLength sets the length of generated random IDs.
func WithLength(length int) RandomIDGeneratorOption {
	return func(g *randomIDGenerator) {
		g.length = length
	}
}

// NewRandomIDGenerator creates a new random ID generator with optional configuration.
// Defaults to alphanumeric alphabet (62 chars) and length of 32.
func NewRandomIDGenerator(opts ...RandomIDGeneratorOption) IDGenerator {
	g := &randomIDGenerator{
		alphabet: DefaultRandomIDGeneratorAlphabet,
		length:   DefaultRandomIDGeneratorLength,
	}

	for _, opt := range opts {
		opt(g)
	}

	return g
}
