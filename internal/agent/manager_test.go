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
