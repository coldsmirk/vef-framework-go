package command

import "context"

type rpcSubmissionKey struct{}

type rpcSubmission struct {
	form           map[string]any
	resolveGlobals func(context.Context) (map[string]any, error)
}

// WithRPCSubmission carries the original numbers and the trusted globals
// resolver through the public service for keyed RPC calls. A replay uses
// the original submission without requiring a fresh host snapshot.
func WithRPCSubmission(ctx context.Context, key string, rawForm any, globals func(context.Context) (map[string]any, error)) context.Context {
	if key == "" {
		return ctx
	}

	form, _ := rawForm.(map[string]any)

	return context.WithValue(ctx, rpcSubmissionKey{}, rpcSubmission{form, globals})
}

// takeRPCSubmission keeps a host's re-entrant command independent of the
// outer RPC. Service callers supply their own form and globals directly.
func takeRPCSubmission(ctx context.Context, form map[string]any) (context.Context, rpcSubmission) {
	if submission, ok := ctx.Value(rpcSubmissionKey{}).(rpcSubmission); ok {
		return context.WithValue(ctx, rpcSubmissionKey{}, nil), submission
	}

	return ctx, rpcSubmission{form: form}
}
