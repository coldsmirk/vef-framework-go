package jsconsole

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coldsmirk/vef-framework-go/js"
	"github.com/coldsmirk/vef-framework-go/logx"
)

// Name is the library identifier and the global binding installed into the
// runtime.
const Name = "console"

// lib exposes script logging as the global "console" object:
//
//	console.info('processed', count, payload)
//	console.warn(...)
//	console.error(...)
//
// Arguments are joined with a space; strings pass through verbatim, errors
// render their message, and everything else is JSON-encoded, so objects stay
// readable in the log output.
type lib struct {
	logger logx.Logger
}

// New builds the console library over logger. Pass a Named logger (e.g.
// logger.Named("js")) when script output should be distinguishable from host
// logs.
func New(logger logx.Logger) js.Lib {
	return &lib{logger: logger}
}

func (*lib) Name() string {
	return Name
}

func (l *lib) Install(rt *js.Runtime) error {
	return rt.Set(Name, map[string]any{
		"info": func(args ...any) {
			l.logger.Info(formatArgs(args))
		},
		"warn": func(args ...any) {
			l.logger.Warn(formatArgs(args))
		},
		"error": func(args ...any) {
			l.logger.Error(formatArgs(args))
		},
	})
}

// formatArgs renders the console arguments into one log message.
func formatArgs(args []any) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, formatArg(arg))
	}

	return strings.Join(parts, " ")
}

// formatArg renders a single argument: strings verbatim, errors by message,
// anything else as JSON with a fmt fallback for unmarshalable values.
func formatArg(arg any) string {
	switch v := arg.(type) {
	case string:
		return v
	case error:
		return v.Error()
	default:
		if data, err := json.Marshal(v); err == nil {
			return string(data)
		}

		return fmt.Sprintf("%v", v)
	}
}
