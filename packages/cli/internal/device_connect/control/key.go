package control

import (
	"log"
)

// KeyHandler handles keyboard control events
type KeyHandler struct {
	controlService *ControlService
}

// NewKeyHandler creates a new key handler
func NewKeyHandler(controlService *ControlService) *KeyHandler {
	return &KeyHandler{
		controlService: controlService,
	}
}

// ProcessKeyEvent processes a key event
func (h *KeyHandler) ProcessKeyEvent(msg map[string]interface{}, deviceSerial string) error {
	action, _ := msg["action"].(string)
	keycode, _ := msg["keycode"].(float64)
	metaState, _ := msg["metaState"].(float64)

	log.Printf("Key event: device=%s, action=%s, keycode=%.0f, metaState=%.0f",
		deviceSerial, action, keycode, metaState)

	// TODO: Implement key event processing logic
	// This could include:
	// - Key code validation
	// - Meta state processing
	// - Event queuing
	// - Bridge communication

	return nil
}

// ValidateKeyEvent validates key event data
func (h *KeyHandler) ValidateKeyEvent(msg map[string]interface{}) error {
	// Validate required fields
	if _, ok := msg["action"].(string); !ok {
		return ErrMissingAction
	}
	if _, ok := msg["keycode"].(float64); !ok {
		return ErrMissingKeycode
	}
	
	// Validate action type
	action, _ := msg["action"].(string)
	if action != "down" && action != "up" {
		return ErrInvalidAction
	}
	
	// Validate keycode
	keycode, _ := msg["keycode"].(float64)
	if keycode < 0 || keycode > 255 {
		return ErrInvalidKeycode
	}
	
	return nil
}

// Error definitions
var (
	ErrMissingKeycode  = &ControlError{Code: "MISSING_KEYCODE", Message: "Missing keycode field"}
	ErrInvalidKeycode  = &ControlError{Code: "INVALID_KEYCODE", Message: "Invalid keycode value"}
)
