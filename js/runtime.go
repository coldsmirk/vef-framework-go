package js

import (
	"context"
	"errors"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja/parser"
)

// Runtime is a sandboxed JavaScript runtime bound to a single goroutine. It
// carries the engine baseline plus the catalog libraries activated at
// creation; scripts can touch nothing beyond what was installed.
//
// A Runtime is not safe for concurrent use, and only one Run call may be in
// flight at a time.
type Runtime struct {
	vm         *goja.Runtime
	ctx        context.Context
	runTimeout time.Duration
}

// runtimeConfig collects the settings resolved from RuntimeOptions.
type runtimeConfig struct {
	enabledLibs      []string
	runTimeout       time.Duration
	maxCallStackSize int
}

// RuntimeOption customizes a single runtime produced by Engine.NewRuntime.
type RuntimeOption func(*runtimeConfig)

// EnableLibs activates catalog libraries by name for the new runtime.
// Installation follows the argument order; enabling a name twice is a no-op.
// An unknown name fails NewRuntime with ErrLibNotFound.
func EnableLibs(names ...string) RuntimeOption {
	return func(c *runtimeConfig) {
		c.enabledLibs = append(c.enabledLibs, names...)
	}
}

// WithRunTimeout caps the duration of every Run call on the runtime. It
// combines with the caller's context: whichever deadline is earlier wins.
func WithRunTimeout(d time.Duration) RuntimeOption {
	return func(c *runtimeConfig) {
		c.runTimeout = d
	}
}

// WithMaxCallStackSize bounds the JavaScript call stack depth, guarding
// against runaway recursion.
func WithMaxCallStackSize(size int) RuntimeOption {
	return func(c *runtimeConfig) {
		c.maxCallStackSize = size
	}
}

// newRuntime assembles a bare goja runtime configured for framework use.
func newRuntime(cfg runtimeConfig) *Runtime {
	vm := goja.New()
	vm.SetParserOptions(parser.WithDisableSourceMaps)
	vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))

	if cfg.maxCallStackSize > 0 {
		vm.SetMaxCallStackSize(cfg.maxCallStackSize)
	}

	return &Runtime{vm: vm, runTimeout: cfg.runTimeout}
}

// RunProgram executes a pre-compiled program. Cancellation of ctx interrupts
// the running script and, through Context, any in-flight host library IO;
// the returned error is then the context's error.
func (r *Runtime) RunProgram(ctx context.Context, program *Program) (Value, error) {
	return r.run(ctx, func() (Value, error) {
		return r.vm.RunProgram(program)
	})
}

// RunString compiles and executes source with the same cancellation semantics
// as RunProgram.
func (r *Runtime) RunString(ctx context.Context, source string) (Value, error) {
	return r.run(ctx, func() (Value, error) {
		return r.vm.RunString(source)
	})
}

// Set binds a value as a global variable of the runtime.
func (r *Runtime) Set(name string, value any) error {
	return r.vm.Set(name, value)
}

// Context returns the context of the in-flight Run call, or
// context.Background when the runtime is idle. Host libraries must issue IO
// through it so cancellation reaches blocking Go calls.
func (r *Runtime) Context() context.Context {
	if r.ctx != nil {
		return r.ctx
	}

	return context.Background()
}

// VM exposes the underlying goja runtime for advanced library authoring
// (custom object construction, exception throwing). Prefer the Runtime
// methods for everything else.
func (r *Runtime) VM() *goja.Runtime {
	return r.vm
}

// run executes exec under ctx: the context is published for host libraries,
// its cancellation interrupts the JavaScript loop, and the interrupt
// lifecycle is fully settled before the runtime is handed back.
func (r *Runtime) run(ctx context.Context, exec func() (Value, error)) (value Value, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if r.runTimeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, r.runTimeout)
		defer cancel()
	}

	r.ctx = ctx

	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(interrupted)

		r.vm.Interrupt(ctx.Err())
	})

	defer func() {
		if !stop() {
			// The interrupt is in flight; wait for it to land so
			// ClearInterrupt cannot race it and poison the next run.
			<-interrupted
		}

		r.vm.ClearInterrupt()
		r.ctx = nil

		if err != nil && ctx.Err() != nil {
			if _, ok := errors.AsType[*goja.InterruptedError](err); ok {
				value, err = nil, ctx.Err()
			}
		}
	}()

	return exec()
}
