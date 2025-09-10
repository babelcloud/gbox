package device_connect

import (
	"fmt"
	"log"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/api"
)

// Server is the main device connect server
type Server struct {
	apiServer *api.Server
	port      int
}

// NewServer creates a new device connect server
func NewServer(port int) *Server {
	return &Server{
		port:      port,
		apiServer: api.NewServer(port),
	}
}

// Start starts the device connect server
func (s *Server) Start() error {
	log.Printf("Starting device connect server on port %d", s.port)
	
	// Start the API server
	if err := s.apiServer.Start(); err != nil {
		return fmt.Errorf("failed to start API server: %w", err)
	}
	
	log.Printf("Device connect server started successfully on http://localhost:%d", s.port)
	return nil
}

// Stop stops the device connect server
func (s *Server) Stop() error {
	log.Println("Stopping device connect server...")
	
	if s.apiServer != nil {
		if err := s.apiServer.Stop(); err != nil {
			return fmt.Errorf("failed to stop API server: %w", err)
		}
	}
	
	log.Println("Device connect server stopped")
	return nil
}

// IsRunning returns whether the server is running
func (s *Server) IsRunning() bool {
	if s.apiServer != nil {
		return s.apiServer.IsRunning()
	}
	return false
}