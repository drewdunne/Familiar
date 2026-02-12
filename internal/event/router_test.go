package event

import (
	"context"
	"testing"

	"github.com/drewdunne/familiar/internal/config"
	"github.com/drewdunne/familiar/internal/intent"
)

func TestRouter_Route(t *testing.T) {
	var handledEvent *Event
	handler := func(ctx context.Context, e *Event, cfg *config.MergedConfig, parsedIntent *intent.ParsedIntent) error {
		handledEvent = e
		return nil
	}

	serverCfg := &config.Config{
		Events: config.ServerEventsConfig{
			MROpened: true,
		},
		Agents: config.AgentsConfig{
			DebounceSeconds: 1,
		},
	}

	router := NewRouter(serverCfg, handler, nil)

	event := &Event{
		Type:      TypeMROpened,
		Provider:  "github",
		RepoOwner: "owner",
		RepoName:  "repo",
		MRNumber:  42,
	}

	err := router.Route(context.Background(), event)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}

	if handledEvent == nil {
		t.Error("Handler was not called")
	}
}

func TestRouter_EventDisabled(t *testing.T) {
	handlerCalled := false
	handler := func(ctx context.Context, e *Event, cfg *config.MergedConfig, parsedIntent *intent.ParsedIntent) error {
		handlerCalled = true
		return nil
	}

	serverCfg := &config.Config{
		Events: config.ServerEventsConfig{
			MROpened: false, // Disabled
		},
		Agents: config.AgentsConfig{
			DebounceSeconds: 1,
		},
	}

	router := NewRouter(serverCfg, handler, nil)

	event := &Event{
		Type:      TypeMROpened,
		Provider:  "github",
		RepoOwner: "owner",
		RepoName:  "repo",
		MRNumber:  42,
	}

	// Should not error, but also not call handler
	router.Route(context.Background(), event)

	if handlerCalled {
		t.Error("Handler should not be called for disabled event")
	}
}

func TestRouter_Debounce(t *testing.T) {
	callCount := 0
	handler := func(ctx context.Context, e *Event, cfg *config.MergedConfig, parsedIntent *intent.ParsedIntent) error {
		callCount++
		return nil
	}

	serverCfg := &config.Config{
		Events: config.ServerEventsConfig{
			MRUpdated: true,
		},
		Agents: config.AgentsConfig{
			DebounceSeconds: 1,
		},
	}

	router := NewRouter(serverCfg, handler, nil)

	event := &Event{
		Type:      TypeMRUpdated,
		Provider:  "github",
		RepoOwner: "owner",
		RepoName:  "repo",
		MRNumber:  42,
	}

	// First call should go through
	router.Route(context.Background(), event)
	// Second call immediately should be debounced
	router.Route(context.Background(), event)

	if callCount != 1 {
		t.Errorf("Handler called %d times, want 1 (second should be debounced)", callCount)
	}
}

func TestRouter_AllEventTypes(t *testing.T) {
	callCount := 0
	handler := func(ctx context.Context, e *Event, cfg *config.MergedConfig, parsedIntent *intent.ParsedIntent) error {
		callCount++
		return nil
	}

	serverCfg := &config.Config{
		Events: config.ServerEventsConfig{
			MROpened:  true,
			MRComment: true,
			MRUpdated: true,
			Mention:   true,
		},
		Agents: config.AgentsConfig{
			DebounceSeconds: 0, // Use default
		},
	}

	router := NewRouter(serverCfg, handler, nil)

	events := []*Event{
		{Type: TypeMROpened, Provider: "github", RepoOwner: "o", RepoName: "r", MRNumber: 1},
		{Type: TypeMRComment, Provider: "github", RepoOwner: "o", RepoName: "r", MRNumber: 2},
		{Type: TypeMRUpdated, Provider: "github", RepoOwner: "o", RepoName: "r", MRNumber: 3},
		{Type: TypeMention, Provider: "github", RepoOwner: "o", RepoName: "r", MRNumber: 4},
	}

	for _, e := range events {
		router.Route(context.Background(), e)
	}

	if callCount != 4 {
		t.Errorf("Handler called %d times, want 4", callCount)
	}
}

