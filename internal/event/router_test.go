package event

import (
	"context"
	"testing"

	"github.com/drewdunne/familiar/internal/config"
)

func TestRouter_Route(t *testing.T) {
	var handledEvent *Event
	handler := func(ctx context.Context, e *Event, cfg *config.MergedConfig) error {
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

	router := NewRouter(serverCfg, handler)

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
	handler := func(ctx context.Context, e *Event, cfg *config.MergedConfig) error {
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

	router := NewRouter(serverCfg, handler)

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
	handler := func(ctx context.Context, e *Event, cfg *config.MergedConfig) error {
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

	router := NewRouter(serverCfg, handler)

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
	handler := func(ctx context.Context, e *Event, cfg *config.MergedConfig) error {
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

	router := NewRouter(serverCfg, handler)

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
