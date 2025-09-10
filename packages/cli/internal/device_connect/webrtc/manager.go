package webrtc

import (
	"log"
	"sync"
)

// Manager manages WebRTC bridges for multiple devices
type Manager struct {
	bridges map[string]*Bridge
	mu      sync.RWMutex
	adbPath string
}

// NewManager creates a new WebRTC manager
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

	// Remove existing bridge if any
	if existing, exists := m.bridges[deviceSerial]; exists {
		existing.Close()
		delete(m.bridges, deviceSerial)
	}

	// Create new bridge
	bridge, err := NewBridge(deviceSerial, m.adbPath)
	if err != nil {
		return nil, err
	}

	// Start the bridge
	if err := bridge.Start(); err != nil {
		bridge.Close()
		return nil, err
	}

	m.bridges[deviceSerial] = bridge
	log.Printf("Created WebRTC bridge for device %s", deviceSerial)
	
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
		log.Printf("Removed WebRTC bridge for device %s", deviceSerial)
	}
}

// Close closes all bridges
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for serial, bridge := range m.bridges {
		if err := bridge.Close(); err != nil {
			log.Printf("Error closing bridge for device %s: %v", serial, err)
		}
	}

	m.bridges = make(map[string]*Bridge)
	return nil
}