// mockParser is a test parser that returns predefined results.
type mockParser struct {
	result *intent.ParsedIntent
	err    error
}

func (m *mockParser) Parse(ctx context.Context, text string) (*intent.ParsedIntent, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestRouter_IntentParsing_MRComment(t *testing.T) {
	var receivedIntent *intent.ParsedIntent
	handler := func(ctx context.Context, e *Event, cfg *config.MergedConfig, parsedIntent *intent.ParsedIntent) error {
		receivedIntent = parsedIntent
		return nil
	}

	expectedIntent := &intent.ParsedIntent{
		Instructions:     "Please review this code",
		RequestedActions: []intent.Action{intent.ActionApprove},
		Confidence:       0.9,
		Raw:              "@familiar please review and approve",
	}

	parser := &mockParser{result: expectedIntent}

	serverCfg := &config.Config{
		Events: config.ServerEventsConfig{
			MRComment: true,
		},
		Agents: config.AgentsConfig{
			DebounceSeconds: 1,
		},
	}

	router := NewRouter(serverCfg, handler, parser)

	event := &Event{
		Type:        TypeMRComment,
		Provider:    "github",
		RepoOwner:   "owner",
		RepoName:    "repo",
		MRNumber:    42,
		CommentBody: "@familiar please review and approve",
	}

	err := router.Route(context.Background(), event)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}

	if receivedIntent == nil {
		t.Fatal("Handler should receive parsed intent for MRComment events")
	}

	if receivedIntent.Instructions != expectedIntent.Instructions {
		t.Errorf("Intent.Instructions = %q, want %q", receivedIntent.Instructions, expectedIntent.Instructions)
	}

	if len(receivedIntent.RequestedActions) != 1 || receivedIntent.RequestedActions[0] != intent.ActionApprove {
		t.Errorf("Intent.RequestedActions = %v, want [%s]", receivedIntent.RequestedActions, intent.ActionApprove)
	}
}

func TestRouter_IntentParsing_Mention(t *testing.T) {
	var receivedIntent *intent.ParsedIntent
	handler := func(ctx context.Context, e *Event, cfg *config.MergedConfig, parsedIntent *intent.ParsedIntent) error {
		receivedIntent = parsedIntent
		return nil
	}

	expectedIntent := &intent.ParsedIntent{
		Instructions: "Help me with this bug",
		Confidence:   0.85,
		Raw:          "@familiar help me with this bug",
	}

	parser := &mockParser{result: expectedIntent}

	serverCfg := &config.Config{
		Events: config.ServerEventsConfig{
			Mention: true,
		},
		Agents: config.AgentsConfig{
			DebounceSeconds: 1,
		},
	}

	router := NewRouter(serverCfg, handler, parser)

	event := &Event{
		Type:        TypeMention,
		Provider:    "github",
		RepoOwner:   "owner",
		RepoName:    "repo",
		MRNumber:    42,
		CommentBody: "@familiar help me with this bug",
	}

	err := router.Route(context.Background(), event)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}

	if receivedIntent == nil {
		t.Fatal("Handler should receive parsed intent for Mention events")
	}

	if receivedIntent.Instructions != expectedIntent.Instructions {
		t.Errorf("Intent.Instructions = %q, want %q", receivedIntent.Instructions, expectedIntent.Instructions)
	}
}

