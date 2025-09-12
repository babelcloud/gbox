package api

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// handleWebSocket handles WebSocket connections for WebRTC signaling
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket: %v", err)
		return
	}
	defer conn.Close()

	log.Println("WebSocket connection established")

	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		msgType, ok := msg["type"].(string)
		if !ok {
			continue
		}

		switch msgType {
		case "connect":
			s.handleWebSocketConnect(conn, msg)
		case "offer":
			s.handleWebSocketOffer(conn, msg)
		case "ice-candidate":
			s.handleWebSocketICECandidate(conn, msg)
		case "disconnect":
			s.handleWebSocketDisconnect(conn, msg)
		case "touch":
			s.handleWebSocketTouch(conn, msg)
		case "key":
			s.handleWebSocketKey(conn, msg)
		case "scroll":
			s.handleWebSocketScroll(conn, msg)
		}
	}
}

// handleWebSocketConnect handles WebSocket connect message
func (s *Server) handleWebSocketConnect(conn *websocket.Conn, msg map[string]interface{}) {
	deviceSerial, ok := msg["deviceSerial"].(string)
	if !ok {
		conn.WriteJSON(map[string]interface{}{
			"type":  "error",
			"error": "Device serial required",
		})
		return
	}

	bridge, exists := s.webrtcManager.GetBridge(deviceSerial)
	if !exists {
		var err error
		bridge, err = s.webrtcManager.CreateBridge(deviceSerial)
		if err != nil {
			log.Printf("Failed to create bridge: %v", err)
			conn.WriteJSON(map[string]interface{}{
				"type":  "error",
				"error": err.Error(),
			})
			return
		}
	}

	bridge.WSConnection = conn

	conn.WriteJSON(map[string]interface{}{
		"type":         "connected",
		"deviceSerial": deviceSerial,
	})
}

// handleWebSocketOffer handles WebRTC offer
func (s *Server) handleWebSocketOffer(conn *websocket.Conn, msg map[string]interface{}) {
	deviceSerial, ok := msg["deviceSerial"].(string)
	if !ok {
		return
	}

	offerData, ok := msg["offer"].(map[string]interface{})
	if !ok {
		return
	}

	sdp, ok := offerData["sdp"].(string)
	if !ok {
		return
	}

	// Get or create bridge for the device
	bridge, exists := s.webrtcManager.GetBridge(deviceSerial)
	if !exists {
		log.Printf("Bridge not found for device %s, creating new bridge", deviceSerial)
		var err error
		bridge, err = s.webrtcManager.CreateBridge(deviceSerial)
		if err != nil {
			log.Printf("Failed to create bridge: %v", err)
			conn.WriteJSON(map[string]interface{}{
				"type":  "error",
				"error": fmt.Sprintf("Failed to connect to device: %v", err),
			})
			return
		}
	}

	// Check signaling state - only recreate if truly necessary
	signalingState := bridge.WebRTCConn.SignalingState()
	connState := bridge.WebRTCConn.ConnectionState()

	log.Printf("Bridge state for device %s: signaling=%s, connection=%s", deviceSerial, signalingState, connState)

	// Only recreate bridge if connection is truly closed or failed
	if connState == webrtc.PeerConnectionStateClosed || connState == webrtc.PeerConnectionStateFailed {
		log.Printf("WebRTC connection is %s for device %s, recreating bridge", connState, deviceSerial)
		s.webrtcManager.RemoveBridge(deviceSerial)

		// Create new bridge
		var err error
		bridge, err = s.webrtcManager.CreateBridge(deviceSerial)
		if err != nil {
			log.Printf("Failed to recreate bridge: %v", err)
			conn.WriteJSON(map[string]interface{}{
				"type":  "error",
				"error": fmt.Sprintf("Failed to reconnect to device: %v", err),
			})
			return
		}
	} else if signalingState == webrtc.SignalingStateClosed {
		// Only recreate if signaling is closed but connection is still active
		log.Printf("Signaling state is closed for device %s, recreating bridge", deviceSerial)
		s.webrtcManager.RemoveBridge(deviceSerial)

		var err error
		bridge, err = s.webrtcManager.CreateBridge(deviceSerial)
		if err != nil {
			log.Printf("Failed to recreate bridge: %v", err)
			conn.WriteJSON(map[string]interface{}{
				"type":  "error",
				"error": fmt.Sprintf("Failed to reset connection: %v", err),
			})
			return
		}
	}

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sdp,
	}

	if err := bridge.WebRTCConn.SetRemoteDescription(offer); err != nil {
		log.Printf("Failed to set remote description: %v", err)
		conn.WriteJSON(map[string]interface{}{
			"type":  "error",
			"error": err.Error(),
		})
		return
	}

	answer, err := bridge.WebRTCConn.CreateAnswer(nil)
	if err != nil {
		log.Printf("Failed to create answer: %v", err)
		conn.WriteJSON(map[string]interface{}{
			"type":  "error",
			"error": err.Error(),
		})
		return
	}

	if err := bridge.WebRTCConn.SetLocalDescription(answer); err != nil {
		log.Printf("Failed to set local description: %v", err)
		conn.WriteJSON(map[string]interface{}{
			"type":  "error",
			"error": err.Error(),
		})
		return
	}

	conn.WriteJSON(map[string]interface{}{
		"type": "answer",
		"answer": map[string]interface{}{
			"type": "answer",
			"sdp":  answer.SDP,
		},
	})

	// Set up ICE candidate handler
	bridge.WebRTCConn.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}

		candidateJSON := candidate.ToJSON()
		conn.WriteJSON(map[string]interface{}{
			"type": "ice-candidate",
			"candidate": map[string]interface{}{
				"candidate":     candidateJSON.Candidate,
				"sdpMLineIndex": candidateJSON.SDPMLineIndex,
				"sdpMid":        candidateJSON.SDPMid,
			},
		})
	})

	// Device info is not needed by frontend, video dimensions will be available through video track
}

