package control

import (
	"log"
)

// TouchHandler handles touch control events
type TouchHandler struct {
	controlService *ControlService
}

// NewTouchHandler creates a new touch handler
func NewTouchHandler(controlService *ControlService) *TouchHandler {
	return &TouchHandler{
		controlService: controlService,
	}
}

// ProcessTouchEvent processes a touch event
func (h *TouchHandler) ProcessTouchEvent(msg map[string]interface{}, deviceSerial string) error {
	action, _ := msg["action"].(string)
	x, _ := msg["x"].(float64)
	y, _ := msg["y"].(float64)
	pressure, _ := msg["pressure"].(float64)
	pointerId, _ := msg["pointerId"].(float64)

	log.Printf("Touch event: device=%s, action=%s, x=%.3f, y=%.3f, pressure=%.2f, pointerId=%.0f",
		deviceSerial, action, x, y, pressure, pointerId)

	// TODO: Implement touch event processing logic
	// This could include:
	// - Input validation
	// - Coordinate transformation
	// - Event queuing
	// - Bridge communication

	return nil
}

// ValidateTouchEvent validates touch event data
func (h *TouchHandler) ValidateTouchEvent(msg map[string]interface{}) error {
	// Validate required fields
	if _, ok := msg["action"].(string); !ok {
		return ErrMissingAction
	}
	if _, ok := msg["x"].(float64); !ok {
		return ErrMissingX
	}
	if _, ok := msg["y"].(float64); !ok {
		return ErrMissingY
	}
	
	// Validate action type
	action, _ := msg["action"].(string)
	if action != "down" && action != "up" && action != "move" {
		return ErrInvalidAction
	}
	
	// Validate coordinates
	x, _ := msg["x"].(float64)
	y, _ := msg["y"].(float64)
	if x < 0 || y < 0 {
		return ErrInvalidCoordinates
	}
	
	return nil
}

// Error definitions
var (
	ErrMissingAction     = &ControlError{Code: "MISSING_ACTION", Message: "Missing action field"}
	ErrMissingX          = &ControlError{Code: "MISSING_X", Message: "Missing x coordinate"}
	ErrMissingY          = &ControlError{Code: "MISSING_Y", Message: "Missing y coordinate"}
	ErrInvalidAction     = &ControlError{Code: "INVALID_ACTION", Message: "Invalid action type"}
	ErrInvalidCoordinates = &ControlError{Code: "INVALID_COORDINATES", Message: "Invalid coordinates"}
)

// ControlError represents a control-related error
type ControlError struct {
	Code    string
	Message string
}

func (e *ControlError) Error() string {
	return e.Message
}
