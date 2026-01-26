package intent_test

import (
	"testing"

	"github.com/drewdunne/familiar/internal/config"
	"github.com/drewdunne/familiar/internal/intent"
	_ "github.com/drewdunne/familiar/internal/intent/api" // Register API parser
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

	parser, err := intent.NewParser(cfg)
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

	_, err := intent.NewParser(cfg)
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

	_, err := intent.NewParser(cfg)
	if err == nil {
		t.Error("Unknown strategy should return error")
	}
}
