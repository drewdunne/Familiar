package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestAPIParser_Retry(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&attempts, 1)

		if attempt < 3 {
			// First two attempts fail
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		// Third attempt succeeds
		response := map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": `{"instructions": "test", "requested_actions": [], "confidence": 0.9}`,
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	parser := New("test-key", "claude-sonnet-4-20250514",
		WithBaseURL(server.URL),
		WithRetries(3),
	)

	result, err := parser.Parse(context.Background(), "test")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if result.Instructions != "test" {
		t.Errorf("Instructions = %q, want %q", result.Instructions, "test")
	}

	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestAPIParser_ExhaustedRetries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	parser := New("test-key", "claude-sonnet-4-20250514",
		WithBaseURL(server.URL),
		WithRetries(2),
	)

	_, err := parser.Parse(context.Background(), "test")
	if err == nil {
		t.Error("Parse() should error after exhausting retries")
	}
}

func TestAPIParser_NoRetryOn4xx(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer server.Close()

	parser := New("test-key", "claude-sonnet-4-20250514",
		WithBaseURL(server.URL),
		WithRetries(3),
	)

	_, err := parser.Parse(context.Background(), "test")
	if err == nil {
		t.Error("Parse() should error on 4xx")
	}

	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (should not retry on 4xx)", attempts)
	}
}

func TestAPIParser_DefaultNoRetry(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	// No WithRetries option - should default to 1 (no retries)
	parser := New("test-key", "claude-sonnet-4-20250514",
		WithBaseURL(server.URL),
	)

	_, err := parser.Parse(context.Background(), "test")
	if err == nil {
		t.Error("Parse() should error on 5xx")
	}

	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (default should be no retries)", attempts)
	}
}