// handleWebSocketICECandidate handles ICE candidate
func (s *Server) handleWebSocketICECandidate(conn *websocket.Conn, msg map[string]interface{}) {
	deviceSerial, ok := msg["deviceSerial"].(string)
	if !ok {
		return
	}

	candidateData, ok := msg["candidate"].(map[string]interface{})
	if !ok {
		return
	}

	bridge, exists := s.webrtcManager.GetBridge(deviceSerial)
	if !exists {
		return
	}

	candidate := webrtc.ICECandidateInit{
		Candidate: candidateData["candidate"].(string),
	}

	if sdpMLineIndex, ok := candidateData["sdpMLineIndex"].(float64); ok {
		index := uint16(sdpMLineIndex)
		candidate.SDPMLineIndex = &index
	}

	if sdpMid, ok := candidateData["sdpMid"].(string); ok {
		candidate.SDPMid = &sdpMid
	}

	if err := bridge.WebRTCConn.AddICECandidate(candidate); err != nil {
		log.Printf("Failed to add ICE candidate: %v", err)
	}
}

// handleWebSocketDisconnect handles disconnect message
func (s *Server) handleWebSocketDisconnect(conn *websocket.Conn, msg map[string]interface{}) {
	deviceSerial, ok := msg["deviceSerial"].(string)
	if !ok {
		return
	}

	s.webrtcManager.RemoveBridge(deviceSerial)

	conn.WriteJSON(map[string]interface{}{
		"type": "disconnected",
	})
}

// handleWebSocketTouch handles touch events
func (s *Server) handleWebSocketTouch(conn *websocket.Conn, msg map[string]interface{}) {
	deviceSerial, ok := msg["deviceSerial"].(string)
	if !ok {
		return
	}

	bridge, exists := s.webrtcManager.GetBridge(deviceSerial)
	if !exists {
		log.Printf("Bridge not found for device %s", deviceSerial)
		return
	}

	bridge.HandleTouchEvent(msg)
}

// handleWebSocketKey handles key events
func (s *Server) handleWebSocketKey(conn *websocket.Conn, msg map[string]interface{}) {
	deviceSerial, ok := msg["deviceSerial"].(string)
	if !ok {
		return
	}

	bridge, exists := s.webrtcManager.GetBridge(deviceSerial)
	if !exists {
		log.Printf("Bridge not found for device %s", deviceSerial)
		return
	}

	bridge.HandleKeyEvent(msg)
}

// handleWebSocketScroll handles scroll events
func (s *Server) handleWebSocketScroll(conn *websocket.Conn, msg map[string]interface{}) {
	deviceSerial, ok := msg["deviceSerial"].(string)
	if !ok {
		return
	}

	bridge, exists := s.webrtcManager.GetBridge(deviceSerial)
	if !exists {
		log.Printf("Bridge not found for device %s", deviceSerial)
		return
	}

	bridge.HandleScrollEvent(msg)
}
