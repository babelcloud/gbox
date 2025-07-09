package model

// ProgressStatus defines the type for progress statuses.
// These are used in ProgressUpdate messages.
type ProgressStatus string

const (
	// ProgressStatusPrepare indicates that an operation is being prepared.
	ProgressStatusPrepare ProgressStatus = "prepare"
	// ProgressStatusComplete indicates that an operation has completed successfully.
	ProgressStatusComplete ProgressStatus = "complete"
	// ProgressStatusError indicates that an error occurred during an operation.
	ProgressStatusError ProgressStatus = "error"
)

// ProgressUpdate represents a generic progress update message streamed to the client.
type ProgressUpdate struct {
	Status  ProgressStatus `json:"status"`            // Status of the current operation
	Message string         `json:"message,omitempty"` // Human-readable message describing the progress
	Error   string         `json:"error,omitempty"`   // Error message, if an error occurred (used when Status is ProgressStatusError)
	ImageID string         `json:"imageId,omitempty"` // Image ID, if relevant (e.g., after a successful image pull)
}

// LinuxAndroidBoxCreateParam represents parameters for creating Linux or Android boxes
// This struct is used inline in BoxCreateParams to support SDK format
type LinuxAndroidBoxCreateParam struct {
	Type    string               `json:"type"`              // Type of box to create (linux, android)
	Wait    bool                 `json:"wait,omitempty"`    // Wait for the box operation to complete
	Config  CreateBoxConfigParam `json:"config"`            // Box configuration
}

// CreateBoxConfigParam represents the configuration for a box
type CreateBoxConfigParam struct {
	ExpiresIn string            `json:"expiresIn"` // Box expiration duration (e.g., "1000s")
	Envs      map[string]string `json:"envs"`      // Environment variables
	Labels    map[string]string `json:"labels"`    // Key-value labels
}

// Legacy types - kept for backwards compatibility but deprecated
type AndroidBoxCreateParam struct {
	CreateAndroidBox LinuxAndroidBoxCreateParam
}

type LinuxBoxCreateParam struct {
	CreateLinuxBox LinuxAndroidBoxCreateParam
}

// VolumeMount represents a volume mount configuration
type VolumeMount struct {
	Source      string `json:"source"`      // Host path
	Target      string `json:"target"`      // Container path
	ReadOnly    bool   `json:"read_only"`   // Whether the mount is read-only
	Propagation string `json:"propagation"` // Mount propagation (private, rprivate, shared, rshared, slave, rslave)
}

// BoxCreateResult represents the response from creating a box
type BoxCreateResult struct {
	Box     Box    `json:"box"`
	Message string `json:"message,omitempty"`
}

// BoxDeleteParams represents a request to delete a box
type BoxDeleteParams struct {
	Force bool `json:"force,omitempty"` // Whether to force delete the box
}

// BoxDeleteResult represents a response from deleting a box
type BoxDeleteResult struct {
	Message string `json:"message"`
}

// BoxesDeleteParams represents a request to delete multiple boxes
type BoxesDeleteParams struct {
	Force bool `json:"force,omitempty"` // Whether to force delete the boxes
}

// BoxesDeleteResult represents a response from deleting multiple boxes
type BoxesDeleteResult struct {
	Count   int      `json:"count"`         // Number of boxes deleted
	Message string   `json:"message"`       // Response message
	IDs     []string `json:"ids,omitempty"` // IDs of deleted boxes
}

// BoxStartResult represents a response from starting a box.
// Returns the complete box information after starting.
type BoxStartResult = Box

// BoxStopResult represents a response from stopping a box.
// Returns the complete box information after stopping.
type BoxStopResult = Box

// BoxReclaimResult represents a response from reclaiming boxes
type BoxReclaimResult struct {
	StoppedCount int      `json:"stopped_count"`         // Number of boxes stopped
	DeletedCount int      `json:"deleted_count"`         // Number of boxes deleted
	StoppedIDs   []string `json:"stopped_ids,omitempty"` // IDs of stopped boxes
	DeletedIDs   []string `json:"deleted_ids,omitempty"` // IDs of deleted boxes
}
