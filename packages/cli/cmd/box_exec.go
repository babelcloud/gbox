package cmd

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// BoxExecOptions holds command options
type BoxExecOptions struct {
	Interactive bool
	Tty         bool
	BoxID       string
	Command     []string
}

// BoxExecRequest represents the request to execute a command in a box
type BoxExecRequest struct {
	Cmd      []string          `json:"cmd"`
	Args     []string          `json:"args,omitempty"`
	Stdin    bool              `json:"stdin"`
	Stdout   bool              `json:"stdout"`
	Stderr   bool              `json:"stderr"`
	Tty      bool              `json:"tty"`
	TermSize map[string]int    `json:"term_size,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
	WorkDir  string            `json:"workdir,omitempty"`
}

// TerminalSize represents terminal dimensions
type TerminalSize struct {
	Height int
	Width  int
}

// GetTerminalSize returns the current terminal dimensions
func GetTerminalSize() (*TerminalSize, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, fmt.Errorf("not a terminal")
	}

	width, height, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		return nil, err
	}

	return &TerminalSize{
		Height: height,
		Width:  width,
	}, nil
}

// NewBoxExecCommand creates a new box exec command
func NewBoxExecCommand() *cobra.Command {
	opts := &BoxExecOptions{}

	cmd := &cobra.Command{
		Use:   "exec [box-id] -- [command] [args...]",
		Short: "Execute a command in a box",
		Long: `usage: gbox-box-exec [-h] [-i] [-t] box_id

Execute a command in a box

positional arguments:
  box_id             ID of the box

options:
  -h, --help         show this help message and exit
  -i, --interactive  Enable interactive mode (with stdin)
  -t, --tty          Force TTY allocation`,
		Example: `    gbox box exec 550e8400-e29b-41d4-a716-446655440000 -- ls -l     # List files in box
    gbox box exec 550e8400-e29b-41d4-a716-446655440000 -t -- bash     # Run interactive bash
    gbox box exec 550e8400-e29b-41d4-a716-446655440000 -i -- cat       # Run cat with stdin`,
		RunE: func(cmd *cobra.Command, args []string) error {
			argsLenAtDash := cmd.ArgsLenAtDash()
			if argsLenAtDash == -1 {
				return fmt.Errorf("command must be specified after '--'")
			}

			if len(args) == 0 || argsLenAtDash == 0 {
				cmd.Help()
				return fmt.Errorf("box ID is required")
			}

			// Get box ID (first argument before --)
			opts.BoxID = args[0]

			// Get command (all arguments after --)
			if argsLenAtDash >= len(args) {
				return fmt.Errorf("command must be specified after '--'")
			}
			opts.Command = args[argsLenAtDash:]

			// Run the command
			return runExec(opts)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			// Only complete the first argument (box-id)
			if len(args) == 0 {
				return completeBoxIDs(cmd, args, toComplete)
			}
			// No completion for subsequent arguments before -- or anything after --
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	}

	// Add flags
	cmd.Flags().BoolVarP(&opts.Interactive, "interactive", "i", false, "Enable interactive mode (with stdin)")
	cmd.Flags().BoolVarP(&opts.Tty, "tty", "t", false, "Force TTY allocation")

	return cmd
}

