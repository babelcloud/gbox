package handlers

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/webrtc"
	"github.com/gorilla/websocket"
	pionwebrtc "github.com/pion/webrtc/v4"
)

// WebRTCHandlers handles WebRTC signaling operations
type WebRTCHandlers struct {
	serverService ServerService
	upgrader      websocket.Upgrader
	webrtcManager *webrtc.Manager
}

// NewWebRTCHandlers creates a new WebRTC handlers instance
func NewWebRTCHandlers(serverSvc ServerService) *WebRTCHandlers {
	return &WebRTCHandlers{
		serverService: serverSvc,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for development
			},
		},
		webrtcManager: webrtc.NewManager("adb"), // Use default adb path
	}
}

// HandleWebRTCSignaling handles WebRTC signaling WebSocket connections
func (h *WebRTCHandlers) HandleWebRTCSignaling(conn *websocket.Conn, deviceSerial string) {
	log.Printf("WebRTC signaling connection established for device: %s", deviceSerial)

	// Check and clean up any existing connections for this device
	if existingBridge, exists := h.webrtcManager.GetBridge(deviceSerial); exists {
		if pc := existingBridge.GetPeerConnection(); pc != nil {
			log.Printf("Existing bridge found for device: %s, state: %s", deviceSerial, pc.ConnectionState().String())
		}
	}

	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebRTC signaling read error: %v", err)
			}
			break
		}

		msgType, ok := msg["type"].(string)
		if !ok {
			continue
		}

		log.Printf("WebRTC signaling message received: type=%s, device=%s", msgType, deviceSerial)

		switch msgType {
		case "offer":
			h.HandleOffer(conn, msg, deviceSerial)

		case "answer":
			h.HandleAnswer(conn, msg, deviceSerial)

		case "ice-candidate":
			h.HandleIceCandidate(conn, msg, deviceSerial)

		case "ping":
			h.HandlePing(conn, msg)

		default:
			log.Printf("Unknown WebRTC signaling message type: %s", msgType)
		}
	}
}

// HandleOffer processes WebRTC offer messages
func (h *WebRTCHandlers) HandleOffer(conn *websocket.Conn, msg map[string]interface{}, deviceSerial string) {
	log.Printf("WebRTC offer received: device=%s", deviceSerial)

	// Extract the offer SDP from the message
	var offerSDP string
	if offer, exists := msg["offer"].(map[string]interface{}); exists {
		if sdp, ok := offer["sdp"].(string); ok {
			offerSDP = sdp
		}
	} else if sdp, ok := msg["sdp"].(string); ok {
		offerSDP = sdp
	}

	if offerSDP == "" {
		log.Printf("No valid SDP found in offer message")
		h.sendError(conn, "Invalid offer: missing SDP")
		return
	}

	// Check if existing bridge's peer connection is closed, if so remove it
	if existingBridge, exists := h.webrtcManager.GetBridge(deviceSerial); exists {
		if pc := existingBridge.GetPeerConnection(); pc != nil {
			if pc.ConnectionState() == pionwebrtc.PeerConnectionStateClosed ||
			   pc.ConnectionState() == pionwebrtc.PeerConnectionStateFailed {
				log.Printf("Removing closed WebRTC bridge for device: %s", deviceSerial)
				h.webrtcManager.RemoveBridge(deviceSerial)
				// Add delay to ensure complete ICE cleanup
				time.Sleep(1000 * time.Millisecond)
			}
		}
	}

	// Create or get WebRTC bridge for this device
	bridge, err := h.webrtcManager.CreateBridge(deviceSerial)
	if err != nil {
		log.Printf("Failed to create WebRTC bridge: %v", err)
		h.sendError(conn, fmt.Sprintf("Failed to create bridge: %v", err))
		return
	}

	// Get the peer connection from the bridge
	pc := bridge.GetPeerConnection()
	if pc == nil {
		log.Printf("No peer connection available from bridge")
		h.sendError(conn, "Peer connection not available")
		return
	}

	// Check peer connection state before setting remote description
	if pc.ConnectionState() == pionwebrtc.PeerConnectionStateClosed ||
	   pc.ConnectionState() == pionwebrtc.PeerConnectionStateFailed {
		log.Printf("Peer connection is closed/failed, recreating bridge for device: %s", deviceSerial)
		h.webrtcManager.RemoveBridge(deviceSerial)

		// Create new bridge
		bridge, err = h.webrtcManager.CreateBridge(deviceSerial)
		if err != nil {
			log.Printf("Failed to recreate WebRTC bridge: %v", err)
			h.sendError(conn, fmt.Sprintf("Failed to recreate bridge: %v", err))
			return
		}
		pc = bridge.GetPeerConnection()
		if pc == nil {
			log.Printf("No peer connection available from new bridge")
			h.sendError(conn, "Peer connection not available")
			return
		}
	}

	// Set the remote offer
	offerDesc := pionwebrtc.SessionDescription{
		Type: pionwebrtc.SDPTypeOffer,
		SDP:  offerSDP,
	}

	log.Printf("Setting remote description for device: %s, PC state: %s", deviceSerial, pc.ConnectionState().String())
	// Debug: Offer SDP preview (disabled in production)
	// log.Printf("Offer SDP preview: %.200s...", offerSDP)
	if err := pc.SetRemoteDescription(offerDesc); err != nil {
		log.Printf("Failed to set remote description: %v", err)
		h.sendError(conn, fmt.Sprintf("Failed to set remote description: %v", err))
		return
	}

	// Create answer
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		log.Printf("Failed to create answer: %v", err)
		h.sendError(conn, fmt.Sprintf("Failed to create answer: %v", err))
		return
	}

	// Set local description
	if err := pc.SetLocalDescription(answer); err != nil {
		log.Printf("Failed to set local description: %v", err)
		h.sendError(conn, fmt.Sprintf("Failed to set local description: %v", err))
		return
	}

	// Debug: Answer SDP preview (disabled in production)
	// log.Printf("Answer SDP preview: %.200s...", answer.SDP)

	// Send answer back to client (using old format)
	answerResponse := map[string]interface{}{
		"type": "answer",
		"answer": map[string]interface{}{
			"type": "answer",
			"sdp":  answer.SDP,
		},
	}

	if err := conn.WriteJSON(answerResponse); err != nil {
		log.Printf("Failed to send WebRTC answer: %v", err)
		return
	}

	log.Printf("WebRTC answer sent successfully for device: %s, PC state: %s", deviceSerial, pc.ConnectionState().String())

	// Set up ICE candidate forwarding AFTER sending answer (like old implementation)
	pc.OnICECandidate(func(candidate *pionwebrtc.ICECandidate) {
		if candidate == nil {
			log.Printf("ICE candidate gathering finished for device: %s", deviceSerial)
			return
		}

		// Debug: ICE candidate forwarding (disabled in production)
		// log.Printf("Forwarding server ICE candidate to client for device: %s", deviceSerial)

		// Use ToJSON() like old implementation
		candidateJSON := candidate.ToJSON()
		candidateMessage := map[string]interface{}{
			"type": "ice-candidate",
			"candidate": map[string]interface{}{
				"candidate":     candidateJSON.Candidate,
				"sdpMLineIndex": candidateJSON.SDPMLineIndex,
				"sdpMid":        candidateJSON.SDPMid,
			},
		}

		if err := conn.WriteJSON(candidateMessage); err != nil {
			log.Printf("Failed to send ICE candidate to client: %v", err)
		}
	})

	// Add timeout monitoring for connection establishment
	go func() {
		time.Sleep(10 * time.Second)
		if pc.ConnectionState() != pionwebrtc.PeerConnectionStateConnected {
			log.Printf("WebRTC connection timeout for device: %s, current state: %s", deviceSerial, pc.ConnectionState().String())
		}
	}()
}

