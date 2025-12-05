package server

import (
	"bufio"
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/control"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/webrtc"
	"github.com/babelcloud/gbox/packages/cli/internal/server/handlers"
	"github.com/babelcloud/gbox/packages/cli/internal/server/router"
	"github.com/pkg/errors"
)

//go:embed all:static
var staticFiles embed.FS

// GBoxServer is the unified server for all gbox services
type GBoxServer struct {
	port       int
	httpServer *http.Server
	mux        *http.ServeMux

	// Services
	bridgeManager *webrtc.Manager
	deviceKeeper  *DeviceKeeper

	// State
	mu        sync.RWMutex
	running   bool
	startTime time.Time
	buildID   string // Store build ID at startup
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewGBoxServer creates a new unified gbox server
func NewGBoxServer(port int) *GBoxServer {
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize control service
	control.SetControlService()

	return &GBoxServer{
		port:          port,
		mux:           http.NewServeMux(),
		bridgeManager: webrtc.NewManager("adb"),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start starts the unified server
func (s *GBoxServer) Start() error {
	// Set start time and build ID
	s.startTime = time.Now()
	s.buildID = GetBuildID()

	// Setup routes
	s.setupRoutes()

	if err := s.startDeviceKeeper(); err != nil {
		return err
	}

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      loggingMiddleware(s.mux),
		ReadTimeout:  0, // No read timeout for streaming connections
		WriteTimeout: 0, // No write timeout for streaming connections
		IdleTimeout:  0, // No idle timeout for streaming connections
	}

	return s.httpServer.ListenAndServe()
}

// Stop stops the server
func (s *GBoxServer) Stop() error {
	s.cancel()

	// Shutdown HTTP server with longer timeout
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
			// Force close if graceful shutdown fails
			if err := s.httpServer.Close(); err != nil {
				log.Printf("HTTP server force close error: %v", err)
			}
		}
	}

	// Cleanup services
	s.bridgeManager.Close()
	s.deviceKeeper.Close()

	log.Println("GBox server stopped")
	return nil
}

func (s *GBoxServer) startDeviceKeeper() error {
	var err error
	s.deviceKeeper, err = NewDeviceKeeper()
	if err != nil {
		return errors.Wrap(err, "failed to create device keeper")
	}
	if err := s.deviceKeeper.Start(); err != nil {
		return errors.Wrap(err, "failed to start device keeper")
	}
	return nil
}

// IsRunning returns whether the server is running
func (s *GBoxServer) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// setupRoutes sets up all HTTP routes using the new router system
func (s *GBoxServer) setupRoutes() {
	// Register routers in order of specificity (most specific first)
	routers := []router.Router{
		&router.APIRouter{},
		&router.ADBExposeRouter{},
		&router.AssetsRouter{},
		&router.PagesRouter{}, // Must be last as it includes root handler
	}

	// Register all routes
	for _, r := range routers {
		r.RegisterRoutes(s.mux, s)
	}
}

// ServerService interface implementations for handlers

// GetPort returns the server port
func (s *GBoxServer) GetPort() int {
	return s.port
}

// GetUptime returns server uptime
func (s *GBoxServer) GetUptime() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.startTime)
}

// GetBuildID returns build ID
func (s *GBoxServer) GetBuildID() string {
	return s.buildID
}

// GetVersion returns version info
func (s *GBoxServer) GetVersion() string {
	return BuildInfo.Version
}

// IsADBExposeRunning returns ADB expose status
func (s *GBoxServer) IsADBExposeRunning() bool {
	return true // Always available through handlers
}

// ListBridges returns list of bridge device serials
func (s *GBoxServer) ListBridges() []string {
	return s.bridgeManager.ListBridges()
}

// CreateBridge creates a bridge for device
func (s *GBoxServer) CreateBridge(deviceSerial string) error {
	_, err := s.bridgeManager.CreateBridge(deviceSerial)
	return err
}

// RemoveBridge removes a bridge
func (s *GBoxServer) RemoveBridge(deviceSerial string) {
	s.bridgeManager.RemoveBridge(deviceSerial)
}

// GetBridge gets a bridge by device serial
func (s *GBoxServer) GetBridge(deviceSerial string) (handlers.Bridge, bool) {
	bridge, exists := s.bridgeManager.GetBridge(deviceSerial)
	return bridge, exists
}

// GetStaticFS returns static file system
func (s *GBoxServer) GetStaticFS() fs.FS {
	return staticFiles
}

// StartPortForward starts port forwarding for ADB expose
// This method is kept for ServerService interface compatibility
// but ADB functionality is now handled by ADBExposeHandlers
func (s *GBoxServer) StartPortForward(boxID string, localPorts, remotePorts []int) error {
	return fmt.Errorf("ADB port forwarding is now handled through API endpoints")
}

// StopPortForward stops port forwarding for ADB expose
// This method is kept for ServerService interface compatibility
func (s *GBoxServer) StopPortForward(boxID string) error {
	return fmt.Errorf("ADB port forwarding is now handled through API endpoints")
}

// ListPortForwards lists all active port forwards
// This method is kept for ServerService interface compatibility
func (s *GBoxServer) ListPortForwards() interface{} {
	return map[string]interface{}{
		"forwards": []interface{}{},
		"count":    0,
		"message":  "ADB port forwarding is now handled through API endpoints",
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	length int
}

func (lw *loggingResponseWriter) WriteHeader(code int) {
	lw.status = code
	lw.ResponseWriter.WriteHeader(code)
}

func (lw *loggingResponseWriter) Write(b []byte) (int, error) {
	if lw.status == 0 {
		lw.status = http.StatusOK
	}
	n, err := lw.ResponseWriter.Write(b)
	lw.length += n
	return n, err
}

func (w *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("http.Hijacker interface is not supported")
	}
	return hj.Hijack()
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(lw, r)
		duration := time.Since(start)
		remoteAddr := r.RemoteAddr
		log.Printf("%s %s %d %d %s %s", r.Method, r.URL.Path, lw.status, lw.length, duration, remoteAddr)
	})
}

func (s *GBoxServer) ConnectAP(serial string) error {
	return s.deviceKeeper.connectAP(serial)
}

func (s *GBoxServer) DisconnectAP(serial string) error {
	return s.deviceKeeper.disconnectAPForce(serial)
}

func (s *GBoxServer) GetSerialByDeviceId(deviceId string) string {
	return s.deviceKeeper.getSerialByDeviceId(deviceId)
}

func (s *GBoxServer) GetDeviceInfo(serial string) interface{} {
	return s.deviceKeeper.GetDeviceInfo(serial)
}

func (s *GBoxServer) UpdateDeviceInfo(device interface{}) {
	if dto, ok := device.(*handlers.DeviceDTO); ok {
		s.deviceKeeper.updateDeviceInfo(dto)
	}
}

func (s *GBoxServer) IsDeviceConnected(serial string) bool {
	return s.deviceKeeper.IsDeviceConnected(serial)
}

func (s *GBoxServer) GetDeviceReconnectState(serial string) interface{} {
	return s.deviceKeeper.getReconnectState(serial)
}

func (s *GBoxServer) ReconnectRegisteredDevices() error {
	return s.deviceKeeper.ReconnectRegisteredDevices()
}
