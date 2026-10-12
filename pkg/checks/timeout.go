// This file centralizes configured and default deadlines for health checks.
// It depends on context deadlines so every protocol honors timeout_ms consistently.
package checks

import (
	"context"
	"net"
	"strings"
	"time"
)

func configuredCheckTimeout(check CheckConfig) time.Duration {
	if check.TimeoutMS > 0 {
		return time.Duration(check.TimeoutMS) * time.Millisecond
	}

	switch strings.ToLower(check.Type) {
	case "tcp", "redis":
		return 3 * time.Second
	case "smtp":
		return 10 * time.Second
	default:
		return 4 * time.Second
	}
}

func contextWithCheckTimeout(parent context.Context, check CheckConfig) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, configuredCheckTimeout(check))
}

func contextWithDefaultTimeout(parent context.Context, fallback time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := parent.Deadline(); ok {
		return parent, func() {}
	}
	return context.WithTimeout(parent, fallback)
}

func timeoutFromContext(ctx context.Context, fallback time.Duration) time.Duration {
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 {
			return remaining
		}
	}
	return fallback
}

func setConnectionDeadline(ctx context.Context, conn net.Conn, fallback time.Duration) error {
	deadline := time.Now().Add(fallback)
	if configured, ok := ctx.Deadline(); ok {
		deadline = configured
	}
	return conn.SetDeadline(deadline)
}
