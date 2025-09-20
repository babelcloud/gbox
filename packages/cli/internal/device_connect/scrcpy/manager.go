package scrcpy

import (
	"context"
	"sync"

	"github.com/babelcloud/gbox/packages/cli/internal/util"
)

// GlobalManager manages shared scrcpy sources per device
type GlobalManager struct {
	mu      sync.RWMutex
	sources map[string]*Source
}

var globalManager = &GlobalManager{
	sources: make(map[string]*Source),
}

// GetOrCreateSource returns an existing source or creates a new one
func GetOrCreateSource(deviceSerial string) *Source {
	return GetOrCreateSourceWithMode(deviceSerial, "webrtc") // Default mode
}

// GetOrCreateSourceWithMode returns an existing source or creates a new one with specific mode
func GetOrCreateSourceWithMode(deviceSerial string, streamingMode string) *Source {
	globalManager.mu.Lock()
	defer globalManager.mu.Unlock()

	if src, exists := globalManager.sources[deviceSerial]; exists {
		// Check if source is still valid (has active cancel function)
		src.mu.Lock()
		if src.cancel != nil {
			// Source is still active, update streaming mode if different
			if src.streamingMode != streamingMode {
				util.GetLogger().Info("Updating streaming mode", "device", deviceSerial, "from", src.streamingMode, "to", streamingMode)
				src.streamingMode = streamingMode
			}
			src.mu.Unlock()
			util.GetLogger().Info("Using existing scrcpy source", "device", deviceSerial, "mode", streamingMode)
			return src
		}
		src.mu.Unlock()

		// Source exists but is not active, remove it and create a new one
		util.GetLogger().Info("Removing inactive scrcpy source", "device", deviceSerial)
		delete(globalManager.sources, deviceSerial)
	}

	util.GetLogger().Info("Creating new scrcpy source", "device", deviceSerial, "mode", streamingMode)
	src := NewSourceWithMode(deviceSerial, streamingMode)
	globalManager.sources[deviceSerial] = src
	return src
}

// StartSource starts a source if not already started
func StartSource(deviceSerial string, ctx context.Context) (*Source, error) {
	return StartSourceWithMode(deviceSerial, ctx, "webrtc") // Default mode
}

// StartSourceWithMode starts a source with specific streaming mode
func StartSourceWithMode(deviceSerial string, ctx context.Context, streamingMode string) (*Source, error) {
	src := GetOrCreateSourceWithMode(deviceSerial, streamingMode)

	// Check if already started
	src.mu.Lock()
	if src.cancel != nil {
		src.mu.Unlock()
		util.GetLogger().Info("Scrcpy source already started", "device", deviceSerial)
		return src, nil
	}
	src.mu.Unlock()

	// Start the source
	if err := src.Start(ctx, deviceSerial); err != nil {
		util.GetLogger().Error("Failed to start scrcpy source", "device", deviceSerial, "error", err)

		// If start failed, clean up the source state
		src.mu.Lock()
		src.cancel = nil
		src.mu.Unlock()

		return nil, err
	}

	util.GetLogger().Info("Scrcpy source started successfully", "device", deviceSerial)
	return src, nil
}

// RemoveSource removes a source from the global manager
func RemoveSource(deviceSerial string) {
	globalManager.mu.Lock()
	defer globalManager.mu.Unlock()

	if src, exists := globalManager.sources[deviceSerial]; exists {
		src.Stop()
		delete(globalManager.sources, deviceSerial)
		util.GetLogger().Info("Removed scrcpy source", "device", deviceSerial)
	}
}

// GetSource returns an existing source if it exists
func GetSource(deviceSerial string) *Source {
	globalManager.mu.RLock()
	defer globalManager.mu.RUnlock()
	return globalManager.sources[deviceSerial]
}
