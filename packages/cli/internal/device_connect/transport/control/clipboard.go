package control

import (
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/protocol"
	"github.com/babelcloud/gbox/packages/cli/internal/util"
)

// handleClipboardGet handles clipboard get requests
func (h *Handler) handleClipboardGet(message map[string]interface{}) {
	util.GetLogger().Info("Clipboard get requested")
	h.sendClipboardToDevice("", false)
}

// handleClipboardSet handles clipboard set requests
func (h *Handler) handleClipboardSet(message map[string]interface{}) {
	text, ok := message["text"].(string)
	if !ok {
		util.GetLogger().Error("Invalid clipboard text")
		return
	}

	paste, ok := message["paste"].(bool)
	if !ok {
		paste = false
	}

	util.GetLogger().Info("Clipboard set requested", "text", text, "paste", paste)
	h.sendClipboardToDevice(text, paste)
}

// sendClipboardToDevice sends clipboard data to the device
func (h *Handler) sendClipboardToDevice(text string, paste bool) {
	logger := util.GetLogger()
	logger.Debug("Sending clipboard to device", "text", text, "paste", paste)

	// Create control message for setting clipboard
	msg := &protocol.ControlMessage{
		Type: protocol.ControlMsgTypeSetClipboard,
		Data: []byte(text), // Text content as data
	}

	h.sendControlMessage(msg)

	// If paste is requested, also send the text as input
	if paste && text != "" {
		time.Sleep(100 * time.Millisecond) // Small delay
		h.handleInjectText(map[string]interface{}{
			"text": text,
		})
	}
}

// HandleOutgoingMessages handles outgoing control messages (for future use)
func (h *Handler) HandleOutgoingMessages() {
	// This can be used for handling outgoing messages in the future
	// For now, it's a placeholder
}
