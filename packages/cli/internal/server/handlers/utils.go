package handlers

import (
	"encoding/json"
	"net/http"
)

// RespondJSON sends a JSON response with the given status code and data
func RespondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func isValidDeviceSerial(serial string) bool {
	if serial == "" {
		return false
	}

	// Basic validation - should be alphanumeric with possible special chars
	if len(serial) < 3 || len(serial) > 64 {
		return false
	}

	// Allow alphanumeric, dots, dashes, underscores
	for _, c := range serial {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '.' || c == '-' || c == '_') {
			return false
		}
	}

	return true
}