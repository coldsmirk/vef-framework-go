package js_test

import (
	"context"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/js"
)

// TestRunString tests script execution and error surfacing.
func TestRunString(t *testing.T) {
	rt := newStdRuntime(t)

	t.Run("ReturnsValue", func(t *testing.T) {
		result, err := rt.RunString(t.Context(), `1 + 2`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, int64(3), result.ToInteger(), "Result should match the evaluated expression")
	})

	t.Run("ReferenceError", func(t *testing.T) {
		_, err := rt.RunString(t.Context(), `nonExistentVariable`)
		require.Error(t, err, "Undefined variable should return an error")
		assert.Contains(t, err.Error(), "not defined", "Error should mention the undefined reference")
	})

	t.Run("SyntaxError", func(t *testing.T) {
		_, err := rt.RunString(t.Context(), `const x = ;`)
		require.Error(t, err, "Invalid syntax should return an error")
	})

	t.Run("TypeError", func(t *testing.T) {
		_, err := rt.RunString(t.Context(), `null.toString()`)
		require.Error(t, err, "Type error should be returned")
	})
}

// TestRunCancellation tests context propagation into running scripts.
func TestRunCancellation(t *testing.T) {
	t.Run("CanceledBeforeRun", func(t *testing.T) {
		rt := newStdRuntime(t)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := rt.RunString(ctx, `1 + 1`)
		require.ErrorIs(t, err, context.Canceled, "A canceled context should fail the run immediately")
	})

	t.Run("InterruptsInfiniteLoop", func(t *testing.T) {
		rt := newStdRuntime(t)

		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()

		_, err := rt.RunString(ctx, `for (;;) {}`)
		require.ErrorIs(t, err, context.DeadlineExceeded, "An expired context should interrupt the script")
	})

	t.Run("RuntimeReusableAfterInterrupt", func(t *testing.T) {
		rt := newStdRuntime(t)

		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()

		_, err := rt.RunString(ctx, `for (;;) {}`)
		require.ErrorIs(t, err, context.DeadlineExceeded, "An expired context should interrupt the script")

		result, err := rt.RunString(t.Context(), `1 + 1`)
		require.NoError(t, err, "Runtime should be reusable after an interrupt")
		assert.Equal(t, int64(2), result.ToInteger(), "Reused runtime should evaluate correctly")
	})
}

// TestWithRunTimeout tests the per-runtime execution cap.
func TestWithRunTimeout(t *testing.T) {
	t.Run("CapsUnboundedContext", func(t *testing.T) {
		rt := newStdRuntime(t, js.WithRunTimeout(100*time.Millisecond))

		_, err := rt.RunString(context.Background(), `for (;;) {}`)
		require.ErrorIs(t, err, context.DeadlineExceeded, "Run timeout should interrupt the script under an unbounded context")
	})

	t.Run("FastScriptUnaffected", func(t *testing.T) {
		rt := newStdRuntime(t, js.WithRunTimeout(time.Minute))

		result, err := rt.RunString(t.Context(), `1 + 1`)
		require.NoError(t, err, "A fast script should finish well within the cap")
		assert.Equal(t, int64(2), result.ToInteger(), "Result should match the evaluated expression")
	})
}

// TestWithMaxCallStackSize tests the recursion depth guard.
func TestWithMaxCallStackSize(t *testing.T) {
	rt := newStdRuntime(t, js.WithMaxCallStackSize(64))

	_, err := rt.RunString(t.Context(), `function f() { return f(); } f()`)
	require.Error(t, err, "Runaway recursion should fail once the stack bound is hit")

	var overflowErr *goja.StackOverflowError

	assert.ErrorAs(t, err, &overflowErr, "Error should be a stack overflow")
}

// TestSet tests Go-to-JavaScript value binding.
func TestSet(t *testing.T) {
	rt := newStdRuntime(t)

	t.Run("Struct", func(t *testing.T) {
		type User struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Age   int    `json:"age"`
		}

		require.NoError(t, rt.Set("user", User{Name: "alice", Email: "alice@example.com", Age: 30}), "Set should bind the struct")

		result, err := rt.RunString(t.Context(), `user.name + ':' + user.age`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "alice:30", result.String(), "Struct fields should be visible through json tags")
	})

	t.Run("Slice", func(t *testing.T) {
		require.NoError(t, rt.Set("numbers", []int{5, 2, 8}), "Set should bind the slice")

		result, err := rt.RunString(t.Context(), `numbers.reduce((sum, n) => sum + n, 0)`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, int64(15), result.ToInteger(), "Slice elements should be iterable in scripts")
	})

	t.Run("Function", func(t *testing.T) {
		require.NoError(t, rt.Set("double", func(n int) int { return n * 2 }), "Set should bind the function")

		result, err := rt.RunString(t.Context(), `double(21)`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, int64(42), result.ToInteger(), "Bound Go function should be callable")
	})
}

// TestContext tests the execution context exposed to host libraries.
func TestContext(t *testing.T) {
	rt := newStdRuntime(t)

	t.Run("BackgroundWhenIdle", func(t *testing.T) {
		assert.Equal(t, context.Background(), rt.Context(), "Idle runtime should expose the background context")
	})

	t.Run("ExposesRunContext", func(t *testing.T) {
		var sawDeadline bool

		require.NoError(t, rt.Set("probe", func() {
			_, sawDeadline = rt.Context().Deadline()
		}), "Set should bind the probe")

		ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
		defer cancel()

		_, err := rt.RunString(ctx, `probe()`)
		require.NoError(t, err, "Probe script should run")
		assert.True(t, sawDeadline, "Host function should observe the run context deadline")
	})

	t.Run("ResetAfterRun", func(t *testing.T) {
		_, err := rt.RunString(t.Context(), `1 + 1`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, context.Background(), rt.Context(), "Context should reset to background after the run")
	})
}

// TestAsFunction tests the host-side function handle over script values.
func TestAsFunction(t *testing.T) {
	rt := newStdRuntime(t)

	t.Run("CallsWithArguments", func(t *testing.T) {
		value, err := rt.RunString(t.Context(), `(function (a, b) { return a + b.suffix })`)
		require.NoError(t, err, "Function expression should evaluate")

		fn, ok := rt.AsFunction(value)
		require.True(t, ok, "A function value should convert")

		result, err := fn("x-", map[string]any{"suffix": "y"})
		require.NoError(t, err, "Call should succeed")
		assert.Equal(t, "x-y", result.Export(), "Arguments should reach the script function")
	})

	t.Run("ScriptExceptionSurfacesAsError", func(t *testing.T) {
		value, err := rt.RunString(t.Context(), `(function () { throw new Error('boom') })`)
		require.NoError(t, err, "Function expression should evaluate")

		fn, ok := rt.AsFunction(value)
		require.True(t, ok, "A function value should convert")

		_, err = fn()
		require.Error(t, err, "A thrown exception should surface as an error")
		assert.Contains(t, err.Error(), "boom", "Error should carry the exception message")
	})

	t.Run("NonFunctionIsRejected", func(t *testing.T) {
		value, err := rt.RunString(t.Context(), `42`)
		require.NoError(t, err, "Script should execute successfully")

		_, ok := rt.AsFunction(value)
		assert.False(t, ok, "A non-function value should not convert")
	})
}
