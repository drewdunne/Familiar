# Phase 4: Intent Parsing Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

> **Note:** This plan may need adjustment based on patterns established in Phases 1-3. Review previous implementations before starting.

**Goal:** Extract user intent from comments using Claude API, identify requested actions (merge, approve, etc.), design interface for future CLI-based strategy.

**Architecture:** IntentParser interface with API-based implementation. Parser extracts structured intent including requested actions and instructions. Designed for future CLI-based implementation.

**Tech Stack:** Go 1.25, Anthropic API (HTTP), existing event package

**Prerequisites:** Phases 1-3 complete

---

## Task 1: Define Intent Types

**Files:**
- Create: `internal/intent/intent.go`

**Step 1: Create intent types**

Create `internal/intent/intent.go`:
```go
package intent

// Action represents a requested privileged action.
type Action string

const (
	ActionMerge          Action = "merge"
	ActionApprove        Action = "approve"
	ActionDismissReviews Action = "dismiss_reviews"
)

// ParsedIntent represents the extracted intent from user input.
type ParsedIntent struct {
	// Instructions is the core user request/instructions.
	Instructions string

	// RequestedActions are privileged actions the user explicitly requested.
	RequestedActions []Action

	// Confidence is how confident the parser is (0.0 to 1.0).
	Confidence float64

	// Raw is the original input text.
	Raw string
}

// HasAction checks if a specific action was requested.
func (p *ParsedIntent) HasAction(action Action) bool {
	for _, a := range p.RequestedActions {
		if a == action {
			return true
		}
	}
	return false
}
```

**Step 2: Commit**

```bash
git add internal/intent/
git commit -m "feat(intent): define intent types"
```

---

## Task 2: Define Parser Interface

**Files:**
- Create: `internal/intent/parser.go`

**Step 1: Create parser interface**

Create `internal/intent/parser.go`:
```go
package intent

import "context"

// Parser extracts intent from user input.
type Parser interface {
	// Parse extracts intent from the given text.
	Parse(ctx context.Context, text string) (*ParsedIntent, error)
}

// Strategy identifies the parsing strategy.
type Strategy string

const (
	StrategyAPI Strategy = "api"
	StrategyCLI Strategy = "cli" // Future
)
```

**Step 2: Commit**

```bash
git add internal/intent/
git commit -m "feat(intent): define parser interface"
```

---

## Task 3: API Parser - Basic Implementation (TDD)

**Files:**
- Create: `internal/intent/api/parser.go`
- Create: `internal/intent/api/parser_test.go`

**Step 1: Write failing test**

Create `internal/intent/api/parser_test.go`:
```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/drewdunne/familiar/internal/intent"
)

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
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/intent/api/... -v
```

**Step 3: Implement API parser**

Create `internal/intent/api/parser.go`:
```go
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/drewdunne/familiar/internal/intent"
)

const defaultBaseURL = "https://api.anthropic.com/v1"

// Parser implements intent.Parser using the Anthropic API.
type Parser struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// Option configures the API parser.
type Option func(*Parser)

// WithBaseURL sets a custom base URL (for testing).
func WithBaseURL(url string) Option {
	return func(p *Parser) {
		p.baseURL = url
	}
}

// New creates a new API-based intent parser.
func New(apiKey, model string, opts ...Option) *Parser {
	p := &Parser{
		apiKey:  apiKey,
		model:   model,
		baseURL: defaultBaseURL,
		client:  &http.Client{},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Parse extracts intent from the given text using Claude API.
func (p *Parser) Parse(ctx context.Context, text string) (*intent.ParsedIntent, error) {
	prompt := buildPrompt(text)

	reqBody := map[string]interface{}{
		"model":      p.model,
		"max_tokens": 1024,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, body)
	}

	var apiResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return parseResponse(apiResp, text)
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

type parsedResponse struct {
	Instructions     string   `json:"instructions"`
	RequestedActions []string `json:"requested_actions"`
	Confidence       float64  `json:"confidence"`
}

func buildPrompt(text string) string {
	return fmt.Sprintf(`Extract the user's intent from this message. Return JSON with:
- instructions: The core request/instructions (what they want done)
- requested_actions: Array of privileged actions explicitly requested. Valid values: "merge", "approve", "dismiss_reviews"
- confidence: How confident you are in the extraction (0.0 to 1.0)

Only include actions in requested_actions if the user EXPLICITLY asks for them.

User message:
%s

Respond with only valid JSON, no other text.`, text)
}

func parseResponse(resp anthropicResponse, originalText string) (*intent.ParsedIntent, error) {
	if len(resp.Content) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}

	var parsed parsedResponse
	if err := json.Unmarshal([]byte(resp.Content[0].Text), &parsed); err != nil {
		return nil, fmt.Errorf("parsing response JSON: %w", err)
	}

	actions := make([]intent.Action, len(parsed.RequestedActions))
	for i, a := range parsed.RequestedActions {
		actions[i] = intent.Action(a)
	}

	return &intent.ParsedIntent{
		Instructions:     parsed.Instructions,
		RequestedActions: actions,
		Confidence:       parsed.Confidence,
		Raw:              originalText,
	}, nil
}
```

**Step 4: Run tests**

```bash
go test ./internal/intent/api/... -v
```

