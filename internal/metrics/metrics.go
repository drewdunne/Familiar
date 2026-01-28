package metrics

import (
	"sync/atomic"
)

// Metrics tracks operational metrics.
type Metrics struct {
	AgentsSpawned     uint64 `json:"agents_spawned"`
	AgentsCompleted   uint64 `json:"agents_completed"`
	AgentsFailed      uint64 `json:"agents_failed"`
	AgentsTimedOut    uint64 `json:"agents_timed_out"`
	WebhooksReceived  uint64 `json:"webhooks_received"`
	WebhooksProcessed uint64 `json:"webhooks_processed"`
}

var global = &Metrics{}

// AgentSpawned increments the count of agents spawned.
func AgentSpawned() { atomic.AddUint64(&global.AgentsSpawned, 1) }

// AgentCompleted increments the count of agents that completed successfully.
func AgentCompleted() { atomic.AddUint64(&global.AgentsCompleted, 1) }

// AgentFailed increments the count of agents that failed.
func AgentFailed() { atomic.AddUint64(&global.AgentsFailed, 1) }

// AgentTimedOut increments the count of agents that timed out.
func AgentTimedOut() { atomic.AddUint64(&global.AgentsTimedOut, 1) }

// WebhookReceived increments the count of webhooks received.
func WebhookReceived() { atomic.AddUint64(&global.WebhooksReceived, 1) }

// WebhookProcessed increments the count of webhooks processed.
func WebhookProcessed() { atomic.AddUint64(&global.WebhooksProcessed, 1) }

// Get returns a snapshot of the current metrics.
func Get() Metrics {
	return Metrics{
		AgentsSpawned:     atomic.LoadUint64(&global.AgentsSpawned),
		AgentsCompleted:   atomic.LoadUint64(&global.AgentsCompleted),
		AgentsFailed:      atomic.LoadUint64(&global.AgentsFailed),
		AgentsTimedOut:    atomic.LoadUint64(&global.AgentsTimedOut),
		WebhooksReceived:  atomic.LoadUint64(&global.WebhooksReceived),
		WebhooksProcessed: atomic.LoadUint64(&global.WebhooksProcessed),
	}
}

// Reset resets all metrics to zero (useful for testing).
func Reset() {
	atomic.StoreUint64(&global.AgentsSpawned, 0)
	atomic.StoreUint64(&global.AgentsCompleted, 0)
	atomic.StoreUint64(&global.AgentsFailed, 0)
	atomic.StoreUint64(&global.AgentsTimedOut, 0)
	atomic.StoreUint64(&global.WebhooksReceived, 0)
	atomic.StoreUint64(&global.WebhooksProcessed, 0)
}
