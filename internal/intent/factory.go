package intent

import (
	"fmt"

	"github.com/drewdunne/familiar/internal/config"
)

// ParserFactory is a function that creates a Parser.
type ParserFactory func(apiKey, model string) Parser

// registry holds registered parser factories by strategy.
var registry = make(map[Strategy]ParserFactory)

// Register registers a parser factory for a strategy.
func Register(strategy Strategy, factory ParserFactory) {
	registry[strategy] = factory
}

// NewParser creates a parser based on the configured strategy.
func NewParser(cfg *config.Config) (Parser, error) {
	switch Strategy(cfg.LLM.Strategy) {
	case StrategyAPI:
		factory, ok := registry[StrategyAPI]
		if !ok {
			return nil, fmt.Errorf("API strategy not registered (import _ \"github.com/drewdunne/familiar/internal/intent/api\")")
		}
		return factory(
			cfg.LLM.API.APIKey,
			cfg.LLM.API.Model,
		), nil

	case StrategyCLI:
		return nil, fmt.Errorf("CLI strategy not yet implemented")

	default:
		return nil, fmt.Errorf("unknown intent parsing strategy: %s", cfg.LLM.Strategy)
	}
}
