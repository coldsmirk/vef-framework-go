package logx

import (
	"fmt"

	"github.com/coldsmirk/vef-framework-go/logx"
)

// Discard returns a logger that silently drops every message. Useful
// for tests, off-by-default jobs, and any component constructor that
// must remain operable when a caller passes nil for the logger
// dependency. The returned value satisfies the full logx.Logger
// interface so it can be substituted anywhere a real logger is taken.
func Discard() logx.Logger { return discardLogger{} }

type discardLogger struct{}

func (d discardLogger) Named(string) logx.Logger          { return d }
func (d discardLogger) WithCallerSkip(int) logx.Logger    { return d }
func (discardLogger) Enabled(logx.Level) bool             { return false }
func (discardLogger) Sync()                               {}
func (discardLogger) Debug(string)                        {}
func (discardLogger) Debugf(string, ...any)               {}
func (discardLogger) Info(string)                         {}
func (discardLogger) Infof(string, ...any)                {}
func (discardLogger) Warn(string)                         {}
func (discardLogger) Warnf(string, ...any)                {}
func (discardLogger) Error(string)                        {}
func (discardLogger) Errorf(string, ...any)               {}
func (discardLogger) Panic(message string)                { panic(message) }
func (discardLogger) Panicf(template string, args ...any) { panic(fmt.Sprintf(template, args...)) }
