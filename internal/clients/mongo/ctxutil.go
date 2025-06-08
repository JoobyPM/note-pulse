package mongo

import (
	"context"
	"time"
)

// OpTimeout is the default timeout for MongoDB operations
const OpTimeout = 5 * time.Second

// WithRepoTimeout returns ctx unchanged when it is already ≤ d away from expiring;
// otherwise it wraps ctx in context.WithTimeout(ctx, d).
// The returned cancel is always safe to defer: when no new context
// is created we return a stub that does nothing, so callers can write:
//
//	ctx, cancel := WithRepoTimeout(parentCtx, d)
//	defer cancel() // safe even if cancel is a no-op
//
// without needing extra branching or nil checks.
func WithRepoTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if err := ctx.Err(); err != nil {
		// Parent context is already canceled or deadline exceeded.
		// Return the original context plus a dummy cancel so the
		// caller can still defer cancel() unconditionally.
		return ctx, func() {}
	}
	if dl, ok := ctx.Deadline(); ok && time.Until(dl) <= d {
		// The existing deadline is sooner than (or equal to) the
		// requested timeout, so keep the stricter deadline. Again,
		// provide a no-op cancel for ease of use.
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}
