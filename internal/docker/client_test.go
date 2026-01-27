package docker

import (
	"context"
	"testing"
)

func TestClient_Ping(t *testing.T) {
	// Skip if Docker not available
	client, err := NewClient()
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}
	defer client.Close()

	if err := client.Ping(context.Background()); err != nil {
		t.Errorf("Ping() error = %v", err)
	}
}

func TestClient_ImageExists(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}
	defer client.Close()

	// Alpine should exist on most systems or be quick to pull
	exists, err := client.ImageExists(context.Background(), "alpine:latest")
	if err != nil {
		t.Logf("ImageExists() error = %v (may need to pull image)", err)
	}
	_ = exists // Just checking it doesn't panic
}
