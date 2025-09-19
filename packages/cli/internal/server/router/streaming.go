package router

import (
	"net/http"

	"github.com/babelcloud/gbox/packages/cli/internal/server/handlers"
)

// StreamingRouter handles all streaming routes
type StreamingRouter struct {
	handlers    *handlers.StreamingHandlers
	transformer *PathTransformer
}

// RegisterRoutes registers all streaming routes
func (r *StreamingRouter) RegisterRoutes(mux *http.ServeMux, server interface{}) {
	// Create handlers instance
	r.handlers = handlers.NewStreamingHandlers()

	// Create path transformer with /api prefix
	r.transformer = NewPathTransformer("/api")

	// Set server service dependency if server implements ServerService
	if serverService, ok := server.(handlers.ServerService); ok {
		r.handlers.SetServerService(serverService)
	}

	// Set the path prefix for URL responses
	r.handlers.SetPathPrefix("/api")

	// Video streaming endpoints (handlers expect /stream/video/ but we serve from /api/stream/video/)
	mux.HandleFunc("/api/stream/video/", r.transformer.TransformHandler(r.handlers.HandleVideoStream))

	// Audio streaming endpoints
	mux.HandleFunc("/api/stream/audio/", r.transformer.TransformHandler(r.handlers.HandleAudioStream))

	// Device control WebSocket
	mux.HandleFunc("/api/stream/control/", r.transformer.TransformHandler(r.handlers.HandleControlWebSocket))

	// Stream connection management endpoints
	mux.HandleFunc("/api/stream/", r.transformer.TransformHandler(r.handlers.HandleStreamConnect))

	// Stream info endpoint
	mux.HandleFunc("/api/stream/info", r.transformer.TransformHandler(r.handlers.HandleStreamInfo))

}

// GetPathPrefix returns the path prefix for this router
func (r *StreamingRouter) GetPathPrefix() string {
	return "/stream"
}
