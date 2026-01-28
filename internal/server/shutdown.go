package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// httpServer holds the HTTP server instance and its listener.
type httpServer struct {
	server   *http.Server
	listener net.Listener
	mu       sync.RWMutex
}

// Shutdown gracefully shuts down the server.
// If the server hasn't been started, this is a no-op.
func (s *Server) Shutdown(ctx context.Context) error {
	s.httpServerMu.RLock()
	hs := s.httpServer
	s.httpServerMu.RUnlock()

	if hs == nil {
		return nil
	}

	hs.mu.RLock()
	server := hs.server
	hs.mu.RUnlock()

	if server == nil {
		return nil
	}

	return server.Shutdown(ctx)
}

// Addr returns the address the server is listening on.
// Returns empty string if the server hasn't been started.
func (s *Server) Addr() string {
	s.httpServerMu.RLock()
	hs := s.httpServer
	s.httpServerMu.RUnlock()

	if hs == nil {
		return ""
	}

	hs.mu.RLock()
	defer hs.mu.RUnlock()

	if hs.listener == nil {
		return ""
	}

	return hs.listener.Addr().String()
}

// ListenAndServeWithShutdown starts the server with graceful shutdown handling.
// It listens for SIGINT and SIGTERM signals and initiates graceful shutdown.
// Returns nil on successful shutdown, or an error if the server fails to start.
func (s *Server) ListenAndServeWithShutdown() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)

	// Create listener first so we know the actual address (important for port 0)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	hs := &httpServer{
		server: &http.Server{
			Handler: s.Handler(),
		},
		listener: listener,
	}

	s.httpServerMu.Lock()
	s.httpServer = hs
	s.httpServerMu.Unlock()

	// Channel for shutdown signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Channel to signal server has stopped
	serverDone := make(chan error, 1)

	go func() {
		if err := hs.server.Serve(listener); err != http.ErrServerClosed {
			serverDone <- err
			return
		}
		serverDone <- nil
	}()

	log.Printf("Server started on %s", listener.Addr().String())

	// Signal that server is ready
	close(s.ready)

	// Wait for shutdown signal or programmatic shutdown
	select {
	case sig := <-shutdown:
		log.Printf("Received signal %v, initiating shutdown...", sig)
	case err := <-serverDone:
		// Server stopped on its own (error or shutdown called)
		if err != nil {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	}

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := hs.server.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
		return err
	}

	log.Println("Server shutdown complete")

	// Wait for Serve to return
	<-serverDone

	return nil
}
