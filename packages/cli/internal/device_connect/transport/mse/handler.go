package mse

import (
	"net/http"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/pipeline"
	"github.com/babelcloud/gbox/packages/cli/internal/util"
	"github.com/gorilla/mux"
)

// Handler provides HTTP endpoints for MSE fMP4 streaming.
type Handler struct {
	broadcaster *pipeline.Broadcaster
}

// NewHandler creates a new MSE HTTP handler.
func NewHandler(broadcaster *pipeline.Broadcaster) *Handler {
	return &Handler{
		broadcaster: broadcaster,
	}
}

// RegisterRoutes registers MSE-related routes with the given router.
func (h *Handler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/stream/video/{device_id}", h.handleVideoStream).Methods("GET")
}

// handleVideoStream serves the fMP4 video stream for MSE consumption.
func (h *Handler) handleVideoStream(w http.ResponseWriter, r *http.Request) {
	logger := util.GetLogger()
	vars := mux.Vars(r)
	deviceID := vars["device_id"]

	if deviceID == "" {
		http.Error(w, "Device ID required", http.StatusBadRequest)
		return
	}

	// Check if MSE mode is requested
	mode := r.URL.Query().Get("mode")
	if mode != "mse" {
		http.Error(w, "MSE mode required", http.StatusBadRequest)
		return
	}

	// Set appropriate headers for fMP4 streaming
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Range")

	// Generate unique subscriber ID for this request
	subscriberID := util.GenerateRandomString(8) + "_mse_" + deviceID

	// Subscribe to the broadcaster
	dataCh := h.broadcaster.Subscribe(subscriberID, 50) // Buffer 50 chunks
	defer h.broadcaster.Unsubscribe(subscriberID)

	logger.Info("MSE video stream started", "device", deviceID, "subscriber", subscriberID, "remote", r.RemoteAddr)

	// Set up cleanup on client disconnect
	notify := w.(http.CloseNotifier).CloseNotify()

	// Stream data to client
	for {
		select {
		case data, ok := <-dataCh:
			if !ok {
				logger.Info("MSE data channel closed", "device", deviceID, "subscriber", subscriberID)
				return
			}

			// Write data to response
			if _, err := w.Write(data); err != nil {
				logger.Warn("Failed to write to MSE client", "device", deviceID, "subscriber", subscriberID, "error", err)
				return
			}

			// Flush data immediately for low latency
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}

		case <-notify:
			logger.Info("MSE client disconnected", "device", deviceID, "subscriber", subscriberID)
			return

		case <-r.Context().Done():
			logger.Info("MSE request context cancelled", "device", deviceID, "subscriber", subscriberID)
			return

		case <-time.After(30 * time.Second):
			// Timeout if no data for 30 seconds
			logger.Warn("MSE stream timeout", "device", deviceID, "subscriber", subscriberID)
			return
		}
	}
}
