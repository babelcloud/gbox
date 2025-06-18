package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/spf13/cobra"
)

// BoxPath represents the structure of a box path
type BoxPath struct {
	BoxID string
	Path  string
}

// Parse box path (format BOX_ID:PATH)
func parseBoxPath(path string) (*BoxPath, error) {
	re := regexp.MustCompile(`^([^:]+):(.+)$`)
	matches := re.FindStringSubmatch(path)
	if len(matches) != 3 {
		return nil, fmt.Errorf("invalid box path format, should be BOX_ID:PATH")
	}
	return &BoxPath{
		BoxID: matches[1],
		Path:  matches[2],
	}, nil
}

// Check if path is a box path
func isBoxPath(path string) bool {
	return strings.Contains(path, ":")
}

// Convert relative path to absolute path
func getAbsolutePath(path string) string {
	if _, err := os.Stat(path); err == nil {
		absPath, err := filepath.Abs(path)
		if err == nil {
			return absPath
		}
	}

	dir := filepath.Dir(path)
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return path
	}

	return filepath.Join(absDir, filepath.Base(path))
}

// BoxCpOptions holds command options and parameters
type BoxCpOptions struct {
	Source      string
	Destination string
}

func NewBoxCpCommand() *cobra.Command {
	opts := &BoxCpOptions{}

	cmd := &cobra.Command{
		Use:   "cp <src> <dst>",
		Short: "Copy files/folders between a box and the local filesystem",
		Long: `usage: gbox-box-cp [-h] src dst

Copy files/folders between a box and the local filesystem

positional arguments:
  src                Source path
  dst                Destination path

options:
  -h, --help         show this help message and exit`,
		Example: `    gbox box cp ./local_file 550e8400-e29b-41d4-a716-446655440000:/work     # Copy local file to box
    gbox box cp 550e8400-e29b-41d4-a716-446655440000:/var/logs/ /tmp/app_logs     # Copy from box to local
    gbox box cp - 550e8400-e29b-41d4-a716-446655440000:/work     # Copy tar stream from stdin to box
    gbox box cp 550e8400-e29b-41d4-a716-446655440000:/etc/hosts -     # Copy from box to stdout as tar stream`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Source = args[0]
			opts.Destination = args[1]
			return runCopyCommand(opts)
		},
	}

	return cmd
}

func runCopyCommand(opts *BoxCpOptions) error {
	src := opts.Source
	dst := opts.Destination
	debugEnabled := os.Getenv("DEBUG") == "true"
	apiBase := config.GetLocalAPIURL()
	apiURL := fmt.Sprintf("%s/api/v1", strings.TrimSuffix(apiBase, "/"))

	// Debug log
	debug := func(msg string) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "[DEBUG] %s\n", msg)
		}
	}

	// Determine copy direction and process
	if isBoxPath(src) && !isBoxPath(dst) {
		return copyFromBoxToLocal(src, dst, apiURL, debug)
	} else if !isBoxPath(src) && isBoxPath(dst) {
		return copyFromLocalToBox(src, dst, apiURL, debug)
	} else {
		return fmt.Errorf("invalid path format. One path must be a box path (BOX_ID:PATH) and the other must be a local path")
	}
}

func copyFromBoxToLocal(src, dst, apiURL string, debug func(string)) error {
	boxPath, err := parseBoxPath(src)
	if err != nil {
		return err
	}

	debug(fmt.Sprintf("Box ID: %s", boxPath.BoxID))
	debug(fmt.Sprintf("Source path: %s", boxPath.Path))
	debug(fmt.Sprintf("Destination path: %s", dst))

	if dst == "-" {
		// Copy from box to stdout as tar stream
		return copyFromBoxToStdout(boxPath, apiURL, debug)
	} else {
		// Copy from box to local file
		return copyFromBoxToFile(boxPath, dst, apiURL, debug)
	}
}

