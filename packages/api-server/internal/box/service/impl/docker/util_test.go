package docker_test

import (
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
			args:     []string{"test1", "test2"},
			expected: `["test1","test2"]`,
		},
		{
			name:     "args with spaces",
			args:     []string{"test arg1", "test arg2"},
			expected: `["test arg1","test arg2"]`,
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
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.ElementsMatch(t, tt.expected, result)
			}
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
			image:    "ubuntu:20.04",
			expected: "ubuntu:20.04",
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
			image:    "ubuntu:20.04",
			expected: "ubuntu:20.04",
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

func TestMapToEnv(t *testing.T) {
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
			result := docker.MapToEnv(tt.env)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.ElementsMatch(t, tt.expected, result)
			}
		})
	}
}

func TestWaitForResponse(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		expected    string
	}{
		{
			name:        "empty input",
			input:       "",
			expectError: false,
			expected:    "",
		},
		{
			name:        "valid json response",
			input:       `{"status":"Success","progress":"100%"}`,
			expectError: false,
			expected:    "Success\n",
		},
		{
			name:        "error response",
			input:       `{"error":"Failed"}`,
			expectError: true,
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			result, err := docker.WaitForResponse(reader)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, string(result))
			}
		})
	}
}

func TestProcessPullProgress(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "empty input",
			input:       "",
			expectError: false,
		},
		{
			name:        "valid progress",
			input:       `{"status":"Downloading","progressDetail":{},"progress":"100%"}`,
			expectError: false,
		},
		{
			name:        "error response",
			input:       `{"error":"Failed to pull image"}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			writer := &strings.Builder{}
			err := docker.ProcessPullProgress(reader, writer)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPrepareLabels(t *testing.T) {
	tests := []struct {
		name     string
		boxID    string
		params   *model.BoxCreateParams
		expected map[string]string
	}{
		{
			name:  "basic labels",
			boxID: "test-box",
			params: &model.BoxCreateParams{
				Cmd:        "test-cmd",
				Args:       []string{"arg1", "arg2"},
				WorkingDir: "/test",
				ExtraLabels: map[string]string{
					"custom": "value",
				},
			},
			expected: map[string]string{
				"gbox.id":         "test-box",
				"gbox.cmd":        "test-cmd",
				"gbox.args":       `["arg1","arg2"]`,
				"gbox.working-dir": "/test",
				"gbox.extra.custom": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := docker.PrepareLabels(tt.boxID, tt.params)
			for k, v := range tt.expected {
				assert.Equal(t, v, result[k])
			}
		})
	}
}
