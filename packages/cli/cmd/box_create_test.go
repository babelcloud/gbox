 package cmd


 // FIXME cp 实现需要适配
// import (
// 	"bytes"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"net/http"
// 	"net/http/httptest"
// 	"os"
// 	"strings"
// 	"testing"

// 	"github.com/babelcloud/gbox/packages/api-server/pkg/box"
// 	"github.com/stretchr/testify/assert"
// )

// // Fixed test server response format
// const mockResponse = `{"id": "mock-box-id", "status": "stopped", "image": "alpine:latest"}`

// // TestNewBoxCreateCommand tests that NewBoxCreateCommand parses CLI arguments and correctly calls the API
// func TestNewBoxCreateCommand(t *testing.T) {
// 	// Save original stdout for later restoration
// 	oldStdout := os.Stdout
// 	oldStderr := os.Stderr
// 	defer func() {
// 		os.Stdout = oldStdout
// 		os.Stderr = oldStderr
// 	}()

// 	// Create mock server
// 	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		// Check request method
// 		assert.Equal(t, "POST", r.Method)
// 		assert.Equal(t, "/api/v1/boxes", r.URL.Path)

// 		// Read request body
// 		body, err := io.ReadAll(r.Body)
// 		assert.NoError(t, err)
// 		defer r.Body.Close()

// 		// Parse request JSON
// 		var req model.BoxCreateParams
// 		err = json.Unmarshal(body, &req)
// 		assert.NoError(t, err)

// 		// Ensure image field is correctly parsed
// 		assert.Equal(t, "alpine:latest", req.Image)
// 		assert.Equal(t, "/bin/sh", req.Cmd)
// 		assert.Equal(t, []string{"-c", "echo Hello"}, req.Args)
// 		assert.Equal(t, map[string]string{"ENV_VAR": "value"}, req.Env)

// 		// Return mock response
// 		w.Header().Set("Content-Type", "application/json")
// 		w.WriteHeader(http.StatusCreated)
// 		w.Write([]byte(mockResponse))
// 	}))
// 	defer mockServer.Close()

// 	// Save original environment variables
// 	origAPIURL := os.Getenv("API_ENDPOINT")
// 	defer os.Setenv("API_ENDPOINT", origAPIURL)

// 	// Set API URL to mock server
// 	os.Setenv("API_ENDPOINT", mockServer.URL)

// 	// Create pipe to capture stdout
// 	r, w, _ := os.Pipe()
// 	os.Stdout = w
// 	os.Stderr = w

// 	// Execute command
// 	cmd := NewBoxCreateCommand()
// 	cmd.SetArgs([]string{
// 		"--image", "alpine:latest",
// 		"--env", "ENV_VAR=value",
// 		"--", "/bin/sh", "-c", "echo Hello",
// 	})
// 	err := cmd.Execute()
// 	assert.NoError(t, err)

// 	// Read captured output
// 	w.Close()
// 	var buf bytes.Buffer
// 	io.Copy(&buf, r)
// 	output := buf.String()

// 	fmt.Fprintf(oldStdout, "Captured output: %s\n", output)

// 	// Check CLI output
// 	assert.Contains(t, output, "mock-box-id", "CLI should correctly output the returned ID")
// }

// // TestNewBoxCreateCommandWithLabelsAndWorkDir tests the case with labels and working directory
// func TestNewBoxCreateCommandWithLabelsAndWorkDir(t *testing.T) {
// 	// Save original stdout for later restoration
// 	oldStdout := os.Stdout
// 	oldStderr := os.Stderr
// 	defer func() {
// 		os.Stdout = oldStdout
// 		os.Stderr = oldStderr
// 	}()

// 	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		assert.Equal(t, "/api/v1/boxes", r.URL.Path)

// 		body, err := io.ReadAll(r.Body)
// 		assert.NoError(t, err)
// 		defer r.Body.Close()

// 		var req model.BoxCreateParams
// 		err = json.Unmarshal(body, &req)
// 		assert.NoError(t, err)

// 		// Validate labels and working directory
// 		assert.Equal(t, "nginx:latest", req.Image)
// 		assert.Equal(t, "/app", req.WorkingDir)
// 		assert.Equal(t, map[string]string{"app": "web", "env": "test"}, req.ExtraLabels)

// 		w.Header().Set("Content-Type", "application/json")
// 		w.WriteHeader(http.StatusCreated)
// 		w.Write([]byte(mockResponse))
// 	}))
// 	defer mockServer.Close()

