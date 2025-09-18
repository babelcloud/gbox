package h264

import (
	"fmt"
	"net/http"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/scrcpy"
	"github.com/babelcloud/gbox/packages/cli/internal/util"
)

// HTTPHandler handles HTTP-based H.264 streaming
type HTTPHandler struct {
	deviceSerial string
}

// NewHTTPHandler creates a new HTTP handler for H.264 streaming
func NewHTTPHandler(deviceSerial string) *HTTPHandler {
	return &HTTPHandler{
		deviceSerial: deviceSerial,
	}
}

// ServeHTTP implements http.Handler for direct H.264 streaming
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := util.GetLogger()
	logger.Info("Starting H.264 HTTP stream", "device", h.deviceSerial)

	// Set headers for H.264 streaming
	w.Header().Set("Content-Type", "video/h264")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Get or create scrcpy source with H.264 mode
	source, err := scrcpy.StartSourceWithMode(h.deviceSerial, r.Context(), "h264")
	if err != nil {
		logger.Error("Failed to start scrcpy source", "device", h.deviceSerial, "error", err)
		http.Error(w, fmt.Sprintf("Failed to start stream: %v", err), http.StatusInternalServerError)
		return
	}

	// Generate unique subscriber ID for this connection
	subscriberID := fmt.Sprintf("h264_http_%d", time.Now().UnixNano())

	// Subscribe to video stream
	videoCh := source.SubscribeVideo(subscriberID, 100)
	defer source.UnsubscribeVideo(subscriberID)

	// Send SPS/PPS first if available
	if spsPps := source.GetSpsPps(); len(spsPps) > 0 {
		if _, err := w.Write(spsPps); err != nil {
			logger.Error("Failed to write SPS/PPS", "device", h.deviceSerial, "error", err)
			return
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	// Stream video data
	for {
		select {
		case <-r.Context().Done():
			logger.Info("H.264 HTTP stream context cancelled", "device", h.deviceSerial)
			return

		case sample, ok := <-videoCh:
			if !ok {
				logger.Info("H.264 HTTP video channel closed", "device", h.deviceSerial)
				return
			}

			// Write H.264 data directly
			if _, err := w.Write(sample.Data); err != nil {
				logger.Error("Failed to write H.264 data", "device", h.deviceSerial, "error", err)
				return
			}

			// Flush data immediately for low latency
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}
}
