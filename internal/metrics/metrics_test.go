package metrics

import (
	"sync"
	"testing"
)

func TestAgentSpawned(t *testing.T) {
	Reset()

	AgentSpawned()
	m := Get()

	if m.AgentsSpawned != 1 {
		t.Errorf("expected AgentsSpawned=1, got %d", m.AgentsSpawned)
	}
}

func TestAgentCompleted(t *testing.T) {
	Reset()

	AgentCompleted()
	m := Get()

	if m.AgentsCompleted != 1 {
		t.Errorf("expected AgentsCompleted=1, got %d", m.AgentsCompleted)
	}
}

func TestAgentFailed(t *testing.T) {
	Reset()

	AgentFailed()
	m := Get()

	if m.AgentsFailed != 1 {
		t.Errorf("expected AgentsFailed=1, got %d", m.AgentsFailed)
	}
}

func TestAgentTimedOut(t *testing.T) {
	Reset()

	AgentTimedOut()
	m := Get()

	if m.AgentsTimedOut != 1 {
		t.Errorf("expected AgentsTimedOut=1, got %d", m.AgentsTimedOut)
	}
}

func TestWebhookReceived(t *testing.T) {
	Reset()

	WebhookReceived()
	m := Get()

	if m.WebhooksReceived != 1 {
		t.Errorf("expected WebhooksReceived=1, got %d", m.WebhooksReceived)
	}
}

func TestWebhookProcessed(t *testing.T) {
	Reset()

	WebhookProcessed()
	m := Get()

	if m.WebhooksProcessed != 1 {
		t.Errorf("expected WebhooksProcessed=1, got %d", m.WebhooksProcessed)
	}
}

func TestReset(t *testing.T) {
	// Set all counters
	AgentSpawned()
	AgentCompleted()
	AgentFailed()
	AgentTimedOut()
	WebhookReceived()
	WebhookProcessed()

	// Reset
	Reset()
	m := Get()

	if m.AgentsSpawned != 0 {
		t.Errorf("expected AgentsSpawned=0 after reset, got %d", m.AgentsSpawned)
	}
	if m.AgentsCompleted != 0 {
		t.Errorf("expected AgentsCompleted=0 after reset, got %d", m.AgentsCompleted)
	}
	if m.AgentsFailed != 0 {
		t.Errorf("expected AgentsFailed=0 after reset, got %d", m.AgentsFailed)
	}
	if m.AgentsTimedOut != 0 {
		t.Errorf("expected AgentsTimedOut=0 after reset, got %d", m.AgentsTimedOut)
	}
	if m.WebhooksReceived != 0 {
		t.Errorf("expected WebhooksReceived=0 after reset, got %d", m.WebhooksReceived)
	}
	if m.WebhooksProcessed != 0 {
		t.Errorf("expected WebhooksProcessed=0 after reset, got %d", m.WebhooksProcessed)
	}
}

func TestMultipleIncrements(t *testing.T) {
	Reset()

	for i := 0; i < 5; i++ {
		AgentSpawned()
	}
	for i := 0; i < 3; i++ {
		AgentCompleted()
	}
	for i := 0; i < 2; i++ {
		AgentFailed()
	}

	m := Get()

	if m.AgentsSpawned != 5 {
		t.Errorf("expected AgentsSpawned=5, got %d", m.AgentsSpawned)
	}
	if m.AgentsCompleted != 3 {
		t.Errorf("expected AgentsCompleted=3, got %d", m.AgentsCompleted)
	}
	if m.AgentsFailed != 2 {
		t.Errorf("expected AgentsFailed=2, got %d", m.AgentsFailed)
	}
}

func TestConcurrentIncrements(t *testing.T) {
	Reset()

	var wg sync.WaitGroup
	iterations := 1000

	// Spawn multiple goroutines incrementing counters concurrently
	for i := 0; i < iterations; i++ {
		wg.Add(6)
		go func() {
			AgentSpawned()
			wg.Done()
		}()
		go func() {
			AgentCompleted()
			wg.Done()
		}()
		go func() {
			AgentFailed()
			wg.Done()
		}()
		go func() {
			AgentTimedOut()
			wg.Done()
		}()
		go func() {
			WebhookReceived()
			wg.Done()
		}()
		go func() {
			WebhookProcessed()
			wg.Done()
		}()
	}

	wg.Wait()
	m := Get()

	if m.AgentsSpawned != uint64(iterations) {
		t.Errorf("expected AgentsSpawned=%d, got %d", iterations, m.AgentsSpawned)
	}
	if m.AgentsCompleted != uint64(iterations) {
		t.Errorf("expected AgentsCompleted=%d, got %d", iterations, m.AgentsCompleted)
	}
	if m.AgentsFailed != uint64(iterations) {
		t.Errorf("expected AgentsFailed=%d, got %d", iterations, m.AgentsFailed)
	}
	if m.AgentsTimedOut != uint64(iterations) {
		t.Errorf("expected AgentsTimedOut=%d, got %d", iterations, m.AgentsTimedOut)
	}
	if m.WebhooksReceived != uint64(iterations) {
		t.Errorf("expected WebhooksReceived=%d, got %d", iterations, m.WebhooksReceived)
	}
	if m.WebhooksProcessed != uint64(iterations) {
		t.Errorf("expected WebhooksProcessed=%d, got %d", iterations, m.WebhooksProcessed)
	}
}

func TestGetReturnsSnapshot(t *testing.T) {
	Reset()

	AgentSpawned()
	snapshot := Get()

	// Increment again after snapshot
	AgentSpawned()

	// Snapshot should not change
	if snapshot.AgentsSpawned != 1 {
		t.Errorf("snapshot should be immutable, expected 1, got %d", snapshot.AgentsSpawned)
	}

	// New Get should reflect the change
	current := Get()
	if current.AgentsSpawned != 2 {
		t.Errorf("current should be 2, got %d", current.AgentsSpawned)
	}
}