// runExec implements the exec command functionality
func runExec(opts *BoxExecOptions) error {
	// Resolve box ID prefix from opts.BoxID
	resolvedBoxID, _, err := ResolveBoxIDPrefix(opts.BoxID)
	if err != nil {
		return fmt.Errorf("failed to resolve box ID: %w", err)
	}
	// Update opts.BoxID to the fully resolved ID for subsequent use if needed,
	// though for this function, we will primarily use resolvedBoxID directly.
	// opts.BoxID = resolvedBoxID // Optional: update opts if it's used elsewhere by reference

	debug := os.Getenv("DEBUG") == "true"
	apiBase := config.GetLocalAPIURL()
	apiURL := fmt.Sprintf("%s/api/v1", strings.TrimSuffix(apiBase, "/"))

	debugLog := func(msg string) {
		if debug {
			fmt.Fprintf(os.Stderr, "DEBUG: %s\n", msg)
		}
	}

	stdinAvailable := opts.Interactive
	if !stdinAvailable && !term.IsTerminal(int(os.Stdin.Fd())) {
		stdinAvailable = true
	}

	var termSize *TerminalSize
	// var err error // err is already declared by ResolveBoxIDPrefix, reuse or shadow
	if opts.Tty {
		termSize, err = GetTerminalSize() // This might shadow the err from ResolveBoxIDPrefix if not careful
		if err != nil {
			debugLog(fmt.Sprintf("Failed to get terminal size: %v", err))
			// Decide if this is a fatal error. Original code just logs it.
		} else if termSize != nil { // Added nil check for termSize
			debugLog(fmt.Sprintf("Terminal size: height=%d, width=%d", termSize.Height, termSize.Width))
		}
	}

	request := BoxExecRequest{
		Cmd:    []string{opts.Command[0]},
		Args:   opts.Command[1:],
		Stdin:  stdinAvailable,
		Stdout: true,
		Stderr: true,
		Tty:    opts.Tty,
	}

	if opts.Tty {
		request.Stdin = true
	}

	if termSize != nil {
		request.TermSize = map[string]int{
			"height": termSize.Height,
			"width":  termSize.Width,
		}
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to encode request: %v", err)
	}

	debugLog(fmt.Sprintf("Request body: %s", string(requestBody)))

	// Use resolvedBoxID for the API call
	requestURL := fmt.Sprintf("%s/boxes/%s/exec", apiURL, resolvedBoxID)
	req, err := http.NewRequest("POST", requestURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Upgrade", "tcp")
	req.Header.Set("Connection", "Upgrade")

	if opts.Tty {
		req.Header.Set("Accept", "application/vnd.gbox.raw-stream")
	} else {
		req.Header.Set("Accept", "application/vnd.gbox.multiplexed-stream")
	}

	debugLog(fmt.Sprintf("Sending request to: POST %s", requestURL))
	for k, v := range req.Header {
		debugLog(fmt.Sprintf("Header %s: %s", k, v))
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	debugLog(fmt.Sprintf("Response status: %d", resp.StatusCode))
	for k, v := range resp.Header {
		debugLog(fmt.Sprintf("Response header %s: %s", k, v))
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusSwitchingProtocols {
		var errMsg string
		body, _ := io.ReadAll(resp.Body)
		debugLog(fmt.Sprintf("Response body: %s", string(body)))

		var errorData map[string]interface{}
		if err := json.Unmarshal(body, &errorData); err == nil {
			if message, ok := errorData["message"].(string); ok {
				errMsg = message
			}
		}

		if errMsg == "" {
			errMsg = string(body)
		}

		if resp.StatusCode == http.StatusConflict {
			// Use resolvedBoxID in the error message
			return fmt.Errorf("%s (status %d). Maybe run 'gbox box start %s'?", errMsg, resp.StatusCode, resolvedBoxID)
		} else {
			return fmt.Errorf("%s (status %d)", errMsg, resp.StatusCode)
		}
	}

	hijacker, ok := resp.Body.(io.ReadWriteCloser)
	if !ok {
		return fmt.Errorf("response does not support hijacking")
	}

	if opts.Tty {
		return handleRawStream(hijacker)
	} else {
		return handleMultiplexedStream(hijacker, stdinAvailable)
	}
}

// handleRawStream handles raw stream in TTY mode
func handleRawStream(conn io.ReadWriteCloser) error {
	// Save terminal state
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set terminal to raw mode: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Handle terminal resize
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)
	defer signal.Stop(sigChan)

	// Start goroutine for reading from connection
	errChan := make(chan error, 2)
	go func() {
		_, err := io.Copy(os.Stdout, conn)
		errChan <- err
	}()

	// Start goroutine for writing to connection
	go func() {
		_, err := io.Copy(conn, os.Stdin)
		errChan <- err
	}()

	// Wait for either an error or for both goroutines to finish
	err = <-errChan
	if err != nil && err != io.EOF {
		return fmt.Errorf("stream error: %v", err)
	}

	return nil
}

// handleMultiplexedStream handles multiplexed stream in non-TTY mode
// Accepts io.ReadWriteCloser as that's what resp.Body gives us
func handleMultiplexedStream(conn io.ReadWriteCloser, stdinAvailable bool) error {
	var wg sync.WaitGroup
	doneChan := make(chan struct{}) // Channel to signal stdin goroutine to exit

	// Start goroutine for handling stdin if available
	if stdinAvailable {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				// Try to close the write side... (signal EOF)
				if tcpConn, ok := conn.(*net.TCPConn); ok {
					tcpConn.CloseWrite()
				} else {
					conn.Close() // Fallback
				}
			}()

			buffer := make([]byte, 1024)
			for {
				// Use select to read from stdin or wait for done signal
				stdinReadChan := make(chan struct {
					n   int
					err error
				}, 1)

				// Goroutine to perform the potentially blocking read
				go func() {
					n, err := os.Stdin.Read(buffer)
					stdinReadChan <- struct {
						n   int
						err error
					}{n, err}
				}()

				select {
				case <-doneChan: // If main loop signals done
					// fmt.Fprintln(os.Stderr, "DEBUG: stdin goroutine received done signal")
					return // Exit goroutine
				case readResult := <-stdinReadChan: // If stdin read completes
					n := readResult.n
					err := readResult.err
					if n > 0 {
						if _, writeErr := conn.Write(buffer[:n]); writeErr != nil {
							fmt.Fprintf(os.Stderr, "Error writing to connection: %v\n", writeErr)
							return // Exit goroutine on write error
						}
					}
					if err != nil {
						if err != io.EOF {
							fmt.Fprintf(os.Stderr, "Error reading from stdin: %v\n", err)
						}
						// fmt.Fprintf(os.Stderr, "DEBUG: Exiting stdin goroutine due to err: %v\n", err)
						return // Exit goroutine on error or EOF
					}
					// Optional: Add a timeout to prevent deadlock if stdin somehow hangs unexpectedly
					// case <-time.After(10 * time.Second):
					//     fmt.Fprintln(os.Stderr, "DEBUG: stdin read timed out")
					//     return
				}
			}
		}()
	}

	// Read multiplexed stream from connection
	var readErr error
	buf := make([]byte, 8)
	for {
		n, err := io.ReadFull(conn, buf)
		if err != nil {
			isClosedConnErr := errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), "use of closed network connection")
			if err == io.EOF || isClosedConnErr {
				readErr = nil
			} else {
				readErr = fmt.Errorf("failed to read header: %v", err)
			}
			break // Exit loop
		}
		if n != 8 {
			readErr = fmt.Errorf("short read on header: got %d bytes, expected 8", n)
			break
		}

		// Parse header
		streamType := buf[0]
		size := binary.BigEndian.Uint32(buf[4:])

		// Read payload
		if size > 1*1024*1024 {
			readErr = fmt.Errorf("unreasonable payload size received: %d", size)
			break
		}
		if size == 0 {
			continue
		}
		payload := make([]byte, size)
		payloadN, payloadErr := io.ReadFull(conn, payload)
		if payloadErr != nil {
			isClosedConnErr := errors.Is(payloadErr, net.ErrClosed) || strings.Contains(payloadErr.Error(), "use of closed network connection")
			if payloadErr == io.EOF || isClosedConnErr {
				readErr = nil
			} else {
				readErr = fmt.Errorf("failed to read payload: %v", payloadErr)
			}
			break
		}
		if uint32(payloadN) != size {
			readErr = fmt.Errorf("short read on payload: got %d bytes, expected %d", payloadN, size)
			break
		}
		switch streamType {
		case 1:
			os.Stdout.Write(payload[:payloadN])
		case 2:
			os.Stderr.Write(payload[:payloadN])
		default:
			fmt.Fprintf(os.Stderr, "Unknown stream type: %d\n", streamType)
		}
	}

	// Signal stdin goroutine to exit *before* waiting
	// fmt.Fprintln(os.Stderr, "DEBUG: Closing doneChan")
	close(doneChan)

	// Wait for the stdin goroutine to finish
	// fmt.Fprintln(os.Stderr, "DEBUG: Waiting for stdin goroutine (wg.Wait())")
	wg.Wait()
	// fmt.Fprintln(os.Stderr, "DEBUG: Stdin goroutine finished")

	return readErr
}
