package handlers

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Helper function to validate device serial format
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
