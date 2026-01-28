package agent

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestDefaultRecoveryConfig(t *testing.T) {
	cfg := DefaultRecoveryConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", cfg.MaxRetries)
	}
	if cfg.InitialBackoff != 1*time.Second {
		t.Errorf("expected InitialBackoff=1s, got %v", cfg.InitialBackoff)
	}
	if cfg.MaxBackoff != 30*time.Second {
		t.Errorf("expected MaxBackoff=30s, got %v", cfg.MaxBackoff)
	}
}

func TestWithRetry_Success(t *testing.T) {
	// Test that a function that succeeds on first try returns immediately
	ctx := context.Background()
	cfg := RecoveryConfig{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
	}

	callCount := 0
	err := WithRetry(ctx, cfg, func() error {
		callCount++
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected function to be called once, got %d calls", callCount)
	}
}

func TestWithRetry_RetrySuccess(t *testing.T) {
	// Test that a function that fails then succeeds is retried
	ctx := context.Background()
	cfg := RecoveryConfig{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
	}

	callCount := 0
	err := WithRetry(ctx, cfg, func() error {
		callCount++
		if callCount < 3 {
			// Return a transient error (timeout)
			return &net.OpError{Op: "read", Err: &timeoutError{}}
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected no error after retry, got %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected function to be called 3 times, got %d calls", callCount)
	}
}

func TestWithRetry_MaxRetriesExceeded(t *testing.T) {
	// Test that after max retries, the last error is returned
	ctx := context.Background()
	cfg := RecoveryConfig{
		MaxRetries:     2,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
	}

	callCount := 0
	transientErr := &net.OpError{Op: "read", Err: &timeoutError{}}
	err := WithRetry(ctx, cfg, func() error {
		callCount++
		return transientErr
	})

	if err == nil {
		t.Error("expected error after max retries, got nil")
	}
	// Should be called MaxRetries + 1 times (initial + retries)
	expectedCalls := cfg.MaxRetries + 1
	if callCount != expectedCalls {
		t.Errorf("expected function to be called %d times, got %d calls", expectedCalls, callCount)
	}
}

func TestWithRetry_ContextCancelled(t *testing.T) {
	// Test that context cancellation stops retries
	ctx, cancel := context.WithCancel(context.Background())
	cfg := RecoveryConfig{
		MaxRetries:     10,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     1 * time.Second,
	}

	callCount := 0
	err := WithRetry(ctx, cfg, func() error {
		callCount++
		if callCount == 1 {
			// Cancel context after first call
			cancel()
		}
		return &net.OpError{Op: "read", Err: &timeoutError{}}
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
	// Should only be called once before context cancellation stops retries
	if callCount != 1 {
		t.Errorf("expected function to be called 1 time before cancellation, got %d calls", callCount)
	}
}

func TestWithRetry_NonTransientError(t *testing.T) {
	// Test that non-transient errors are not retried
	ctx := context.Background()
	cfg := RecoveryConfig{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
	}

	callCount := 0
	permanentErr := errors.New("permanent error")
	err := WithRetry(ctx, cfg, func() error {
		callCount++
		return permanentErr
	})

	if err != permanentErr {
		t.Errorf("expected permanent error, got %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected function to be called once for non-transient error, got %d calls", callCount)
	}
}

func TestWithRetry_ExponentialBackoff(t *testing.T) {
	// Test that backoff increases exponentially and respects MaxBackoff
	ctx := context.Background()
	cfg := RecoveryConfig{
		MaxRetries:     4,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     50 * time.Millisecond, // Should cap at this
	}

	var callTimes []time.Time
	err := WithRetry(ctx, cfg, func() error {
		callTimes = append(callTimes, time.Now())
		return &net.OpError{Op: "read", Err: &timeoutError{}}
	})

	if err == nil {
		t.Error("expected error after max retries, got nil")
	}

	// Check that we got the expected number of calls
	if len(callTimes) != 5 { // initial + 4 retries
		t.Errorf("expected 5 calls, got %d", len(callTimes))
	}

	// Verify backoff is roughly exponential (with some tolerance for timing)
	// Expected: 10ms, 20ms, 40ms, 50ms (capped)
	for i := 1; i < len(callTimes); i++ {
		delay := callTimes[i].Sub(callTimes[i-1])
		if delay < 5*time.Millisecond {
			t.Errorf("delay %d was too short: %v", i, delay)
		}
	}
}

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		transient bool
	}{
		{
			name:      "nil error",
			err:       nil,
			transient: false,
		},
		{
			name:      "regular error",
			err:       errors.New("regular error"),
			transient: false,
		},
		{
			name:      "timeout error",
			err:       &net.OpError{Op: "read", Err: &timeoutError{}},
			transient: true,
		},
		{
			name:      "temporary error",
			err:       &net.OpError{Op: "dial", Err: &temporaryError{}},
			transient: true,
		},
		{
			name:      "context deadline exceeded",
			err:       context.DeadlineExceeded,
			transient: true,
		},
		{
			name:      "wrapped deadline exceeded",
			err:       errors.New("operation failed: " + context.DeadlineExceeded.Error()),
			transient: false, // Only direct or wrapped-via-errors.Is should match
		},
		{
			name:      "wrapped network error",
			err:       &wrappedNetError{cause: &net.OpError{Op: "read", Err: &timeoutError{}}},
			transient: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTransientError(tt.err)
			if result != tt.transient {
				t.Errorf("IsTransientError(%v) = %v, want %v", tt.err, result, tt.transient)
			}
		})
	}
}

// timeoutError is a test helper that implements net.Error with Timeout() = true
type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return false }

// temporaryError is a test helper that implements net.Error with Temporary() = true
type temporaryError struct{}

func (e *temporaryError) Error() string   { return "temporary" }
func (e *temporaryError) Timeout() bool   { return false }
func (e *temporaryError) Temporary() bool { return true }

// wrappedNetError wraps a net.Error to test errors.As
type wrappedNetError struct {
	cause error
}

func (e *wrappedNetError) Error() string { return "wrapped: " + e.cause.Error() }
func (e *wrappedNetError) Unwrap() error { return e.cause }
