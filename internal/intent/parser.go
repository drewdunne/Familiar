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
