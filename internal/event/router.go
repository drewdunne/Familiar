package event

import (
	"context"
	"log"
	"time"

	"github.com/drewdunne/familiar/internal/config"
)

// Handler processes a normalized event with merged config.
type Handler func(ctx context.Context, event *Event, cfg *config.MergedConfig) error

// Router routes events to handlers after config merging and validation.
type Router struct {
	serverCfg *config.Config
	handler   Handler
	debouncer *Debouncer
}

// NewRouter creates a new event router.
func NewRouter(serverCfg *config.Config, handler Handler) *Router {
	debounceWindow := time.Duration(serverCfg.Agents.DebounceSeconds) * time.Second
	if debounceWindow == 0 {
		debounceWindow = 10 * time.Second // Default
	}
	return &Router{
		serverCfg: serverCfg,
		handler:   handler,
		debouncer: NewDebouncer(debounceWindow),
	}
}

// Route processes an event through the routing pipeline.
func (r *Router) Route(ctx context.Context, event *Event) error {
	// Check if event type is enabled at server level first
	if !r.isEventEnabled(event.Type) {
		log.Printf("Event type disabled: %s", event.Type)
		return nil
	}

	// Check debounce
	if !r.debouncer.ShouldProcess(event) {
		log.Printf("Event debounced: %s", event.Key())
		return nil
	}

	// TODO: Fetch repo config and merge
	// For now, use server config only
	merged := config.MergeConfigs(r.serverCfg, &config.RepoConfig{})

	// Call handler
	return r.handler(ctx, event, merged)
}

func (r *Router) isEventEnabled(t Type) bool {
	switch t {
	case TypeMROpened:
		return r.serverCfg.Events.MROpened
	case TypeMRComment:
		return r.serverCfg.Events.MRComment
	case TypeMRUpdated:
		return r.serverCfg.Events.MRUpdated
	case TypeMention:
		return r.serverCfg.Events.Mention
	default:
		return false
	}
}
