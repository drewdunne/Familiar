package server

import (
	"context"
	"net/http"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/drewdunne/familiar/internal/config"
)

func TestServer_Shutdown(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 0, // Use any available port
		},
	}

	srv := New(cfg)

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServeWithShutdown()
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Programmatic shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := srv.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() error = %v, want nil", err)
	}

	// Wait for server goroutine to complete
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("ListenAndServeWithShutdown() error = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Server did not shut down in time")
	}
}

func TestServer_ShutdownOnSignal(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 0, // Use any available port
		},
	}

	srv := New(cfg)

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServeWithShutdown()
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Send SIGINT to trigger shutdown
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)

	// Wait for server to shut down
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("ListenAndServeWithShutdown() error = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Server did not respond to signal in time")
	}
}

func TestServer_ShutdownWithActiveRequests(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 0, // Use any available port
		},
	}

	srv := New(cfg)

	// Add a slow handler for testing
	requestStarted := make(chan struct{})
	requestDone := make(chan struct{})
	srv.mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-requestDone // Wait until test signals completion
		w.Write([]byte("done"))
	})

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServeWithShutdown()
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Get the actual server address
	addr := srv.Addr()
	if addr == "" {
		t.Fatal("Server address not available")
	}

	// Start a slow request
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := http.Get("http://" + addr + "/slow")
		if err != nil {
			// Connection may be reset during shutdown - that's acceptable
			return
		}
		resp.Body.Close()
	}()

	// Wait for request to start
	<-requestStarted

	// Initiate shutdown while request is in progress
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start shutdown in background
	go func() {
		srv.Shutdown(ctx)
	}()

	// Give shutdown a moment to start
	time.Sleep(50 * time.Millisecond)

	// Complete the slow request
	close(requestDone)

	// Wait for request goroutine
	wg.Wait()

	// Wait for server to complete shutdown
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("ListenAndServeWithShutdown() error = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Server did not shut down in time")
	}
}

func TestServer_Addr(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 0, // Use any available port
		},
	}

	srv := New(cfg)

	// Before starting, Addr should be empty
	if addr := srv.Addr(); addr != "" {
		t.Errorf("Addr() before start = %q, want empty", addr)
	}

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServeWithShutdown()
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// After starting, Addr should return the actual address
	addr := srv.Addr()
	if addr == "" {
		t.Error("Addr() after start = empty, want non-empty")
	}

	// Shut down
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	<-errCh
}

func TestServer_ShutdownBeforeStart(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
	}

	srv := New(cfg)

	// Shutdown before starting should not error
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := srv.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() before start error = %v, want nil", err)
	}
}

func TestServer_ShutdownTimeout(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 0, // Use any available port
		},
	}

	srv := New(cfg)

	// Add a handler that never completes
	requestStarted := make(chan struct{})
	srv.mux.HandleFunc("/stuck", func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		// Block forever - simulates a stuck request
		select {}
	})

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServeWithShutdown()
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Get the actual server address
	addr := srv.Addr()

	// Start a stuck request
	go func() {
		http.Get("http://" + addr + "/stuck")
	}()

	// Wait for request to start
	<-requestStarted

	// Shutdown with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := srv.Shutdown(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Shutdown() with stuck request error = %v, want %v", err, context.DeadlineExceeded)
	}
}