// HandleAnswer processes WebRTC answer messages
func (h *WebRTCHandlers) HandleAnswer(conn *websocket.Conn, msg map[string]interface{}, deviceSerial string) {
	log.Printf("WebRTC answer received: device=%s", deviceSerial)

	// TODO: Process WebRTC answer from client
	// This would typically be sent to the media server for processing
	log.Printf("WebRTC answer processing not yet implemented")
}

// HandleIceCandidate processes WebRTC ICE candidate messages
func (h *WebRTCHandlers) HandleIceCandidate(conn *websocket.Conn, msg map[string]interface{}, deviceSerial string) {
	log.Printf("WebRTC ICE candidate received: device=%s", deviceSerial)

	// Get WebRTC bridge for this device
	bridge, exists := h.webrtcManager.GetBridge(deviceSerial)
	if !exists {
		log.Printf("No WebRTC bridge found for device: %s", deviceSerial)
		h.sendError(conn, "No bridge found for device")
		return
	}

	// Get the peer connection
	pc := bridge.GetPeerConnection()
	if pc == nil {
		log.Printf("No peer connection available from bridge")
		h.sendError(conn, "Peer connection not available")
		return
	}

	// Extract ICE candidate from message
	candidateData, ok := msg["candidate"].(map[string]interface{})
	if !ok {
		log.Printf("Invalid ICE candidate format")
		h.sendError(conn, "Invalid ICE candidate format")
		return
	}

	candidateStr, ok := candidateData["candidate"].(string)
	if !ok {
		log.Printf("Missing candidate string")
		h.sendError(conn, "Missing candidate string")
		return
	}

	sdpMid, ok := candidateData["sdpMid"].(string)
	if !ok {
		log.Printf("Missing sdpMid")
		h.sendError(conn, "Missing sdpMid")
		return
	}

	sdpMLineIndex, ok := candidateData["sdpMLineIndex"].(float64)
	if !ok {
		log.Printf("Missing sdpMLineIndex")
		h.sendError(conn, "Missing sdpMLineIndex")
		return
	}

	// Create ICE candidate
	candidate := pionwebrtc.ICECandidateInit{
		Candidate:     candidateStr,
		SDPMid:        &sdpMid,
		SDPMLineIndex: (*uint16)(&[]uint16{uint16(sdpMLineIndex)}[0]),
	}

	// Add ICE candidate to peer connection
	log.Printf("Adding ICE candidate for device: %s, candidate: %.50s...", deviceSerial, candidateStr)
	if err := pc.AddICECandidate(candidate); err != nil {
		log.Printf("Failed to add ICE candidate: %v", err)
		h.sendError(conn, fmt.Sprintf("Failed to add ICE candidate: %v", err))
		return
	}

	// Debug: ICE candidate added (disabled in production)
	// log.Printf("ICE candidate added successfully for device: %s", deviceSerial)
}

// handlePing handles ping messages for latency measurement
func (h *WebRTCHandlers) HandlePing(conn *websocket.Conn, msg map[string]interface{}) {
	pongMsg := map[string]interface{}{
		"type": "pong",
	}
	// Pass through the ping ID if present for latency calculation
	if id, exists := msg["id"]; exists {
		pongMsg["id"] = id
	}
	conn.WriteJSON(pongMsg)
}

// sendError sends an error message to the client
func (h *WebRTCHandlers) sendError(conn *websocket.Conn, errorMsg string) {
	conn.WriteJSON(map[string]interface{}{
		"type":  "error",
		"error": errorMsg,
	})
}

