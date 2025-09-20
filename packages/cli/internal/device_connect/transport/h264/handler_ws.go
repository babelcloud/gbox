package h264

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/scrcpy"
	"github.com/babelcloud/gbox/packages/cli/internal/util"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// WSHandler handles WebSocket-based H.264 streaming
type WSHandler struct {
	deviceSerial string
}

// NewWSHandler creates a new WebSocket handler for H.264 streaming
func NewWSHandler(deviceSerial string) *WSHandler {
	return &WSHandler{
		deviceSerial: deviceSerial,
	}
}

// ServeWebSocket handles WebSocket connections for H.264 streaming
func (h *WSHandler) ServeWebSocket(w http.ResponseWriter, r *http.Request) {
	logger := util.GetLogger()
	logger.Info("Starting H.264 WebSocket stream", "device", h.deviceSerial)

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("Failed to upgrade to WebSocket", "device", h.deviceSerial, "error", err)
		return
	}
	defer conn.Close()

	// Get or create scrcpy source with H.264 mode
	source, err := scrcpy.StartSourceWithMode(h.deviceSerial, r.Context(), "h264")
	if err != nil {
		logger.Error("Failed to start scrcpy source", "device", h.deviceSerial, "error", err)
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Failed to start stream: %v", err)))
		return
	}

	// Subscribe to video stream
	videoCh := source.SubscribeVideo("h264_ws", 100)
	defer source.UnsubscribeVideo("h264_ws")

	// Send SPS/PPS first if available
	if spsPps := source.GetSpsPps(); len(spsPps) > 0 {
		if err := conn.WriteMessage(websocket.BinaryMessage, spsPps); err != nil {
			logger.Error("Failed to write SPS/PPS", "device", h.deviceSerial, "error", err)
			return
		}
	}

	// Start goroutine to handle keyframe requests
	go func() {
		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				logger.Debug("H.264 WebSocket read error", "device", h.deviceSerial, "error", err)
				return
			}
			if messageType == websocket.TextMessage {
				var msg map[string]interface{}
				if err := json.Unmarshal(data, &msg); err == nil {
					if msgType, ok := msg["type"].(string); ok && msgType == "request_keyframe" {
						logger.Debug("Received keyframe request from H.264 client", "device", h.deviceSerial)
						source.RequestKeyframe()
					}
				}
			}
		}
	}()

	// Stream video data
	for {
		select {
		case <-r.Context().Done():
			logger.Info("H.264 WebSocket stream context cancelled", "device", h.deviceSerial)
			return

		case sample, ok := <-videoCh:
			if !ok {
				logger.Info("H.264 WebSocket video channel closed", "device", h.deviceSerial)
				return
			}

			// Send H.264 data as binary message
			if err := conn.WriteMessage(websocket.BinaryMessage, sample.Data); err != nil {
				logger.Error("Failed to write H.264 data", "device", h.deviceSerial, "error", err)
				return
			}
		}
	}
}
