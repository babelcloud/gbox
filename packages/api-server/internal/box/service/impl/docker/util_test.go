package docker_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/babelcloud/gbox/packages/api-server/internal/box/service/impl/docker"
	model "github.com/babelcloud/gbox/packages/api-server/pkg/box"
	"github.com/stretchr/testify/assert"
)

func TestJoinArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "empty args",
			args:     []string{},
			expected: "",
		},
		{
			name:     "single arg",
			args:     []string{"test"},
			expected: `["test"]`,
		},
		{
			name:     "multiple args",
			args:     []string{"test1", "test2", "test3"},
			expected: `["test1","test2","test3"]`,
		},
		{
			name:     "args with spaces",
			args:     []string{"hello world", "test arg"},
			expected: `["hello world","test arg"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := docker.JoinArgs(tt.args)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetCommand(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		args     []string
		expected []string
	}{
		{
			name:     "empty command",
			cmd:      "",
			args:     nil,
			expected: []string{"sleep", "infinity"},
		},
		{
			name:     "command without args",
			cmd:      "test",
			args:     nil,
			expected: []string{"/bin/sh", "-c", "test"},
		},
		{
			name:     "command with args",
			cmd:      "test",
			args:     []string{"arg1", "arg2"},
			expected: []string{"test", "arg1", "arg2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := docker.GetCommand(tt.cmd, tt.args)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		expected []string
	}{
		{
			name:     "nil env",
			env:      nil,
			expected: nil,
		},
		{
			name:     "empty env",
			env:      map[string]string{},
			expected: []string{},
		},
		{
			name: "single env var",
			env: map[string]string{
				"KEY": "value",
			},
			expected: []string{"KEY=value"},
		},
		{
			name: "multiple env vars",
			env: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			expected: []string{"KEY1=value1", "KEY2=value2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := docker.GetEnvVars(tt.env)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestEnsureImageTag(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{
			name:     "empty image",
			image:    "",
			expected: "",
		},
		{
			name:     "image with tag",
			image:    "ubuntu:18.04",
			expected: "ubuntu:18.04",
		},
		{
			name:     "image without tag",
			image:    "ubuntu",
			expected: "ubuntu:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := docker.EnsureImageTag(tt.image)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProcessPullProgress(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErr     bool
		expectedErr string
	}{
		{
			name:    "valid progress",
			input:   `{"status":"Downloading","progressDetail":{},"id":"layer1"}`,
			wantErr: false,
		},
		{
			name:        "error in progress",
			input:       `{"error":"failed to pull"}`,
			wantErr:     true,
			expectedErr: "failed to pull",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			writer := &bytes.Buffer{}

			err := docker.ProcessPullProgress(reader, writer)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != "" {
					assert.Contains(t, err.Error(), tt.expectedErr)
				}
			} else {
				assert.NoError(t, err)
				var output map[string]interface{}
				err = json.Unmarshal(writer.Bytes(), &output)
				assert.NoError(t, err)
			}
		})
	}
}

func TestWaitForResponse(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErr     bool
		expectedErr string
	}{
		{
			name:    "success response",
			input:   `{"status":"Success"}`,
			wantErr: false,
		},
		{
			name:        "error response",
			input:       `{"error":"operation failed"}`,
			wantErr:     true,
			expectedErr: "operation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			result, err := docker.WaitForResponse(reader)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != "" {
					assert.Contains(t, err.Error(), tt.expectedErr)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestPrepareLabels(t *testing.T) {
	boxID := "test-box-id"
	params := &model.BoxCreateParams{
		Cmd:        "test-cmd",
		Args:       []string{"arg1", "arg2"},
		WorkingDir: "/test/dir",
		ExtraLabels: map[string]string{
			"custom.label": "value",
		},
	}

	labels := docker.PrepareLabels(boxID, params)

	assert.Equal(t, boxID, labels["gbox.id"])
	assert.Equal(t, "test-cmd", labels["gbox.cmd"])
	assert.Equal(t, `["arg1","arg2"]`, labels["gbox.args"])
	assert.Equal(t, "/test/dir", labels["gbox.working-dir"])
	assert.Equal(t, "value", labels["gbox.extra.custom.label"])
}

func TestMapToEnv(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		expected []string
	}{
		{
			name:     "nil map",
			env:      nil,
			expected: nil,
		},
		{
			name:     "empty map",
			env:      map[string]string{},
			expected: []string{},
		},
		{
			name: "single entry",
			env: map[string]string{
				"KEY": "value",
			},
			expected: []string{"KEY=value"},
		},
		{
			name: "multiple entries",
			env: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			expected: []string{"KEY1=value1", "KEY2=value2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := docker.MapToEnv(tt.env)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestGetImage(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{
			name:     "empty image",
			image:    "",
			expected: "babelcloud/gbox-playwright:latest",
		},
		{
			name:     "image with tag",
			image:    "ubuntu:18.04",
			expected: "ubuntu:18.04",
		},
		{
			name:     "image without tag",
			image:    "ubuntu",
			expected: "ubuntu:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := docker.GetImage(tt.image)
			assert.Equal(t, tt.expected, result)
		})
	}
}
