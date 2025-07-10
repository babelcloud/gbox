package model_test

import (
	"testing"

	model "github.com/babelcloud/gbox/packages/api-server/pkg/box"
	"github.com/stretchr/testify/assert"
)

func TestProgressStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   model.ProgressStatus
		expected string
	}{
		{
			name:     "prepare status",
			status:   model.ProgressStatusPrepare,
			expected: "prepare",
		},
		{
			name:     "complete status",
			status:   model.ProgressStatusComplete,
			expected: "complete",
		},
		{
			name:     "error status",
			status:   model.ProgressStatusError,
			expected: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.status))
		})
	}
}

func TestProgressUpdate(t *testing.T) {
	tests := []struct {
		name     string
		update   model.ProgressUpdate
		expected model.ProgressUpdate
	}{
		{
			name: "complete progress update",
			update: model.ProgressUpdate{
				Status:  model.ProgressStatusComplete,
				Message: "Operation completed",
				ImageID: "sha256:abc123",
			},
			expected: model.ProgressUpdate{
				Status:  model.ProgressStatusComplete,
				Message: "Operation completed",
				ImageID: "sha256:abc123",
			},
		},
		{
			name: "error progress update",
			update: model.ProgressUpdate{
				Status:  model.ProgressStatusError,
				Message: "Operation failed",
				Error:   "timeout error",
			},
			expected: model.ProgressUpdate{
				Status:  model.ProgressStatusError,
				Message: "Operation failed",
				Error:   "timeout error",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.update)
		})
	}
}

func TestLinuxAndroidBoxCreateParam(t *testing.T) {
	param := model.LinuxAndroidBoxCreateParam{
		Type: "linux",
		Wait: true,
		Config: model.CreateBoxConfigParam{
			ExpiresIn: "1000s",
			Envs: map[string]string{
				"KEY": "value",
			},
			Labels: map[string]string{
				"app": "test",
			},
		},
	}

	assert.Equal(t, "linux", param.Type)
	assert.True(t, param.Wait)
	assert.Equal(t, "1000s", param.Config.ExpiresIn)
	assert.Equal(t, "value", param.Config.Envs["KEY"])
	assert.Equal(t, "test", param.Config.Labels["app"])
}

func TestVolumeMount(t *testing.T) {
	mount := model.VolumeMount{
		Source:      "/host/path",
		Target:      "/container/path",
		ReadOnly:    true,
		Propagation: "rprivate",
	}

	assert.Equal(t, "/host/path", mount.Source)
	assert.Equal(t, "/container/path", mount.Target)
	assert.True(t, mount.ReadOnly)
	assert.Equal(t, "rprivate", mount.Propagation)
}

func TestBoxResults(t *testing.T) {
	t.Run("BoxCreateResult", func(t *testing.T) {
		result := model.BoxCreateResult{
			Box:     model.Box{},
			Message: "Box created successfully",
		}
		assert.Equal(t, "Box created successfully", result.Message)
	})

	t.Run("BoxDeleteResult", func(t *testing.T) {
		result := model.BoxDeleteResult{
			Message: "Box deleted successfully",
		}
		assert.Equal(t, "Box deleted successfully", result.Message)
	})

	t.Run("BoxesDeleteResult", func(t *testing.T) {
		result := model.BoxesDeleteResult{
			Count:   2,
			Message: "Boxes deleted successfully",
			IDs:     []string{"box1", "box2"},
		}
		assert.Equal(t, 2, result.Count)
		assert.Equal(t, "Boxes deleted successfully", result.Message)
		assert.Equal(t, []string{"box1", "box2"}, result.IDs)
	})

	t.Run("BoxReclaimResult", func(t *testing.T) {
		result := model.BoxReclaimResult{
			StoppedCount: 1,
			DeletedCount: 1,
			StoppedIDs:   []string{"box1"},
			DeletedIDs:   []string{"box2"},
		}
		assert.Equal(t, 1, result.StoppedCount)
		assert.Equal(t, 1, result.DeletedCount)
		assert.Equal(t, []string{"box1"}, result.StoppedIDs)
		assert.Equal(t, []string{"box2"}, result.DeletedIDs)
	})
}