func copyFromBoxToStdout(boxPath *BoxPath, apiURL string, debug func(string)) error {
	requestURL := fmt.Sprintf("%s/boxes/%s/archive?path=%s", apiURL, boxPath.BoxID, boxPath.Path)
	debug(fmt.Sprintf("Sending GET request to: %s", requestURL))

	resp, err := http.Get(requestURL)
	if err != nil {
		return fmt.Errorf("failed to download from box: %v", err)
	}
	defer resp.Body.Close()

	debug(fmt.Sprintf("HTTP response status code: %d", resp.StatusCode))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download from box, HTTP status code: %d", resp.StatusCode)
	}

	_, err = io.Copy(os.Stdout, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write to stdout: %v", err)
	}

	return nil
}

func copyFromBoxToFile(boxPath *BoxPath, dst, apiURL string, debug func(string)) error {
	// Convert local path to absolute path
	dst = getAbsolutePath(dst)

	// Create destination directory if it doesn't exist
	err := os.MkdirAll(filepath.Dir(dst), 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination directory: %v", err)
	}

	// Download to temporary file
	tempFile, err := os.CreateTemp("", "gbox-cp-")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %v", err)
	}
	tempFilePath := tempFile.Name()
	defer os.Remove(tempFilePath)

	requestURL := fmt.Sprintf("%s/boxes/%s/archive?path=%s", apiURL, boxPath.BoxID, boxPath.Path)
	debug(fmt.Sprintf("Sending GET request to: %s", requestURL))

	resp, err := http.Get(requestURL)
	if err != nil {
		return fmt.Errorf("failed to download from box: %v", err)
	}
	defer resp.Body.Close()

	debug(fmt.Sprintf("HTTP response status code: %d", resp.StatusCode))

	if resp.StatusCode != http.StatusOK {
		// It's helpful to read the body even on error for more details
		bodyBytes, _ := io.ReadAll(resp.Body)
		debug(fmt.Sprintf("Error response body: %s", string(bodyBytes)))
		return fmt.Errorf("failed to download from box, HTTP status code: %d", resp.StatusCode)
	}

	bytesCopied, copyErr := io.Copy(tempFile, resp.Body)
	debug(fmt.Sprintf("Bytes copied to temporary file: %d", bytesCopied))

	// Ensure file is closed regardless of copy errors
	defer tempFile.Close() // Close should happen after potential errors are checked

	// Handle errors after attempting copy and close
	if copyErr != nil {
		// Check if the error is specifically UnexpectedEOF, likely from the reader (resp.Body)
		if copyErr == io.ErrUnexpectedEOF {
			return fmt.Errorf("failed to download complete file from box (unexpected EOF): %v", copyErr)
		}
		// Otherwise, report it as a failure to write to the temp file
		return fmt.Errorf("failed to write to temporary file: %v", copyErr)
	}

	// Check format and extract
	dstDir := filepath.Dir(dst)
	srcBaseName := filepath.Base(boxPath.Path)
	dstBaseName := filepath.Base(dst)

	// Try to extract as gzip tar
	cmd := exec.Command("tar", "-xzf", tempFilePath, "-C", dstDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try to extract as regular tar
		cmd = exec.Command("tar", "-xf", tempFilePath, "-C", dstDir)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to extract archive: %v\nOutput:\n%s", err, string(output))
		}
	}

	// After extraction, check if the destination path `dst` was intended to be a file or directory.
	// If `dst` does not end with a separator, assume it was a file.
	extractedPath := filepath.Join(dstDir, srcBaseName) // Default expected path after extraction
	finalDstPath := dst

	// Smarter check: Did the tar command create the exact dstBaseName in dstDir?
	// Or did it potentially create srcBaseName or a path structure?
	// Let's check if the srcBaseName exists first
	if _, statErr := os.Stat(extractedPath); statErr == nil {
		// If the original dst path didn't end with '/' and is different from the extracted path,
		// it implies the user wanted to copy INTO a file named dstBaseName.
		if !strings.HasSuffix(dst, string(os.PathSeparator)) && dstBaseName != srcBaseName {
			if renameErr := os.Rename(extractedPath, finalDstPath); renameErr != nil {
				return fmt.Errorf("failed to rename extracted file to destination: %v", renameErr)
			}
		} else if dstBaseName == srcBaseName {
			// If dst and src base names match, extraction likely overwrote/created the correct file/dir.
		} else {
			// Destination was likely a directory, files extracted inside.
			finalDstPath = extractedPath // The message should report the actual extracted path
		}
	} else {
		// If srcBaseName doesn't exist directly, maybe tar extracted with full path?
		// This case is harder to handle reliably without knowing archive structure.
		// We will assume for now the extraction target was dstDir and tar placed files inside.
		// The user might need to check dstDir content.
		// We can try to list files extracted by tar output parsing, but it's complex.
		// We cannot be sure what the final path is, report the directory
		finalDstPath = dstDir
	}

	fmt.Fprintf(os.Stderr, "Copied from box %s:%s to %s\n", boxPath.BoxID, boxPath.Path, finalDstPath)
	return nil
}

