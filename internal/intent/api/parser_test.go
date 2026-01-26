package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/drewdunne/familiar/internal/intent"
)

func TestAPIParser_Parse_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "invalid request"}`))
	}))
	defer server.Close()

	parser := New("test-key", "claude-sonnet-4-20250514", WithBaseURL(server.URL))
	_, err := parser.Parse(context.Background(), "test")
	if err == nil {
		t.Error("Parse() should error on non-200 status")
	}
}

func TestAPIParser_Parse_EmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{"content": []map[string]interface{}{}}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	parser := New("test-key", "claude-sonnet-4-20250514", WithBaseURL(server.URL))
	_, err := parser.Parse(context.Background(), "test")
	if err == nil {
		t.Error("Parse() should error on empty content")
	}
}

func TestAPIParser_Parse_SimpleInstruction(t *testing.T) {
	// Mock Anthropic API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Header.Get("x-api-key") != "test-key" {
			t.Error("Missing API key header")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("Missing anthropic-version header")
		}

		// Return mock response
		response := map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": `{"instructions": "Fix the bug in the login handler", "requested_actions": [], "confidence": 0.95}`,
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	parser := New("test-key", "claude-sonnet-4-20250514", WithBaseURL(server.URL))

	result, err := parser.Parse(context.Background(), "Fix the bug in the login handler")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if result.Instructions != "Fix the bug in the login handler" {
		t.Errorf("Instructions = %q, want %q", result.Instructions, "Fix the bug in the login handler")
	}
	if len(result.RequestedActions) != 0 {
		t.Errorf("RequestedActions = %v, want empty", result.RequestedActions)
	}
	if result.Confidence < 0.9 {
		t.Errorf("Confidence = %f, want >= 0.9", result.Confidence)
	}
}

func TestAPIParser_Parse_WithMergeRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": `{"instructions": "Fix the tests and merge when ready", "requested_actions": ["merge"], "confidence": 0.92}`,
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	parser := New("test-key", "claude-sonnet-4-20250514", WithBaseURL(server.URL))

	result, err := parser.Parse(context.Background(), "@familiar fix the tests and merge when ready")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if !result.HasAction(intent.ActionMerge) {
		t.Error("Should have detected merge action")
	}
}

func TestAPIParser_Parse_MultipleActions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": `{"instructions": "Address the feedback, approve, and merge", "requested_actions": ["approve", "merge"], "confidence": 0.88}`,
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	parser := New("test-key", "claude-sonnet-4-20250514", WithBaseURL(server.URL))

	result, err := parser.Parse(context.Background(), "Address the feedback, approve this, and merge")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if !result.HasAction(intent.ActionMerge) {
		t.Error("Should have detected merge action")
	}
	if !result.HasAction(intent.ActionApprove) {
		t.Error("Should have detected approve action")
	}
}
