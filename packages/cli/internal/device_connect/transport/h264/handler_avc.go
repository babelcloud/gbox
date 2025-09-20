package h264

import (
	"fmt"
	"net/http"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/scrcpy"
	"github.com/babelcloud/gbox/packages/cli/internal/util"
)

// AVCHTTPHandler handles HTTP-based AVC format H.264 streaming
type AVCHTTPHandler struct {
	deviceSerial string
	converter    *AnnexBToAVCConverter
}

// NewAVCHTTPHandler creates a new HTTP handler for AVC format H.264 streaming
func NewAVCHTTPHandler(deviceSerial string) *AVCHTTPHandler {
	return &AVCHTTPHandler{
		deviceSerial: deviceSerial,
		converter:    NewAnnexBToAVCConverter(),
	}
}

// ServeHTTP implements http.Handler for AVC format H.264 streaming
func (h *AVCHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := util.GetLogger()
	logger.Info("Starting AVC format H.264 HTTP stream", "device", h.deviceSerial)

	// Set headers for AVC format H.264 streaming
	w.Header().Set("Content-Type", "video/h264")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Format", "avc") // Custom header to indicate AVC format

	// Get or create scrcpy source with H.264 mode
	source, err := scrcpy.StartSourceWithMode(h.deviceSerial, r.Context(), "h264")
	if err != nil {
		logger.Error("Failed to start scrcpy source", "device", h.deviceSerial, "error", err)
		http.Error(w, fmt.Sprintf("Failed to start stream: %v", err), http.StatusInternalServerError)
		return
	}

	// Generate unique subscriber ID for this connection
	subscriberID := fmt.Sprintf("avc_http_%d", time.Now().UnixNano())

	// Subscribe to video stream
	videoCh := source.SubscribeVideo(subscriberID, 100)
	defer source.UnsubscribeVideo(subscriberID)

	// Send SPS/PPS first if available (convert to AVC format)
	if spsPps := source.GetSpsPps(); len(spsPps) > 0 {
		avcSpsPps, convertErr := h.converter.Convert(spsPps)
		if convertErr != nil {
			logger.Error("Failed to convert SPS/PPS to AVC format", "device", h.deviceSerial, "error", convertErr)
			http.Error(w, "Failed to convert SPS/PPS", http.StatusInternalServerError)
			return
		}

		if len(avcSpsPps) > 0 {
			if _, err := w.Write(avcSpsPps); err != nil {
				logger.Error("Failed to write AVC SPS/PPS", "device", h.deviceSerial, "error", err)
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			logger.Info("Sent AVC SPS/PPS", "device", h.deviceSerial, "size", len(avcSpsPps))
		}
	}

	// Stream video data
	frameCount := 0
	for {
		select {
		case <-r.Context().Done():
			logger.Info("AVC HTTP stream context cancelled", "device", h.deviceSerial)
			return

		case sample, ok := <-videoCh:
			if !ok {
				logger.Info("AVC HTTP video channel closed", "device", h.deviceSerial)
				return
			}

			frameCount++

			// Convert H.264 Annex-B data to AVC format
			avcData, convertErr := h.converter.Convert(sample.Data)
			if convertErr != nil {
				logger.Error("Failed to convert H.264 data to AVC format",
					"device", h.deviceSerial,
					"frame", frameCount,
					"error", convertErr)
				continue // Skip this frame but continue streaming
			}

			// Write AVC format data
			if len(avcData) > 0 {
				if _, err := w.Write(avcData); err != nil {
					logger.Error("Failed to write AVC data",
						"device", h.deviceSerial,
						"frame", frameCount,
						"error", err)
					return
				}

				// Flush data immediately for low latency
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}

				// Log first few frames for debugging
				if frameCount <= 5 {
					logger.Info("Sent AVC frame",
						"device", h.deviceSerial,
						"frame", frameCount,
						"originalSize", len(sample.Data),
						"avcSize", len(avcData))
				}
			} else {
				logger.Warn("Empty AVC data after conversion",
					"device", h.deviceSerial,
					"frame", frameCount,
					"originalSize", len(sample.Data))
			}
		}
	}
}
