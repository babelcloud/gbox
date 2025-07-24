//go:build windows

package cmd

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// handleRawStream for Windows does not handle SIGWINCH.
// It sets the terminal to raw mode and copies data between the connection and stdio.
func handleRawStream(conn io.ReadWriteCloser) error {
	// Save terminal state
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set terminal to raw mode: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

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