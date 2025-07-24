//go:build !windows

package cmd

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
)

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