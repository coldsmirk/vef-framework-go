package orm

import (
	"context"
	"sync"
)

type commitHooksKey struct{}

// commitHooks collects the after-commit callbacks of one outermost
// transaction. Nested RunInTx calls share the collector rather than
// owning one, because bun implements a nested transaction as a savepoint
// (see BunDB.runInTx) whose release is not a commit.
type commitHooks struct {
	mu   sync.Mutex
	fns  []func(context.Context)
	done bool
}

// add registers fn, reporting false once the scope has closed. A context
// captured by a goroutine outlives its transaction, and a late
// registration on it would otherwise be accepted and never run — silent
// loss, where the caller deserves to hear that the commit it meant to
// hang the work on has already happened.
func (h *commitHooks) add(fn func(context.Context)) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.done {
		return false
	}

	h.fns = append(h.fns, fn)

	return true
}

// mark records the current hook count so a savepoint scope can discard
// everything registered inside it. The nil receiver is the manual-BeginTx
// case, where no collector exists and there is nothing to unwind.
func (h *commitHooks) mark() int {
	if h == nil {
		return 0
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	return len(h.fns)
}

// truncate drops the hooks registered after mark, undoing the
// registrations of a savepoint that rolled back.
func (h *commitHooks) truncate(mark int) {
	if h == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if mark < len(h.fns) {
		h.fns = h.fns[:mark]
	}
}

// run closes the scope, then invokes the drained hooks in registration
// order.
func (h *commitHooks) run(ctx context.Context) {
	h.mu.Lock()
	fns := h.fns
	h.fns = nil
	h.done = true
	h.mu.Unlock()

	if len(fns) == 0 {
		return
	}

	// The commit already happened durably, so a caller that canceled its
	// context — a disconnected HTTP client, an elapsed handler deadline —
	// must not also cancel the follow-up work. Values (trace ids, request
	// id) are kept so hooks still log under the originating request.
	hookCtx := context.WithoutCancel(ctx)

	for _, fn := range fns {
		runCommitHook(hookCtx, fn)
	}
}

// runCommitHook isolates one callback. A panic here must not surface at
// the RunInTx call site, where the transaction has already committed and
// the failure would be misread as a failed commit.
func runCommitHook(ctx context.Context, fn func(context.Context)) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Commit hook panicked: %v", r)
		}
	}()

	fn(ctx)
}

func commitHooksFrom(ctx context.Context) *commitHooks {
	hooks, _ := ctx.Value(commitHooksKey{}).(*commitHooks)

	return hooks
}

func withCommitHooks(ctx context.Context, hooks *commitHooks) context.Context {
	return context.WithValue(ctx, commitHooksKey{}, hooks)
}

// OnCommit registers fn to run once the transaction that owns ctx has
// committed. It is the seam for work that must happen if and only if the
// surrounding unit of work became durable — dispatching an in-process
// event, invalidating a cache — without that work being able to fail the
// transaction.
//
// Semantics:
//   - fn runs after the outermost transaction commits, in registration
//     order. Registrations made inside a nested RunInTx (a savepoint) are
//     discarded when that savepoint rolls back.
//   - fn receives a context detached from cancellation, so a canceled
//     request cannot suppress work whose transaction already committed.
//   - fn cannot report failure: by the time it runs there is nothing left
//     to roll back. Callbacks own their own error handling.
//   - Calling outside an open RunInTx / RunInReadOnlyTx scope returns
//     ErrNoCommitScope; nothing is registered. That covers a context whose
//     transaction has already committed, so registering from a goroutine
//     that outlived the scope reports instead of silently never running.
func OnCommit(ctx context.Context, fn func(context.Context)) error {
	hooks := commitHooksFrom(ctx)
	if hooks == nil || !hooks.add(fn) {
		return ErrNoCommitScope
	}

	return nil
}