func TestRouter_IntentParsing_MROpened_NoIntent(t *testing.T) {
	var receivedIntent *intent.ParsedIntent
	handler := func(ctx context.Context, e *Event, cfg *config.MergedConfig, parsedIntent *intent.ParsedIntent) error {
		receivedIntent = parsedIntent
		return nil
	}

	// Parser should not be called for MROpened events
	parser := &mockParser{result: &intent.ParsedIntent{Instructions: "should not be used"}}

	serverCfg := &config.Config{
		Events: config.ServerEventsConfig{
			MROpened: true,
		},
		Agents: config.AgentsConfig{
			DebounceSeconds: 1,
		},
	}

	router := NewRouter(serverCfg, handler, parser)

	event := &Event{
		Type:      TypeMROpened,
		Provider:  "github",
		RepoOwner: "owner",
		RepoName:  "repo",
		MRNumber:  42,
	}

	err := router.Route(context.Background(), event)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}

	if receivedIntent != nil {
		t.Error("Handler should receive nil intent for MROpened events")
	}
}

func TestRouter_IntentParsing_NilParser(t *testing.T) {
	var receivedIntent *intent.ParsedIntent
	handler := func(ctx context.Context, e *Event, cfg *config.MergedConfig, parsedIntent *intent.ParsedIntent) error {
		receivedIntent = parsedIntent
		return nil
	}

	serverCfg := &config.Config{
		Events: config.ServerEventsConfig{
			MRComment: true,
		},
		Agents: config.AgentsConfig{
			DebounceSeconds: 1,
		},
	}

	// Router with nil parser
	router := NewRouter(serverCfg, handler, nil)

	event := &Event{
		Type:        TypeMRComment,
		Provider:    "github",
		RepoOwner:   "owner",
		RepoName:    "repo",
		MRNumber:    42,
		CommentBody: "@familiar please review",
	}

	err := router.Route(context.Background(), event)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}

	// With nil parser, intent should be nil
	if receivedIntent != nil {
		t.Error("Handler should receive nil intent when parser is nil")
	}
}

func TestRouter_IntentParsing_ParserError(t *testing.T) {
	var handlerCalled bool
	var receivedIntent *intent.ParsedIntent
	handler := func(ctx context.Context, e *Event, cfg *config.MergedConfig, parsedIntent *intent.ParsedIntent) error {
		handlerCalled = true
		receivedIntent = parsedIntent
		return nil
	}

	// Parser that returns an error
	parser := &mockParser{err: context.DeadlineExceeded}

	serverCfg := &config.Config{
		Events: config.ServerEventsConfig{
			MRComment: true,
		},
		Agents: config.AgentsConfig{
			DebounceSeconds: 1,
		},
	}

	router := NewRouter(serverCfg, handler, parser)

	event := &Event{
		Type:        TypeMRComment,
		Provider:    "github",
		RepoOwner:   "owner",
		RepoName:    "repo",
		MRNumber:    42,
		CommentBody: "@familiar please review",
	}

	err := router.Route(context.Background(), event)
	if err != nil {
		t.Fatalf("Route() should not fail when parser fails, got error = %v", err)
	}

	// Handler should still be called even if parsing fails
	if !handlerCalled {
		t.Error("Handler should be called even when parser fails")
	}

	// Intent should be nil due to parser error
	if receivedIntent != nil {
		t.Error("Handler should receive nil intent when parser fails")
	}
}

