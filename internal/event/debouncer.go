package event

import (
	"sync"
	"time"
)

// Debouncer prevents duplicate events within a time window.
type Debouncer struct {
	window time.Duration
	seen   map[string]time.Time
	mu     sync.Mutex
}

// NewDebouncer creates a new debouncer with the given window.
func NewDebouncer(window time.Duration) *Debouncer {
	return &Debouncer{
		window: window,
		seen:   make(map[string]time.Time),
	}
}

// ShouldProcess returns true if the event should be processed.
// Returns false if a similar event was processed recently.
func (d *Debouncer) ShouldProcess(e *Event) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := e.Key()
	now := time.Now()

	if lastSeen, ok := d.seen[key]; ok {
		if now.Sub(lastSeen) < d.window {
			return false
		}
	}

	d.seen[key] = now
	return true
}

// Cleanup removes old entries from the seen map.
func (d *Debouncer) Cleanup() {
	d.mu.Lock()
	defer d.mu.Unlock()

	threshold := time.Now().Add(-d.window * 2)
	for key, t := range d.seen {
		if t.Before(threshold) {
			delete(d.seen, key)
		}
	}
}
