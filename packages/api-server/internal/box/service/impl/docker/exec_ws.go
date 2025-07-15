package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/babelcloud/gbox/packages/api-server/internal/common"
	model "github.com/babelcloud/gbox/packages/api-server/pkg/box"
	"github.com/docker/docker/api/types"
	"github.com/gorilla/websocket"
)

// ExecWS implements Service.ExecWS for WebSocket connections
func (s *Service) ExecWS(ctx context.Context, id string, params *model.BoxExecWSParams, wsConn *websocket.Conn) (*model.BoxExecResult, error) {
	s.accessTracker.Update(id)

	containerInfo, err := s.getContainerByID(ctx, id)
	if err != nil {
		return nil, err // Includes ErrBoxNotFound
	}

	// Check container status, if not running, start it
	if containerInfo.State != "running" {
		fmt.Printf("Before exec, container(%s) is not running, starting it...\n", id)
		if _, err := s.Start(ctx, id); err != nil {
			return nil, fmt.Errorf("failed to start container: %w", err)
		}
		containerInfo, err = s.getContainerByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to update container: %w", err)
		}
	}

	// Create exec configuration
	execConfig := types.ExecConfig{
		User:         "", // Use default user
		Privileged:   false,
		Tty:          params.TTY,
		AttachStdin:  true, // Always attach stdin for WebSocket
		AttachStdout: true,
		AttachStderr: true,
		Detach:       false,
		DetachKeys:   "",                // Use default detach keys
		Env:          nil,               // No additional environment variables for now
		WorkingDir:   params.WorkingDir, // Use provided or default below
		Cmd:          append(params.Cmd, params.Args...),
	}

	// Use default working directory if not specified
	if execConfig.WorkingDir == "" {
		execConfig.WorkingDir = common.DefaultWorkDirPath
	}

	// Create exec instance
	execResp, err := s.client.ContainerExecCreate(ctx, containerInfo.ID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec: %w", err)
	}

	// Attach to exec instance
	attachResp, err := s.client.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{
		Detach: false,
		Tty:    params.TTY,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer attachResp.Close() // Close the underlying hijack connection when done

	errChan := make(chan error, 2) // Channel for errors from goroutines
	copyDone := make(chan bool, 2) // Track completion of copy routines

	var mu sync.Mutex

	// Goroutine: Read from Docker stdout/stderr -> Write to WebSocket
	go func() {
		defer func() {
			copyDone <- true
			s.logger.Debugf("ExecWS [%s]: Docker output stream ended. Goroutine finished.", id)
		}()
		var writeErr error
		// Always copy raw stream directly to websocket using wsWriter helper.
		// This sends the raw Docker stream (TTY or multiplexed with headers) as binary messages.
		_, writeErr = io.Copy(&wsWriter{conn: wsConn}, attachResp.Reader)

		if writeErr != nil && !isConnectionClosed(writeErr) {
			s.logger.Errorf("ExecWS [%s]: Error writing to WebSocket: %v", id, writeErr)
			errChan <- fmt.Errorf("websocket write error: %w", writeErr)
		}
	}()

	// Goroutine: Read from WebSocket -> Write to Docker stdin
	go func() {
		defer func() {
			copyDone <- true
			// Close the write side of the Docker attach connection to signal EOF to stdin
			// This defer ensures CloseWrite is called even if the loop exits due to WebSocket close/error before receiving stdin_eof
			if closeWriter, ok := attachResp.Conn.(interface{ CloseWrite() error }); ok {
				s.logger.Debugf("ExecWS [%s]: Closing docker stdin pipe in defer", id) // Modified log
				if err := closeWriter.CloseWrite(); err != nil && !isConnectionClosed(err) {
					s.logger.Warnf("ExecWS [%s]: Error closing docker stdin pipe in defer: %v", id, err)
				}
			} else {
				s.logger.Warnf("ExecWS [%s]: Could not get CloseWrite() for docker stdin pipe in defer", id)
			}
			s.logger.Debugf("ExecWS [%s]: WebSocket input processing goroutine finished.", id) // Added log
		}()
		for {
			messageType, message, err := wsConn.ReadMessage()
			if err != nil {
				// Check for clean WebSocket closure first
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					mu.Lock()
					s.logger.Infof("ExecWS [%s]: WebSocket connection closed cleanly by client.", id)
					mu.Unlock()
				} else if errors.Is(err, net.ErrClosed) {
					s.logger.Warnf("ExecWS [%s]: Underlying network connection closed (expected during shutdown): %v", id, err)
				} else {
					s.logger.Errorf("ExecWS [%s]: Unexpected error reading from WebSocket: %v", id, err)
					errChan <- fmt.Errorf("websocket read error: %w", err)
				}
				return // Exit goroutine on any error or close
			}

			switch messageType {
			case websocket.TextMessage:
				// Check for special stdin EOF message
				var controlMsg map[string]string
				if json.Unmarshal(message, &controlMsg) == nil && controlMsg["type"] == "stdin_eof" {
					s.logger.Debugf("ExecWS [%s]: Received stdin_eof message from client. Closing container stdin now.", id)
					// Close the container's stdin pipe immediately
					if closeWriter, ok := attachResp.Conn.(interface{ CloseWrite() error }); ok {
						if err := closeWriter.CloseWrite(); err != nil && !isConnectionClosed(err) {
							s.logger.Warnf("ExecWS [%s]: Error closing docker stdin pipe after EOF message: %v", id, err)
							// Potentially send error back? For now, just log.
						} else {
							s.logger.Debugf("ExecWS [%s]: Successfully closed docker stdin pipe after EOF message.", id)
						}
					} else {
						s.logger.Warnf("ExecWS [%s]: Could not get CloseWrite() for docker stdin pipe after EOF message", id)
					}
					// DO NOT return here. Continue listening on the WebSocket for potential close frame or errors.
					// The defer statement will handle the final CloseWrite if needed.
				} else {
					s.logger.Warnf("ExecWS [%s]: Received unexpected text message, ignoring: %s", id, string(message))
					// Ignore other text messages for now, as stdin data should be binary.
				}

			case websocket.BinaryMessage:
				// Write binary messages directly to container stdin
				_, writeErr := attachResp.Conn.Write(message)
				if writeErr != nil {
					if isConnectionClosed(writeErr) {
						s.logger.Debugf("ExecWS [%s]: Docker stdin pipe closed while writing binary data (might be expected after EOF or container exit).", id)
						// Pipe closed, likely due to container exit or previous EOF close. Exit goroutine.
						return
					}
					s.logger.Errorf("ExecWS [%s]: Error writing binary data to docker stdin: %v", id, writeErr)
					// Send error only if it's not a closed pipe error
					if !errors.Is(writeErr, net.ErrClosed) && !strings.Contains(writeErr.Error(), "use of closed network connection") {
						errChan <- fmt.Errorf("docker stdin write error: %w", writeErr)
					}
					return // Exit goroutine if write fails significantly
				}

			case websocket.CloseMessage:
				s.logger.Infof("ExecWS [%s]: Received WebSocket close message from client.", id)
				return // Exit goroutine

			default:
				s.logger.Warnf("ExecWS [%s]: Ignored WebSocket message type: %d", id, messageType)
			}
		}
	}()

	// Wait primarily for the Docker output goroutine to finish, or an error/cancellation.
	var firstError error