func TestIsBotActor(t *testing.T) {
	tests := []struct {
		name        string
		actor       string
		botUsername string
		want        bool
	}{
		// GitLab project access token bots
		{"gitlab project bot", "project_1_bot_d0fe5ec2e069d8e934540c30708818d6", "Familiar", true},
		{"gitlab project bot short hash", "project_123_bot_abc123", "Familiar", true},
		{"gitlab project bot minimal", "project_0_bot_x", "Familiar", true},

		// GitHub app bots
		{"github dependabot", "dependabot[bot]", "Familiar", true},
		{"github actions bot", "github-actions[bot]", "Familiar", true},
		{"github renovate bot", "renovate[bot]", "Familiar", true},

		// Configured bot username - exact match
		{"bot username exact match", "Familiar", "Familiar", true},

		// Configured bot username - case insensitive
		{"bot username lowercase", "familiar", "Familiar", true},
		{"bot username uppercase", "FAMILIAR", "Familiar", true},
		{"bot username mixed case", "fAmIlIaR", "Familiar", true},

		// Custom bot username
		{"custom bot username", "my-bot", "my-bot", true},
		{"custom bot username case insensitive", "My-Bot", "my-bot", true},

		// Empty bot username should not match everything
		{"empty bot username no false positive", "drewdunne", "", false},

		// Real users
		{"real user drewdunne", "drewdunne", "Familiar", false},
		{"real user alice", "alice", "Familiar", false},
		{"real user project_manager", "project_manager", "Familiar", false},
		{"real user bot_user", "bot_user", "Familiar", false},
		{"empty actor", "", "Familiar", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBotActor(tt.actor, tt.botUsername); got != tt.want {
				t.Errorf("isBotActor(%q, %q) = %v, want %v", tt.actor, tt.botUsername, got, tt.want)
			}
		})
	}
}

func TestRouter_SkipsBotActor(t *testing.T) {
	handlerCalled := false
	handler := func(ctx context.Context, e *Event, cfg *config.MergedConfig, parsedIntent *intent.ParsedIntent) error {
		handlerCalled = true
		return nil
	}

	serverCfg := &config.Config{
		Events: config.ServerEventsConfig{
			MRComment: true,
		},
		Agents: config.AgentsConfig{
			DebounceSeconds: 1,
		},
	}

	router := NewRouter(serverCfg, handler, nil)

	event := &Event{
		Type:      TypeMRComment,
		Provider:  "gitlab",
		RepoOwner: "owner",
		RepoName:  "repo",
		MRNumber:  42,
		Actor:     "project_1_bot_d0fe5ec2e069d8e934540c30708818d6",
	}

	err := router.Route(context.Background(), event)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}

	if handlerCalled {
		t.Error("Handler should not be called for bot actor events")
	}
}

func TestRouter_SkipsConfiguredBotUsername(t *testing.T) {
	handlerCalled := false
	handler := func(ctx context.Context, e *Event, cfg *config.MergedConfig, parsedIntent *intent.ParsedIntent) error {
		handlerCalled = true
		return nil
	}

	serverCfg := &config.Config{
		BotUsername: "Familiar",
		Events: config.ServerEventsConfig{
			MRComment: true,
		},
		Agents: config.AgentsConfig{
			DebounceSeconds: 1,
		},
	}

	router := NewRouter(serverCfg, handler, nil)

	event := &Event{
		Type:      TypeMRComment,
		Provider:  "gitlab",
		RepoOwner: "owner",
		RepoName:  "repo",
		MRNumber:  42,
		Actor:     "familiar", // lowercase — should still match
	}

	err := router.Route(context.Background(), event)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}

	if handlerCalled {
		t.Error("Handler should not be called when actor matches configured bot username")
	}
}

func TestRouter_UnknownEventType(t *testing.T) {
	handlerCalled := false
	handler := func(ctx context.Context, e *Event, cfg *config.MergedConfig, parsedIntent *intent.ParsedIntent) error {
		handlerCalled = true
		return nil
	}

	serverCfg := &config.Config{
		Events: config.ServerEventsConfig{
			MROpened:  true,
			MRComment: true,
			MRUpdated: true,
			Mention:   true,
		},
		Agents: config.AgentsConfig{
			DebounceSeconds: 1,
		},
	}

	router := NewRouter(serverCfg, handler, nil)

	// Event with unknown type
	event := &Event{
		Type:      Type("unknown_event"),
		Provider:  "github",
		RepoOwner: "owner",
		RepoName:  "repo",
		MRNumber:  42,
	}

	// Should not error but should not call handler either
	err := router.Route(context.Background(), event)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}

	if handlerCalled {
		t.Error("Handler should not be called for unknown event type")
	}
}
