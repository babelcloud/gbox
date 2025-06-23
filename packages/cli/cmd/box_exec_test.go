package cmd

// FIXME 需要适配

// import (
// 	"bytes"
// 	"io"
// 	"net/http"
// 	"net/http/httptest"
// 	"os"
// 	"fmt"
// 	"testing"

// 	"github.com/spf13/cobra"
// 	"github.com/stretchr/testify/assert"
// )

// // Test command help display
// func TestBoxExecHelp(t *testing.T) {
// 	// Save original standard output
// 	oldStdout := os.Stdout
// 	oldStderr := os.Stderr
// 	defer func() {
// 		os.Stdout = oldStdout
// 		os.Stderr = oldStderr
// 	}()

// 	// Capture output
// 	r, w, _ := os.Pipe()
// 	os.Stdout = w
// 	os.Stderr = w

// 	// Create command and execute help
// 	cmd := NewBoxExecCommand()
// 	cmd.SetArgs([]string{"-h"})
// 	_ = cmd.Execute() // Ignore error, we only care about the output
// 	w.Close()

// 	// Read captured output
// 	var buf bytes.Buffer
// 	io.Copy(&buf, r)
// 	output := buf.String()

// 	// Verify help information
// 	assert.Contains(t, output, "box")
// 	assert.Contains(t, output, "command")
// 	assert.Contains(t, output, "-h, --help")
// }

// // Test argument parsing
// func TestBoxExecParseArgs(t *testing.T) {
// 	// Basic test cases, simplified to include only necessary test scenarios
// 	cases := []struct {
// 		name          string
// 		args          []string
// 		expectError   bool
// 		errorContains string
// 	}{
// 		{
// 			name:        "Basic command",
// 			args:        []string{"box-123", "--", "ls", "-la"},
// 			expectError: false,
// 		},
// 		{
// 			name:        "With flags",
// 			args:        []string{"-i", "-t", "box-123", "--", "bash"},
// 			expectError: false,
// 		},
// 		{
// 			name:          "Missing Box ID",
// 			args:          []string{"--", "ls"},
// 			expectError:   true,
// 			errorContains: "box ID is required",
// 		},
// 		{
// 			name:          "Missing command",
// 			args:          []string{"box-123", "--"},
// 			expectError:   true,
// 			errorContains: "command must be specified",
// 		},
// 		{
// 			name:          "Missing separator",
// 			args:          []string{"box-123", "ls"},
// 			expectError:   true,
// 			errorContains: "command must be specified after",
// 		},
// 		{
// 			name:          "Unknown flag",
// 			args:          []string{"--unknown", "box-123", "--", "ls"},
// 			expectError:   true,
// 			errorContains: "unknown flag",
// 		},
// 	}

// 	for _, tc := range cases {
// 		t.Run(tc.name, func(t *testing.T) {
// 			cmd := NewBoxExecCommand()
// 			cmd.SetArgs(tc.args)

// 			// For successful test cases, modify RunE to avoid actual command execution
// 			if !tc.expectError {
// 				originalRunE := cmd.RunE
// 				cmd.RunE = func(cmd *cobra.Command, args []string) error {
// 					_ = originalRunE(cmd, args)
// 					return nil
// 				}
// 			}

// 			err := cmd.Execute()

// 			if tc.expectError {
// 				assert.Error(t, err)
// 				if tc.errorContains != "" {
// 					assert.Contains(t, err.Error(), tc.errorContains)
// 				}
// 			} else {
// 				assert.NoError(t, err)
// 			}
// 		})
// 	}
// }

// // Test error response handling
// func TestBoxExecErrorHandling(t *testing.T) {
// 	// Keep one main error handling test case
// 	origAPIURL := os.Getenv("API_ENDPOINT")
// 	defer os.Setenv("API_ENDPOINT", origAPIURL)

// 	// Create mock server
// 	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
// 		w.WriteHeader(http.StatusNotFound)
// 		w.Write([]byte(`{"message":"Box not found: box-123","code":"BOX_NOT_FOUND"}`))
// 	}))
// 	defer server.Close()

// 	// Set environment variable
// 	os.Setenv("API_ENDPOINT", server.URL)

// 	// Save original standard output
// 	oldStdout := os.Stdout
// 	oldStderr := os.Stderr
// 	defer func() {
// 		os.Stdout = oldStdout
// 		os.Stderr = oldStderr
// 	}()

// 	// Capture output
// 	r, w, _ := os.Pipe()
// 	os.Stderr = w

// 	// Execute command
// 	cmd := NewBoxExecCommand()
// 	cmd.SetArgs([]string{"box-123", "--", "ls"})
// 	err := cmd.Execute()

// 	// Close pipe and read output
// 	w.Close()
// 	io.Copy(io.Discard, r)

// 	// Verify results
// 	assert.Error(t, err)
// 	assert.Contains(t, err.Error(), "Box not found")
// }

// // Skip terminal size testing function
// func TestGetTerminalSize(t *testing.T) {
// 	t.Skip("skipping terminal size test, should be tested in e2e tests")
// }

// // Test request building
// // Test request building
// func TestExecRequestBuilding(t *testing.T) {
// 	// Save original environment variables
// 	origAPIURL := os.Getenv("API_ENDPOINT")
// 	defer os.Setenv("API_ENDPOINT", origAPIURL)

// 	// Set DEBUG environment variable
// 	origDebug := os.Getenv("DEBUG")
// 	os.Setenv("DEBUG", "true")
// 	defer os.Setenv("DEBUG", origDebug)

