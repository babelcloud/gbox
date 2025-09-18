package mse

import (
	"fmt"
	"net/http"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/scrcpy"
	"github.com/babelcloud/gbox/packages/cli/internal/util"
)

// Transport implements MSE streaming transport
type Transport struct {
	deviceSerial string
}

// NewTransport creates a new MSE transport
func NewTransport(deviceSerial string) *Transport {
	return &Transport{
		deviceSerial: deviceSerial,
	}
}

// ServeHTTP implements http.Handler for MSE fMP4 streaming
func (t *Transport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := util.GetLogger()
	logger.Info("Starting MSE stream", "device", t.deviceSerial)

	// Set headers for fMP4 streaming
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get or create scrcpy source (for future implementation)
	_, err := scrcpy.StartSource(t.deviceSerial, r.Context())
	if err != nil {
		logger.Error("Failed to start scrcpy source", "device", t.deviceSerial, "error", err)
		http.Error(w, fmt.Sprintf("Failed to start stream: %v", err), http.StatusInternalServerError)
		return
	}

	// For now, return a placeholder response indicating MSE is being developed
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("MSE mode is currently being refactored. Please use WebRTC or H.264 mode."))

	logger.Info("MSE stream request handled", "device", t.deviceSerial)
}
