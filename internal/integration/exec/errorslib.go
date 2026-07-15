package exec

import (
	"github.com/coldsmirk/vef-framework-go/js"
)

// errorsLibName is the global binding of the error-classification helpers.
const errorsLibName = "errors"

// errorsLib gives adapter scripts a vocabulary to classify failures:
// errors.upstream(message) throws an exception the invoker records as an
// upstream failure (the external system misbehaved) instead of a script bug.
type errorsLib struct{}

func newErrorsLib() js.Lib {
	return new(errorsLib)
}

func (*errorsLib) Name() string {
	return errorsLibName
}

func (*errorsLib) Install(rt *js.Runtime) error {
	return rt.Set(errorsLibName, map[string]any{
		"upstream": func(message string) (any, error) {
			return nil, &upstreamError{message: message}
		},
	})
}
