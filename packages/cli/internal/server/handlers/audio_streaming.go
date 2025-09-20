package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/audio"
)

// AudioStreamingHandler handles HTTP requests for audio streaming
type AudioStreamingHandler struct {
	audioService audio.AudioService
	logger       *slog.Logger
}

// NewAudioStreamingHandler creates a new audio streaming handler
func NewAudioStreamingHandler(audioService audio.AudioService) *AudioStreamingHandler {
	return &AudioStreamingHandler{
		audioService: audioService,
		logger:       slog.With("component", "audio_streaming_handler"),
	}
}

// HandleWebMStream handles WebM audio streaming requests
func (h *AudioStreamingHandler) HandleWebMStream(w http.ResponseWriter, r *http.Request) {
	deviceSerial := r.URL.Query().Get("device")
	if deviceSerial == "" {
		h.logger.Error("Missing device parameter")
		http.Error(w, "Missing device parameter", http.StatusBadRequest)
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "webm" // Default to WebM
	}

	h.logger.Info("Handling WebM stream request", "device", deviceSerial, "format", format)

	// Set HTTP headers for WebM streaming
	w.Header().Set("Content-Type", "audio/webm; codecs=opus")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Range")

	// Start streaming immediately
	w.WriteHeader(http.StatusOK)

	// Ensure we can flush chunks
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.logger.Error("Streaming not supported")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}
	flusher.Flush()

	// Stream the audio
	if err := h.audioService.StreamWebM(deviceSerial, w, r); err != nil {
		h.logger.Error("Audio streaming failed", "error", err)
		// Don't send error response as client might have disconnected
		return
	}
}

// HandleOpusStream handles Opus audio streaming requests
func (h *AudioStreamingHandler) HandleOpusStream(w http.ResponseWriter, r *http.Request) {
	deviceSerial := r.URL.Query().Get("device")
	if deviceSerial == "" {
		h.logger.Error("Missing device parameter")
		http.Error(w, "Missing device parameter", http.StatusBadRequest)
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "webm" // Default to WebM
	}

	h.logger.Info("Handling Opus stream request", "device", deviceSerial, "format", format)

	// Set appropriate headers
	w.Header().Set("Content-Type", "audio/webm; codecs=opus")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Stream the audio
	if err := h.audioService.StreamOpus(deviceSerial, w, format); err != nil {
		h.logger.Error("Audio streaming failed", "error", err)
		http.Error(w, fmt.Sprintf("Audio streaming failed: %v", err), http.StatusInternalServerError)
		return
	}
}

// RegisterRoutes registers audio streaming routes
func (h *AudioStreamingHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/stream/audio/", h.handleAudioStream)
}

// handleAudioStream routes audio streaming requests based on query parameters
func (h *AudioStreamingHandler) handleAudioStream(w http.ResponseWriter, r *http.Request) {
	mse := r.URL.Query().Get("mse")
	
	if mse == "true" {
		h.HandleWebMStream(w, r)
	} else {
		h.HandleOpusStream(w, r)
	}
}
