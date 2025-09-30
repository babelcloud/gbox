package handlers

import (
	"log"
	"net/http"

	client "github.com/babelcloud/gbox/packages/cli/internal/client"
)

// BoxHandlers handles box-related operations (proxy to remote GBOX API)
type BoxHandlers struct {
	serverService ServerService
}

// NewBoxHandlers creates a new box handlers instance
func NewBoxHandlers(serverSvc ServerService) *BoxHandlers {
	return &BoxHandlers{
		serverService: serverSvc,
	}
}

// HandleBoxList handles /api/boxes endpoint - proxy to remote GBOX API
func (h *BoxHandlers) HandleBoxList(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	query := req.URL.Query()
	typeFilter := query.Get("type") // e.g., ?type=android

	// Create GBOX client from profile
	sdkClient, err := client.NewClientFromProfile()
	if err != nil {
		log.Printf("Failed to create GBOX client: %v", err)
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to initialize GBOX client",
		})
		return
	}

	// Call GBOX API to get real box list
	boxesData, err := client.ListBoxesRawData(sdkClient, []string{})
	if err != nil {
		log.Printf("Failed to list boxes from GBOX API: %v", err)
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to fetch boxes from GBOX API",
		})
		return
	}

	// Convert to the expected format and add name field
	var allBoxes []map[string]interface{}
	for _, box := range boxesData {
		// Add name field if not present (use ID as fallback)
		if _, ok := box["name"]; !ok {
			if id, ok := box["id"].(string); ok {
				box["name"] = id
			}
		}
		allBoxes = append(allBoxes, box)
	}

	// Filter boxes by type if specified
	var filteredBoxes []map[string]interface{}
	if typeFilter != "" {
		for _, box := range allBoxes {
			if boxType, ok := box["type"].(string); ok && boxType == typeFilter {
				filteredBoxes = append(filteredBoxes, box)
			}
		}
	} else {
		filteredBoxes = allBoxes
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"boxes": filteredBoxes,
		"filter": map[string]interface{}{
			"type": typeFilter,
		},
	})
}