package agent

import (
	"context"
	"errors"
	"net"
	"time"
)

// RecoveryConfig configures error recovery behavior.
type RecoveryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

// DefaultRecoveryConfig returns sensible defaults.
func DefaultRecoveryConfig() RecoveryConfig {
	return RecoveryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
	}
}

// WithRetry wraps a function with retry logic using exponential backoff.
func WithRetry(ctx context.Context, cfg RecoveryConfig, fn func() error) error {
	var err error
	backoff := cfg.InitialBackoff

	for i := 0; i <= cfg.MaxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}

		if !IsTransientError(err) {
			return err // Don't retry non-transient errors
		}

		if i == cfg.MaxRetries {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > cfg.MaxBackoff {
			backoff = cfg.MaxBackoff
		}
	}

	return err
}

// IsTransientError checks if an error is transient and should be retried.
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}

	// Network errors are transient
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}

	// Context deadline exceeded is transient
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	return false
}