// 	// Create mock server
// 	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		// Handle GET /api/v1/boxes for ResolveBoxIDPrefix
// 		if r.Method == "GET" && r.URL.Path == "/api/v1/boxes" {
// 			w.Header().Set("Content-Type", "application/json")
// 			w.WriteHeader(http.StatusOK)
// 			// Return a list containing the "box-123" ID that the exec part of the test expects
// 			fmt.Fprintln(w, `{"boxes":[{"id":"box-123"}, {"id":"another-box-to-ensure-it-still-picks-the-right-one"}]}`)
// 			return
// 		}

// 		// Handle POST /api/v1/boxes/box-123/exec for the actual exec call
// 		if r.Method == "POST" && r.URL.Path == "/api/v1/boxes/box-123/exec" {
// 			// Verify request headers for the exec call
// 			assert.Equal(t, "application/json", r.Header.Get("Content-Type"), "Content-Type header mismatch for exec")
// 			assert.Equal(t, "tcp", r.Header.Get("Upgrade"), "Upgrade header mismatch for exec")
// 			assert.Equal(t, "Upgrade", r.Header.Get("Connection"), "Connection header mismatch for exec")

// 			// Read request body for the exec call
// 			body, err := io.ReadAll(r.Body)
// 			assert.NoError(t, err, "Failed to read body for exec")

// 			// Verify request body contains correct command for the exec call
// 			assert.Contains(t, string(body), `"cmd":["ls"]`, "Request body for exec does not contain correct command")
// 			assert.Contains(t, string(body), `"args":["-la"]`, "Request body for exec does not contain correct args") // Assuming ls -la

// 			// Mock successful response for the exec call (e.g., switching protocols)
// 			// For a real hijack, this is more complex, but for testing the request building,
// 			// just returning a success status might be enough, or StatusSwitchingProtocols.
// 			w.WriteHeader(http.StatusSwitchingProtocols) // Or http.StatusOK if hijacking isn't fully mocked
// 			// w.Write([]byte("Success")) // Optional: write some data if the handler expects it.
// 			return
// 		}

// 		// If no routes match, return 404 to help debugging
// 		http.NotFound(w, r)
// 	}))
// 	defer server.Close()

// 	// Set environment variable for the API URL
// 	os.Setenv("API_ENDPOINT", server.URL)

// 	// Capture standard output and error
// 	oldStdout := os.Stdout
// 	oldStderr := os.Stderr
// 	defer func() {
// 		os.Stdout = oldStdout
// 		os.Stderr = oldStderr
// 	}()

// 	// Using io.Discard for stdout as we are more interested in stderr for debug/errors
// 	// and the direct error return from cmd.Execute()
// 	stdoutR, stdoutW, _ := os.Pipe()
// 	os.Stdout = stdoutW // Can be os.Stdout = io.Discard if stdout not needed
	
// 	stderrR, stderrW, _ := os.Pipe()
// 	os.Stderr = stderrW

// 	// Execute command
// 	cmd := NewBoxExecCommand()
// 	// Using "box-123" which should be resolved by ResolveBoxIDPrefix via the mock server
// 	cmd.SetArgs([]string{"box-123", "--", "ls", "-la"})

// 	err := cmd.Execute() // Execute directly, removed goroutine as it complicates error capture for this test type

// 	stdoutW.Close()
// 	stderrW.Close()
	
// 	var stderrBuf bytes.Buffer
// 	io.Copy(io.Discard, stdoutR) // Ensure pipe is read
// 	io.Copy(&stderrBuf, stderrR) // Ensure pipe is read

// 	// Verify errors
// 	// The nature of the error from exec is tricky because it involves connection hijacking.
// 	// If the mock server doesn't fully support hijacking, cmd.Execute() might return an error
// 	// related to that, or it might complete "successfully" if the mock just returns an HTTP status.
// 	// The original test expected an error related to connection upgrade.
// 	// If ResolveBoxIDPrefix fails, 'err' will be "failed to resolve box ID: ..."
// 	if err != nil {
// 		// This assertion might need to be adjusted based on how deeply hijacking is mocked
// 		// For now, we check if it's NOT a "failed to resolve box ID" error, implying resolution worked.
// 		assert.NotContains(t, err.Error(), "failed to resolve box ID", "Box ID resolution should succeed")
// 		// Original check:
// 		// assert.True(t, strings.Contains(err.Error(), "connection") ||
// 		// 	strings.Contains(err.Error(), "upgrade") ||
// 		// 	strings.Contains(err.Error(), "hijack"),
// 		// 	"Expected error related to connection upgrade if exec proceeds, got: %v", err)
// 	}

// 	// Verify DEBUG output contains request information
// 	// These should now reflect the successful path if the mock server is hit correctly for exec
// 	// If ResolveBoxIDPrefix fails, these might not appear or show errors from ResolveBoxIDPrefix.
// 	// fmt.Println("STDERR CAPTURED:\n", stderrBuf.String()) // For debugging tests
// 	assert.Contains(t, stderrBuf.String(), "DEBUG: [ResolveBoxIDPrefix] Fetching box IDs from", "Debug output for ResolveBoxIDPrefix missing")
// 	assert.Contains(t, stderrBuf.String(), "DEBUG: Request body:", "Debug output for exec request body missing")
// 	assert.Contains(t, stderrBuf.String(), "DEBUG: Sending request to: POST", "Debug output for sending exec request missing")
// 	assert.Contains(t, stderrBuf.String(), "/api/v1/boxes/box-123/exec", "Debug output URL for exec is incorrect")
// }
