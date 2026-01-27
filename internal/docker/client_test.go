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

	ctx := context.Background()

	// Test with a non-existent image - should return false
	exists, err := client.ImageExists(ctx, "nonexistent-image-that-does-not-exist:v999")
	if err != nil {
		t.Fatalf("ImageExists() unexpected error for non-existent image: %v", err)
	}
	if exists {
		t.Error("ImageExists() returned true for non-existent image, expected false")
	}

	// Pull alpine (small image) to test positive case
	if err := client.PullImage(ctx, "alpine:latest"); err != nil {
		t.Skipf("Could not pull alpine image for test: %v", err)
	}

	// Now test that the pulled image exists
	exists, err = client.ImageExists(ctx, "alpine:latest")
	if err != nil {
		t.Fatalf("ImageExists() unexpected error for pulled image: %v", err)
	}
	if !exists {
		t.Error("ImageExists() returned false for pulled image, expected true")
	}
}
