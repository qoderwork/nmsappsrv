package operation

import (
	"context"

	"golang.org/x/time/rate"
)

// rateLimiter is a single-process global limiter that mirrors Java's
// `rateLimiterOperation = RateLimiter.create(200)` from
// `corefunction/component/Receiver.java`. One limiter covers the entire
// dispatcher so a burst of `Reboot` operations cannot starve `GetParameterValues`
// (or vice versa) — Java's behaviour.
//
// The underlying `rate.Limiter` uses a token bucket: 200 tokens/s refill, and
// each `Wait(ctx)` call consumes one token. When the bucket is empty the call
// blocks until a token is available or `ctx` is cancelled.
var rateLimiter = rate.NewLimiter(rate.Limit(200), 200)

// waitRateLimit blocks the caller until the dispatcher is allowed to dispatch
// the next operation under the global 200 ops/s budget. Respects `ctx` so the
// worker's `Stop()` can unblock a stuck dispatcher promptly.
func waitRateLimit(ctx context.Context) error {
	return rateLimiter.Wait(ctx)
}