// 	// Save original environment variables
// 	origAPIURL := os.Getenv("API_ENDPOINT")
// 	defer os.Setenv("API_ENDPOINT", origAPIURL)

// 	// Set API URL to mock server
// 	os.Setenv("API_ENDPOINT", mockServer.URL)

// 	// Create pipe to capture stdout
// 	r, w, _ := os.Pipe()
// 	os.Stdout = w
// 	os.Stderr = w

// 	// Execute command
// 	cmd := NewBoxCreateCommand()
// 	cmd.SetArgs([]string{
// 		"--image", "nginx:latest",
// 		"--work-dir", "/app",
// 		"--label", "app=web",
// 		"-l", "env=test",
// 		"--", "nginx", "-g", "daemon off;",
// 	})
// 	err := cmd.Execute()
// 	assert.NoError(t, err)

// 	// Read captured output
// 	w.Close()
// 	var buf bytes.Buffer
// 	io.Copy(&buf, r)
// 	output := buf.String()

// 	fmt.Fprintf(oldStdout, "Captured output: %s\n", output)

// 	// Check CLI output
// 	assert.Contains(t, output, "mock-box-id", "CLI should correctly output the returned ID")
// }

// // TestNewBoxCreateCommandWithJSONOutput tests JSON output format
// func TestNewBoxCreateCommandWithJSONOutput(t *testing.T) {
// 	// Save original stdout for later restoration
// 	oldStdout := os.Stdout
// 	oldStderr := os.Stderr
// 	defer func() {
// 		os.Stdout = oldStdout
// 		os.Stderr = oldStderr
// 	}()

// 	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		assert.Equal(t, "/api/v1/boxes", r.URL.Path)

// 		w.Header().Set("Content-Type", "application/json")
// 		w.WriteHeader(http.StatusCreated)
// 		w.Write([]byte(mockResponse))
// 	}))
// 	defer mockServer.Close()

// 	// Save original environment variables
// 	origAPIURL := os.Getenv("API_ENDPOINT")
// 	defer os.Setenv("API_ENDPOINT", origAPIURL)

// 	// Set API URL to mock server
// 	os.Setenv("API_ENDPOINT", mockServer.URL)

// 	// Create pipe to capture stdout
// 	r, w, _ := os.Pipe()
// 	os.Stdout = w
// 	os.Stderr = w

// 	// Execute command
// 	cmd := NewBoxCreateCommand()
// 	cmd.SetArgs([]string{
// 		"--output", "json",
// 		"--image", "alpine:latest",
// 		"--", "/bin/sh", "-c", "echo Hello",
// 	})
// 	err := cmd.Execute()
// 	assert.NoError(t, err)

// 	// Read captured output
// 	w.Close()
// 	var buf bytes.Buffer
// 	io.Copy(&buf, r)
// 	output := buf.String()

// 	fmt.Fprintf(oldStdout, "Captured output: %s\n", output)

// 	// Check CLI output
// 	assert.True(t, strings.Contains(output, `"id": "mock-box-id"`), "JSON output should include mock-box-id")
// }

// // TestNewBoxCreateCommandWithError tests the case where the API returns an error
// func TestNewBoxCreateCommandWithError(t *testing.T) {
// 	// Skip this test, as it requires testing os.Exit behavior
// 	// In Go test environment, os.Exit will directly end the test process
// 	// To correctly test this case, a subprocess would be needed
// 	t.Skip("Need to test os.Exit behavior in a subprocess")
// }

// // Note: If you need to test os.Exit behavior, you can use the following method:
// // 1. Create a test program with special parameters
// // 2. Run the program in a subprocess
// // 3. Check the exit code of the subprocess
// // For example:
// // func TestOsExitBehavior(t *testing.T) {
// //     if os.Getenv("TEST_EXIT") == "1" {
// //         // Place code that will call os.Exit here
// //         return
// //     }
// //     cmd := exec.Command(os.Args[0], "-test.run=TestOsExitBehavior")
// //     cmd.Env = append(os.Environ(), "TEST_EXIT=1")
// //     err := cmd.Run()
// //     if e, ok := err.(*exec.ExitError); ok && !e.Success() {
// //         // Test passes, program exits with non-zero code as expected
// //         return
// //     }
// //     t.Fatalf("Expected process to exit, but it didn't")
// // }

