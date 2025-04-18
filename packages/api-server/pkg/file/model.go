package model

// FileType represents the type of a file
type FileType string

const (
	FileTypeDirectory FileType = "directory"
	FileTypeFile      FileType = "file"
	FileTypeSymlink   FileType = "symlink"
	FileTypeSocket    FileType = "socket"
	FileTypePipe      FileType = "pipe"
	FileTypeDevice    FileType = "device"
)

// FileStat represents file metadata
type FileStat struct {
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	Size    int64    `json:"size"`
	Mode    string   `json:"mode"`
	ModTime string   `json:"modTime"`
	Type    FileType `json:"type"`
	Mime    string   `json:"mime"`
}

// FileError represents a file operation error response
type FileError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// FileOperationParams represents a request to share a file from a box
type FileOperationParams struct {
	BoxID   string `json:"boxId"`   // ID of the box to share from
	Path    string `json:"path"`    // Path to the file in the box
	Content string `json:"content"` // Content to write to the file
	Operation string `json:"operation"` // Operation to perform (share, write, reclaim)
}

// FileShareResult represents the response for file sharing operations
type FileShareResult struct {
	Success  bool       `json:"success"`
	Message  string     `json:"message"`
	FileList []FileStat `json:"fileList"`
}
