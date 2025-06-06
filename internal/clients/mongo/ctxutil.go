package mongo

import (
	"context"
	"time"
)

// MongoOpTimeout is the default timeout for MongoDB operations
const MongoOpTimeout = 5 * time.Second

// WithRepoTimeout returns ctx unchanged when it is already ≤ d away from expiring;
// otherwise it wraps it in context.WithTimeout(ctx, d).  The returned cancel
// is always safe to defer - it's a no-op when no new context was created.
func WithRepoTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if err := ctx.Err(); err != nil {
		return ctx, func() {}
	}
	if dl, ok := ctx.Deadline(); ok && time.Until(dl) <= d {
		return ctx, func() {} // keep caller's stricter deadline
	}
	return context.WithTimeout(ctx, d)
}