// // TestNewBoxCreateHelp tests the help information
// func TestNewBoxCreateHelp(t *testing.T) {
// 	// Save original stdout for later restoration
// 	oldStdout := os.Stdout
// 	oldStderr := os.Stderr
// 	defer func() {
// 		os.Stdout = oldStdout
// 		os.Stderr = oldStderr
// 	}()

// 	// Create pipe to capture stdout
// 	r, w, _ := os.Pipe()
// 	os.Stdout = w
// 	os.Stderr = w

// 	// Execute command
// 	cmd := NewBoxCreateCommand()
// 	cmd.SetArgs([]string{
// 		"--help",
// 	})
// 	err := cmd.Execute()
// 	assert.NoError(t, err)

// 	// Read captured output
// 	w.Close()
// 	var buf bytes.Buffer
// 	io.Copy(&buf, r)
// 	output := buf.String()

// 	fmt.Fprintf(oldStdout, "Captured output: %s\n", output)

// 	// Check if help information contains key options and usage instructions
// 	assert.Contains(t, output, "Usage:")
// 	assert.Contains(t, output, "--output")
// 	assert.Contains(t, output, "--image")
// 	assert.Contains(t, output, "--env")
// 	assert.Contains(t, output, "--work-dir")
// 	assert.Contains(t, output, "--label")
// }

// // TestNewBoxCreateCommandWithMultipleOptions tests multiple environment variables and labels
// func TestNewBoxCreateCommandWithMultipleOptions(t *testing.T) {
// 	// Save original stdout for later restoration
// 	oldStdout := os.Stdout
// 	oldStderr := os.Stderr
// 	defer func() {
// 		os.Stdout = oldStdout
// 		os.Stderr = oldStderr
// 	}()

// 	// Create mock server
// 	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		// Check request method
// 		assert.Equal(t, "POST", r.Method)
// 		assert.Equal(t, "/api/v1/boxes", r.URL.Path)

// 		// Read request body
// 		body, err := io.ReadAll(r.Body)
// 		assert.NoError(t, err)
// 		defer r.Body.Close()

// 		// Parse request JSON
// 		var req model.BoxCreateParams
// 		err = json.Unmarshal(body, &req)
// 		assert.NoError(t, err)

// 		// Validate multiple options
// 		assert.Equal(t, "python:3.9", req.Image)
// 		assert.Equal(t, "/app", req.WorkingDir)

// 		// Validate multiple environment variables
// 		expectedEnv := map[string]string{
// 			"PATH":     "/usr/local/bin:/usr/bin:/bin",
// 			"DEBUG":    "true",
// 			"NODE_ENV": "production",
// 		}
// 		assert.Equal(t, expectedEnv, req.Env)

// 		// Validate multiple labels
// 		expectedLabels := map[string]string{
// 			"project": "myapp",
// 			"env":     "prod",
// 			"version": "1.0",
// 		}
// 		assert.Equal(t, expectedLabels, req.ExtraLabels)

// 		// Return mock response
// 		w.Header().Set("Content-Type", "application/json")
// 		w.WriteHeader(http.StatusCreated)
// 		w.Write([]byte(mockResponse))
// 	}))
// 	defer mockServer.Close()

// 	// Save original environment variables
// 	origAPIURL := os.Getenv("API_ENDPOINT")
// 	defer os.Setenv("API_ENDPOINT", origAPIURL)

// 	// Set API URL to mock server
// 	os.Setenv("API_ENDPOINT", mockServer.URL)

// 	// Create pipe to capture stdout
// 	r, w, _ := os.Pipe()
// 	os.Stdout = w
// 	os.Stderr = w

// 	// Execute command
// 	cmd := NewBoxCreateCommand()
// 	cmd.SetArgs([]string{
// 		"--image", "python:3.9",
// 		"--work-dir", "/app",
// 		"--env", "PATH=/usr/local/bin:/usr/bin:/bin",
// 		"--env", "DEBUG=true",
// 		"--env", "NODE_ENV=production",
// 		"--label", "project=myapp",
// 		"--label", "env=prod",
// 		"--label", "version=1.0",
// 		"--", "python", "-m", "http.server", "8000",
// 	})
// 	err := cmd.Execute()
// 	assert.NoError(t, err)

// 	// Read captured output
// 	w.Close()
// 	var buf bytes.Buffer
// 	io.Copy(&buf, r)
// 	output := buf.String()

// 	fmt.Fprintf(oldStdout, "Captured output: %s\n", output)

// 	// Check CLI output
// 	assert.Contains(t, output, "mock-box-id", "CLI should correctly output the returned ID")
// }
