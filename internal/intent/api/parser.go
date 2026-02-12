package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/drewdunne/familiar/internal/intent"
)

const defaultBaseURL = "https://api.anthropic.com/v1"

// Ensure Parser implements intent.Parser.
var _ intent.Parser = (*Parser)(nil)

func init() {
	intent.Register(intent.StrategyAPI, func(apiKey, model string) intent.Parser {
		return New(apiKey, model)
	})
}

// Parser implements intent.Parser using the Anthropic API.
type Parser struct {
	apiKey     string
	model      string
	baseURL    string
	client     *http.Client
	maxRetries int
}

// Option configures the API parser.
type Option func(*Parser)

// WithBaseURL sets a custom base URL (for testing).
func WithBaseURL(url string) Option {
	return func(p *Parser) {
		p.baseURL = url
	}
}

// WithRetries sets the number of retry attempts.
func WithRetries(n int) Option {
	return func(p *Parser) {
		p.maxRetries = n
	}
}

// New creates a new API-based intent parser.
func New(apiKey, model string, opts ...Option) *Parser {
	p := &Parser{
		apiKey:     apiKey,
		model:      model,
		baseURL:    defaultBaseURL,
		client:     &http.Client{Timeout: 30 * time.Second},
		maxRetries: 1, // default: no retries (1 attempt)
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

	var lastErr error
	for attempt := 1; attempt <= p.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(reqJSON))
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", p.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")

		resp, err := p.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("making request: %w", err)
			continue
		}

		if resp.StatusCode >= 500 {
			// Transient server error - retry
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("API error (status %d): %s", resp.StatusCode, body)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			// Client error (4xx) - don't retry
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, body)
		}

		var apiResp anthropicResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decoding response: %w", err)
		}
		resp.Body.Close()

		return parseResponse(apiResp, text)
	}

	return nil, lastErr
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
- requested_actions: Array of privileged actions explicitly requested. Valid values: "merge", "approve", "dismiss_reviews", "push"
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
