package agent

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestManager_Queue(t *testing.T) {
	var spawnCount int32

	// Mock spawner behavior
	manager := NewManager(ManagerConfig{
		MaxConcurrent: 2,
		QueueSize:     5,
	})
	defer manager.Shutdown()

	// Spawn function that tracks calls
	spawnFn := func(ctx context.Context, req SpawnRequest) error {
		atomic.AddInt32(&spawnCount, 1)
		time.Sleep(50 * time.Millisecond)
		return nil
	}

	// Queue 4 requests (2 should run, 2 should queue)
	for i := 0; i < 4; i++ {
		manager.Enqueue(SpawnRequest{ID: fmt.Sprintf("agent-%d", i)}, spawnFn)
	}

	// Give time for processing
	time.Sleep(200 * time.Millisecond)

	if count := atomic.LoadInt32(&spawnCount); count != 4 {
		t.Errorf("spawnCount = %d, want 4", count)
	}
}

func TestManager_QueueFull(t *testing.T) {
	manager := NewManager(ManagerConfig{
		MaxConcurrent: 1,
		QueueSize:     1,
	})
	defer manager.Shutdown()

	// Block the single slot
	blocking := make(chan struct{})
	manager.Enqueue(SpawnRequest{ID: "blocking"}, func(ctx context.Context, req SpawnRequest) error {
		<-blocking
		return nil
	})

	// Fill the queue
	manager.Enqueue(SpawnRequest{ID: "queued"}, func(ctx context.Context, req SpawnRequest) error {
		return nil
	})

	// This should fail - queue full
	err := manager.Enqueue(SpawnRequest{ID: "overflow"}, func(ctx context.Context, req SpawnRequest) error {
		return nil
	})

	if err == nil {
		t.Error("Expected queue full error")
	}

	close(blocking)
}

func TestManager_QueueLengthAndActiveCount(t *testing.T) {
	manager := NewManager(ManagerConfig{
		MaxConcurrent: 1,
		QueueSize:     5,
	})
	defer manager.Shutdown()

	// Initially empty
	if qlen := manager.QueueLength(); qlen != 0 {
		t.Errorf("QueueLength() = %d, want 0", qlen)
	}
	if count := manager.ActiveCount(); count != 0 {
		t.Errorf("ActiveCount() = %d, want 0", count)
	}

	// Block the single worker
	blocking := make(chan struct{})
	started := make(chan struct{})
	manager.Enqueue(SpawnRequest{ID: "active"}, func(ctx context.Context, req SpawnRequest) error {
		close(started)
		<-blocking
		return nil
	})

	// Wait for it to start
	<-started

	// Check active count - should be 1
	time.Sleep(10 * time.Millisecond) // Give time for semaphore to be acquired
	if count := manager.ActiveCount(); count != 1 {
		t.Errorf("ActiveCount() = %d, want 1", count)
	}

	// Queue some more requests
	for i := 0; i < 3; i++ {
		manager.Enqueue(SpawnRequest{ID: fmt.Sprintf("queued-%d", i)}, func(ctx context.Context, req SpawnRequest) error {
			return nil
		})
	}

	// Check queue length (some should be in queue while one is active)
	// Note: since concurrent=1, items go to queue
	time.Sleep(10 * time.Millisecond)
	qlen := manager.QueueLength()
	if qlen < 1 {
		t.Errorf("QueueLength() = %d, want >= 1", qlen)
	}

	// Unblock
	close(blocking)
}

func TestManager_DefaultConfig(t *testing.T) {
	// Test with zero values - should use defaults
	manager := NewManager(ManagerConfig{})
	defer manager.Shutdown()

	// Just verify it doesn't panic and works
	err := manager.Enqueue(SpawnRequest{ID: "test"}, func(ctx context.Context, req SpawnRequest) error {
		return nil
	})
	if err != nil {
		t.Errorf("Enqueue() error = %v", err)
	}

	// Wait for processing
	time.Sleep(50 * time.Millisecond)
}