loop:
	for {
		select {
		case err := <-errChan:
			if err != nil && firstError == nil {
				firstError = err
				s.logger.Errorf("ExecWS [%s]: Goroutine finished with error: %v", id, err)
				// Log the error, but wait for copyDone signals (as appropriate for TTY/non-TTY)
			}
		case <-copyDone:
			s.logger.Debugf("ExecWS [%s]: A copy goroutine finished. Proceeding to cleanup.", id)
			break loop // Exit loop on the first completion signal (should be Docker output stream ending)
		case <-ctx.Done():
			if firstError == nil { // Record context error only if no other error occurred
				firstError = ctx.Err()
			}
			s.logger.Errorf("ExecWS [%s]: Context cancelled: %v", id, ctx.Err())
			break loop // Exit loop on context cancellation
		}
	}

	s.logger.Infof("ExecWS [%s]: Docker output finished or error occurred. Proceeding to inspect exit code. First error: %v", id, firstError)

	// Get exit code (use a separate context for inspect)
	inspectCtx, cancelInspect := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelInspect()
	inspectResp, inspectErr := s.client.ContainerExecInspect(inspectCtx, execResp.ID)
	if inspectErr != nil {
		s.logger.Errorf("ExecWS [%s]: Failed to inspect exec after completion: %v", id, inspectErr)
		// If we already had an I/O error, prioritize that
		if firstError == nil {
			firstError = fmt.Errorf("failed to inspect exec: %w", inspectErr)
		}
	}

	// Determine final result and error
	exitCode := -1
	if inspectErr == nil {
		exitCode = inspectResp.ExitCode
	}
	s.logger.Infof("ExecWS [%s]: Command finished. Exit Code: %d. Final Error recorded: %v", id, exitCode, firstError)

	// 2. Attempt to send a WebSocket close frame
	s.logger.Debugf("ExecWS [%s]: Attempting to send WebSocket close frame from server side.", id)
	// Ignore error here, as the connection might already be closing or closed.
	_ = wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Command finished"))
	// 3. The deferred wsConn.Close() at the function start will handle final cleanup.

	if firstError != nil {
		expectedReadErr := fmt.Sprintf("websocket read error: %s", websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Command finished"))
		clientNormalCloseErr := "websocket read error: websocket: close 1000 (normal)"

		if firstError.Error() == expectedReadErr || firstError.Error() == clientNormalCloseErr {
			s.logger.Infof("ExecWS [%s]: Suppressing expected WebSocket close error after command finished.", id)
			if inspectErr == nil {
				return &model.BoxExecResult{ExitCode: exitCode}, nil
			}
			return nil, fmt.Errorf("failed to inspect exec after command finished: %w", inspectErr)
		}
		return nil, firstError
	}

	// Return the result if no major errors occurred during I/O
	return &model.BoxExecResult{
		ExitCode: exitCode,
	}, nil
}

// wsWriter is a helper to wrap a websocket.Conn to satisfy io.Writer for io.Copy
type wsWriter struct {
	conn *websocket.Conn
}

func (w *wsWriter) Write(p []byte) (n int, err error) {
	// Use BinaryMessage for raw TTY stream or framed stream data
	err = w.conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}
