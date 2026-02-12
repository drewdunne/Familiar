package intent

// Action represents a requested privileged action.
type Action string

const (
	ActionMerge          Action = "merge"
	ActionApprove        Action = "approve"
	ActionDismissReviews Action = "dismiss_reviews"
	ActionPush           Action = "push"
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
