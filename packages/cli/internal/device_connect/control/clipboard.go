package control

import (
	"log"
)

// ClipboardHandler handles clipboard control events
type ClipboardHandler struct {
	controlService *ControlService
}

// NewClipboardHandler creates a new clipboard handler
func NewClipboardHandler(controlService *ControlService) *ClipboardHandler {
	return &ClipboardHandler{
		controlService: controlService,
	}
}

// ProcessClipboardEvent processes a clipboard event
func (h *ClipboardHandler) ProcessClipboardEvent(msg map[string]interface{}, deviceSerial string) error {
	text, _ := msg["text"].(string)
	paste, _ := msg["paste"].(bool)

	log.Printf("Clipboard event: device=%s, text_length=%d, paste=%t",
		deviceSerial, len(text), paste)

	// TODO: Implement clipboard event processing logic
	// This could include:
	// - Text validation
	// - Clipboard state management
	// - Event queuing
	// - Bridge communication

	return nil
}

// ValidateClipboardEvent validates clipboard event data
func (h *ClipboardHandler) ValidateClipboardEvent(msg map[string]interface{}) error {
	// Validate required fields
	if _, ok := msg["text"].(string); !ok {
		return ErrMissingText
	}
	if _, ok := msg["paste"].(bool); !ok {
		return ErrMissingPaste
	}
	
	// Validate text length
	text, _ := msg["text"].(string)
	if len(text) > 10000 { // Reasonable limit
		return ErrTextTooLong
	}
	
	return nil
}

// Error definitions
var (
	ErrMissingText   = &ControlError{Code: "MISSING_TEXT", Message: "Missing text field"}
	ErrMissingPaste  = &ControlError{Code: "MISSING_PASTE", Message: "Missing paste field"}
	ErrTextTooLong   = &ControlError{Code: "TEXT_TOO_LONG", Message: "Text too long"}
)