**Step 5: Commit**

```bash
git add internal/intent/
git commit -m "feat(intent): implement API-based intent parser"
```

---

## Task 4: Parser Factory

**Files:**
- Create: `internal/intent/factory.go`
- Create: `internal/intent/factory_test.go`

**Step 1: Write failing test**

Create `internal/intent/factory_test.go`:
```go
package intent

import (
	"testing"

	"github.com/drewdunne/familiar/internal/config"
)

func TestNewParser_APIStrategy(t *testing.T) {
	cfg := &config.Config{
		LLM: config.LLMConfig{
			Strategy: "api",
			API: config.LLMAPIConfig{
				Provider: "anthropic",
				Model:    "claude-sonnet-4-20250514",
				APIKey:   "test-key",
			},
		},
	}

	parser, err := NewParser(cfg)
	if err != nil {
		t.Fatalf("NewParser() error = %v", err)
	}

	if parser == nil {
		t.Error("Parser should not be nil")
	}
}

func TestNewParser_CLIStrategy(t *testing.T) {
	cfg := &config.Config{
		LLM: config.LLMConfig{
			Strategy: "cli",
		},
	}

	_, err := NewParser(cfg)
	if err == nil {
		t.Error("CLI strategy should return error (not implemented)")
	}
}

func TestNewParser_UnknownStrategy(t *testing.T) {
	cfg := &config.Config{
		LLM: config.LLMConfig{
			Strategy: "unknown",
		},
	}

	_, err := NewParser(cfg)
	if err == nil {
		t.Error("Unknown strategy should return error")
	}
}
```

**Step 2: Implement factory**

Create `internal/intent/factory.go`:
```go
package intent

import (
	"fmt"

	"github.com/drewdunne/familiar/internal/config"
	"github.com/drewdunne/familiar/internal/intent/api"
)

// NewParser creates a parser based on the configured strategy.
func NewParser(cfg *config.Config) (Parser, error) {
	switch Strategy(cfg.LLM.Strategy) {
	case StrategyAPI:
		return api.New(
			cfg.LLM.API.APIKey,
			cfg.LLM.API.Model,
		), nil

	case StrategyCLI:
		return nil, fmt.Errorf("CLI strategy not yet implemented")

	default:
		return nil, fmt.Errorf("unknown intent parsing strategy: %s", cfg.LLM.Strategy)
	}
}
```

**Step 3: Run tests**

```bash
go test ./internal/intent/... -v
```

**Step 4: Commit**

```bash
git add internal/intent/
git commit -m "feat(intent): add parser factory"
```

---

## Task 5: Error Handling and Retries

**Files:**
- Modify: `internal/intent/api/parser.go`
- Create: `internal/intent/api/retry_test.go`

**Step 1: Write failing test for retry**

Create `internal/intent/api/retry_test.go`:
```go
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
```

**Step 2: Implement retry logic**

Update `internal/intent/api/parser.go` to add:
```go
// WithRetries sets the number of retry attempts.
func WithRetries(n int) Option {
	return func(p *Parser) {
		p.maxRetries = n
	}
}

// Add to Parser struct:
maxRetries int // default 1

// Update Parse method to retry on transient errors
```

**Step 3: Run tests**

```bash
go test ./internal/intent/api/... -v
```

**Step 4: Commit**

```bash
git add internal/intent/
git commit -m "feat(intent): add retry logic to API parser"
```

---

## Task 6: Integration with Event Router

**Files:**
- Modify: `internal/event/router.go`
- Modify: `internal/event/router_test.go`

**Step 1: Update router to use intent parser**

Add intent parsing to the router for comment/mention events. The router should:
1. Parse intent from comment body
2. Include parsed intent in the handler call
3. Pass requested actions for permission checking

**Step 2: Update handler signature**

```go
type Handler func(ctx context.Context, event *Event, cfg *config.MergedConfig, intent *intent.ParsedIntent) error
```

**Step 3: Run tests**

```bash
go test ./internal/event/... -v
```

**Step 4: Commit**

```bash
git add internal/event/
git commit -m "feat(event): integrate intent parser into router"
```

---

## Task 7: Update Config for LLM Settings

**Files:**
- Modify: `internal/config/config.go`

**Step 1: Add LLM config types**

Ensure config has proper types for LLM settings:
```go
type LLMConfig struct {
	Strategy string       `yaml:"strategy"`
	API      LLMAPIConfig `yaml:"api"`
}

type LLMAPIConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	APIKey   string `yaml:"api_key"`
}
```

**Step 2: Commit**

```bash
git add internal/config/
git commit -m "feat(config): add LLM configuration types"
```

---

## Task 8: Run Full Test Suite

**Step 1: Run all tests with coverage**

```bash
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

**Step 2: Verify coverage >= 80%**

**Step 3: Commit if needed**

---

## Summary

| Task | Component | Tests Added |
|------|-----------|-------------|
| 1 | Intent types | - |
| 2 | Parser interface | - |
| 3 | API parser | 3 |
| 4 | Parser factory | 3 |
| 5 | Retry logic | 2 |
| 6 | Router integration | 1 |
| 7 | Config types | - |
| 8 | Coverage verification | - |

**Total: 8 tasks, ~9 tests**
