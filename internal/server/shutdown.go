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
	if s.httpServer == nil {
		return nil
	}

	s.httpServer.mu.RLock()
	server := s.httpServer.server
	s.httpServer.mu.RUnlock()

	if server == nil {
		return nil
	}

	return server.Shutdown(ctx)
}

// Addr returns the address the server is listening on.
// Returns empty string if the server hasn't been started.
func (s *Server) Addr() string {
	if s.httpServer == nil {
		return ""
	}

	s.httpServer.mu.RLock()
	defer s.httpServer.mu.RUnlock()

	if s.httpServer.listener == nil {
		return ""
	}

	return s.httpServer.listener.Addr().String()
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

	s.httpServer = &httpServer{
		server: &http.Server{
			Handler: s.Handler(),
		},
		listener: listener,
	}

	// Channel for shutdown signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Channel to signal server has stopped
	serverDone := make(chan error, 1)

	go func() {
		if err := s.httpServer.server.Serve(listener); err != http.ErrServerClosed {
			serverDone <- err
			return
		}
		serverDone <- nil
	}()

	log.Printf("Server started on %s", listener.Addr().String())

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

	if err := s.httpServer.server.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
		return err
	}

	log.Println("Server shutdown complete")

	// Wait for Serve to return
	<-serverDone

	return nil
}
