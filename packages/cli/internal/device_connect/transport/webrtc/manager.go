package webrtc

import (
	"fmt"
	"sync"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/util"
	"github.com/pion/webrtc/v4"
)

// Manager manages WebRTC bridges for multiple devices
// This replaces the old separate webrtc.Manager
type Manager struct {
	bridges map[string]*Bridge // deviceSerial -> bridge
	mu      sync.RWMutex
	adbPath string
}

// NewManager creates a new unified bridge manager
func NewManager(adbPath string) *Manager {
	return &Manager{
		bridges: make(map[string]*Bridge),
		adbPath: adbPath,
	}
}

// CreateBridge creates a new WebRTC bridge for a device
func (m *Manager) CreateBridge(deviceSerial string) (*Bridge, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if bridge already exists and is in a valid state
	if existing := m.bridges[deviceSerial]; existing != nil {
		if pc := existing.GetPeerConnection(); pc != nil {
			state := pc.ConnectionState()
			if state != webrtc.PeerConnectionStateClosed && state != webrtc.PeerConnectionStateFailed && state != webrtc.PeerConnectionStateDisconnected {
				logger := util.GetLogger()
				logger.Info("Reusing existing WebRTC bridge", "device", deviceSerial, "state", state.String())
				return existing, nil
			}
			// Connection is closed/failed/disconnected, remove and recreate
			logger := util.GetLogger()
			logger.Info("Removing invalid WebRTC bridge for recreation", "device", deviceSerial, "state", state.String())
			existing.Close()
			delete(m.bridges, deviceSerial)

			// Add longer delay for ICE connection cleanup
			time.Sleep(500 * time.Millisecond)
		}
	}

	// Create new WebRTC bridge
	bridge, err := NewBridge(deviceSerial, m.adbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create WebRTC bridge: %w", err)
	}

	// Start the bridge
	if err := bridge.Start(); err != nil {
		bridge.Close()
		return nil, fmt.Errorf("failed to start WebRTC bridge: %w", err)
	}

	m.bridges[deviceSerial] = bridge

	logger := util.GetLogger()
	logger.Info("WebRTC bridge created", "device", deviceSerial)

	return bridge, nil
}

// GetBridge returns an existing bridge for a device
func (m *Manager) GetBridge(deviceSerial string) (*Bridge, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	bridge, exists := m.bridges[deviceSerial]
	return bridge, exists
}

// RemoveBridge removes and closes a bridge for a device
func (m *Manager) RemoveBridge(deviceSerial string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if bridge, exists := m.bridges[deviceSerial]; exists {
		bridge.Close()
		delete(m.bridges, deviceSerial)

		logger := util.GetLogger()
		logger.Info("WebRTC bridge removed", "device", deviceSerial)
	}
}

// Close closes all bridges and shuts down the manager
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for deviceSerial, bridge := range m.bridges {
		bridge.Close()
		delete(m.bridges, deviceSerial)
	}

	return nil
}

// ListBridges returns all active bridge device serials
func (m *Manager) ListBridges() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var devices []string
	for deviceSerial := range m.bridges {
		devices = append(devices, deviceSerial)
	}
	return devices
}
