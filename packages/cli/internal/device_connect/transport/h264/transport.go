package h264

import (
	"net/http"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/scrcpy"
	"github.com/babelcloud/gbox/packages/cli/internal/util"
	"github.com/gorilla/mux"
)

// Global handlers - initialized when needed
var (
	httpHandler *HTTPHandler
	wsHandler   *WSHandler
)

// ServeHTTP provides a package-level HTTP handler for H.264 streaming
func ServeHTTP(w http.ResponseWriter, r *http.Request, deviceSerial string) {
	logger := util.GetLogger()

	// Get or create scrcpy source with H.264 mode
	_, err := scrcpy.StartSourceWithMode(deviceSerial, r.Context(), "h264")
	if err != nil {
		logger.Error("Failed to start scrcpy source", "device", deviceSerial, "error", err)
		http.Error(w, "Failed to start device source", http.StatusInternalServerError)
		return
	}

	// Create handler if not exists
	if httpHandler == nil {
		httpHandler = NewHTTPHandler(deviceSerial)
	}

	// Create a fake mux.Router just for this request to extract device_id
	router := mux.NewRouter()
	router.HandleFunc("/stream/video/{device_id}", httpHandler.ServeHTTP).Methods("GET")

	// Modify the request URL to include device_id
	r.URL.Path = "/stream/video/" + deviceSerial

	// Serve the request
	router.ServeHTTP(w, r)
}

// ServeWS provides a package-level WebSocket handler for H.264 streaming
func ServeWS(w http.ResponseWriter, r *http.Request, deviceSerial string) {
	logger := util.GetLogger()

	// Get or create scrcpy source with H.264 mode
	_, err := scrcpy.StartSourceWithMode(deviceSerial, r.Context(), "h264")
	if err != nil {
		logger.Error("Failed to start scrcpy source", "device", deviceSerial, "error", err)
		http.Error(w, "Failed to start device source", http.StatusInternalServerError)
		return
	}

	// Create handler if not exists
	if wsHandler == nil {
		wsHandler = NewWSHandler(deviceSerial)
	}

	// Create a fake mux.Router just for this request to extract device_id
	router := mux.NewRouter()
	router.HandleFunc("/ws/video/{device_id}", wsHandler.ServeWebSocket).Methods("GET")

	// Modify the request URL to include device_id
	r.URL.Path = "/ws/video/" + deviceSerial

	// Serve the request
	router.ServeHTTP(w, r)
}
