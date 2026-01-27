package agent

import (
	"context"
	"errors"
	"sync"
)

// ErrQueueFull is returned when the queue is at capacity.
var ErrQueueFull = errors.New("agent queue is full")

// ManagerConfig configures the session manager.
type ManagerConfig struct {
	MaxConcurrent int
	QueueSize     int
}

// SpawnFunc is a function that spawns an agent.
type SpawnFunc func(ctx context.Context, req SpawnRequest) error

// queuedRequest represents a queued spawn request.
type queuedRequest struct {
	req     SpawnRequest
	spawnFn SpawnFunc
}

// Manager manages agent concurrency and queueing.
type Manager struct {
	cfg       ManagerConfig
	queue     chan queuedRequest
	semaphore chan struct{}
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewManager creates a new session manager.
func NewManager(cfg ManagerConfig) *Manager {
	if cfg.MaxConcurrent == 0 {
		cfg.MaxConcurrent = 5
	}
	if cfg.QueueSize == 0 {
		cfg.QueueSize = 20
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		cfg:       cfg,
		queue:     make(chan queuedRequest, cfg.QueueSize),
		semaphore: make(chan struct{}, cfg.MaxConcurrent),
		ctx:       ctx,
		cancel:    cancel,
	}

	// Start worker
	go m.worker()

	return m
}

// Enqueue adds a spawn request to the queue.
func (m *Manager) Enqueue(req SpawnRequest, spawnFn SpawnFunc) error {
	select {
	case m.queue <- queuedRequest{req: req, spawnFn: spawnFn}:
		return nil
	default:
		return ErrQueueFull
	}
}

// worker processes the queue.
func (m *Manager) worker() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case queued := <-m.queue:
			// Wait for semaphore
			m.semaphore <- struct{}{}

			m.wg.Add(1)
			go func(q queuedRequest) {
				defer m.wg.Done()
				defer func() { <-m.semaphore }()

				q.spawnFn(m.ctx, q.req)
			}(queued)
		}
	}
}

// QueueLength returns current queue length.
func (m *Manager) QueueLength() int {
	return len(m.queue)
}

// ActiveCount returns number of currently running agents.
func (m *Manager) ActiveCount() int {
	return len(m.semaphore)
}

// Shutdown stops the manager and waits for agents to complete.
func (m *Manager) Shutdown() {
	m.cancel()
	m.wg.Wait()
}
