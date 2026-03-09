package stream

// Options configures the stream behavior.
type Options struct {
	SendReasoning bool
	SendSources   bool
	SendStart     bool
	SendFinish    bool
	OnError       func(err error) string
	OnFinish      func(content string)
	GenerateID    func(prefix string) string
}

// DefaultOptions returns the default stream options.
func DefaultOptions() Options {
	return Options{
		SendReasoning: true,
		SendSources:   true,
		SendStart:     true,
		SendFinish:    true,
		OnError: func(err error) string {
			return err.Error()
		},
	}
}