func copyFromLocalToBox(src, dst, apiURL string, debug func(string)) error {
	boxPath, err := parseBoxPath(dst)
	if err != nil {
		return err
	}

	if src == "-" {
		// Copy tar stream from stdin to box
		return copyFromStdinToBox(boxPath, apiURL, debug)
	} else {
		// Copy from local file to box
		return copyFromFileToBox(src, boxPath, apiURL, debug)
	}
}

func copyFromStdinToBox(boxPath *BoxPath, apiURL string, debug func(string)) error {
	requestURL := fmt.Sprintf("%s/boxes/%s/archive?path=%s", apiURL, boxPath.BoxID, boxPath.Path)
	debug(fmt.Sprintf("Sending PUT request to: %s", requestURL))

	req, err := http.NewRequest("PUT", requestURL, os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-tar")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload to box: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to upload to box, HTTP status code: %d", resp.StatusCode)
	}

	fmt.Fprintf(os.Stderr, "Copied from stdin to box %s:%s\n", boxPath.BoxID, boxPath.Path)
	return nil
}

func copyFromFileToBox(src string, boxPath *BoxPath, apiURL string, _ func(string)) error {
	// Convert local path to absolute path
	src = getAbsolutePath(src)

	// Check if source file exists
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("source file or directory does not exist: %s", src)
	}

	// Create temporary file for the tar
	tempFile, err := os.CreateTemp("", "gbox-cp-")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %v", err)
	}
	tempFilePath := tempFile.Name()
	tempFile.Close()
	defer os.Remove(tempFilePath)

	// Create tar archive
	cmd := exec.Command("tar", "--no-xattrs", "-czf", tempFilePath, "-C", filepath.Dir(src), filepath.Base(src))
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to create tar archive: %v", err)
	}

	// Get file size
	fileInfo, err := os.Stat(tempFilePath)
	if err != nil {
		return fmt.Errorf("failed to get temporary file info: %v", err)
	}
	fileSize := fileInfo.Size()

	// Upload archive to box
	file, err := os.Open(tempFilePath)
	if err != nil {
		return fmt.Errorf("failed to open temporary file: %v", err)
	}
	defer file.Close()

	requestURL := fmt.Sprintf("%s/boxes/%s/archive?path=%s", apiURL, boxPath.BoxID, boxPath.Path)

	req, err := http.NewRequest("PUT", requestURL, file)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-tar")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", fileSize))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload to box: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to upload to box, HTTP status code: %d", resp.StatusCode)
	}

	fmt.Fprintf(os.Stderr, "Copied from %s to box %s:%s\n", src, boxPath.BoxID, boxPath.Path)
	return nil
}
