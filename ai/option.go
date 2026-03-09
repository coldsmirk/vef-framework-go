package ai

// Option is a functional option for configuring AI operations.
type Option func(*Options)

// Options contains runtime configuration for AI operations.
type Options struct {
	// Temperature controls randomness in the output.
	Temperature *float64
	// MaxTokens limits the maximum number of tokens to generate.
	MaxTokens *int
	// StopSequences specifies sequences that stop generation.
	StopSequences []string
	// Meta contains additional key-value pairs.
	Meta map[string]string
}

// NewOptions creates a new Options with defaults.
func NewOptions() *Options {
	return &Options{
		Meta: make(map[string]string),
	}
}

// Apply applies the given options to this Options instance.
func (o *Options) Apply(opts ...Option) *Options {
	for _, opt := range opts {
		opt(o)
	}

	return o
}

// WithTemperature sets the temperature parameter.
func WithTemperature(t float64) Option {
	return func(o *Options) {
		o.Temperature = &t
	}
}

// WithMaxTokens sets the maximum tokens parameter.
func WithMaxTokens(n int) Option {
	return func(o *Options) {
		o.MaxTokens = &n
	}
}

// WithStopSequences sets the stop sequences.
func WithStopSequences(seqs ...string) Option {
	return func(o *Options) {
		o.StopSequences = seqs
	}
}

// WithMeta adds a meta key-value pair.
func WithMeta(key, value string) Option {
	return func(o *Options) {
		if o.Meta == nil {
			o.Meta = make(map[string]string)
		}

		o.Meta[key] = value
	}
}
