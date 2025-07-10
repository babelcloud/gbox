package model_test

import (
	"testing"
	"time"

	"github.com/babelcloud/gbox/packages/api-server/pkg/box"
	"github.com/stretchr/testify/assert"
)

func TestBox(t *testing.T) {
	now := time.Now()

	box := model.Box{
		ID: "test-box",
		Config: model.LinuxAndroidBoxConfig{
			Browser: model.LinuxAndroidBoxConfigBrowser{
				Type:    "chrome",
				Version: "100",
			},
			CPU:   2,
			Envs:  map[string]string{"foo": "bar"},
			Labels: map[string]string{"env": "test"},
			Memory: 4096,
			Os: model.LinuxAndroidBoxConfigOs{
				Version: "ubuntu-20.04",
			},
			Resolution: model.LinuxAndroidBoxConfigResolution{
				Width:  1920,
				Height: 1080,
			},
			Storage:    20,
			WorkingDir: "/home/user",
		},
		CreatedAt: now,
		Status:    "running",
		UpdatedAt: now,
		ExpiresAt: now.Add(time.Hour),
		Type:      model.BoxTypeLinux,
	}

	assert.Equal(t, "test-box", box.ID)
	assert.Equal(t, "chrome", box.Config.Browser.Type)
	assert.Equal(t, "100", box.Config.Browser.Version)
	assert.Equal(t, float64(2), box.Config.CPU)
	assert.Equal(t, "bar", box.Config.Envs["foo"])
	assert.Equal(t, "test", box.Config.Labels["env"])
	assert.Equal(t, float64(4096), box.Config.Memory)
	assert.Equal(t, "ubuntu-20.04", box.Config.Os.Version)
	assert.Equal(t, 1920, box.Config.Resolution.Width)
	assert.Equal(t, 1080, box.Config.Resolution.Height)
	assert.Equal(t, float64(20), box.Config.Storage)
	assert.Equal(t, "/home/user", box.Config.WorkingDir)
	assert.Equal(t, now, box.CreatedAt)
	assert.Equal(t, "running", box.Status)
	assert.Equal(t, now, box.UpdatedAt)
	assert.Equal(t, now.Add(time.Hour), box.ExpiresAt)
	assert.Equal(t, model.BoxTypeLinux, box.Type)
}

func TestBoxType(t *testing.T) {
	assert.Equal(t, model.BoxType("linux"), model.BoxTypeLinux)
	assert.Equal(t, model.BoxType("android"), model.BoxTypeAndroid)
}

func TestBoxFile(t *testing.T) {
	now := time.Now()

	file := model.BoxFile{
		LastModified: now,
		Name:         "test.txt",
		Path:         "/home/user/test.txt",
		Size:         "1024",
		Type:         "text/plain",
	}

	assert.Equal(t, now, file.LastModified)
	assert.Equal(t, "test.txt", file.Name)
	assert.Equal(t, "/home/user/test.txt", file.Path)
	assert.Equal(t, "1024", file.Size)
	assert.Equal(t, "text/plain", file.Type)
}

func TestBoxFileListParams(t *testing.T) {
	params := model.BoxFileListParams{
		Path:  "/home/user",
		Depth: 2,
	}

	assert.Equal(t, "/home/user", params.Path)
	assert.Equal(t, float64(2), params.Depth)
}

func TestBoxFileListResult(t *testing.T) {
	now := time.Now()

	result := model.BoxFileListResult{
		Data: []model.BoxFile{
			{
				LastModified: now,
				Name:         "test.txt",
				Path:         "/home/user/test.txt",
				Size:         "1024",
				Type:         "text/plain",
			},
		},
	}

	assert.Len(t, result.Data, 1)
	assert.Equal(t, now, result.Data[0].LastModified)
	assert.Equal(t, "test.txt", result.Data[0].Name)
	assert.Equal(t, "/home/user/test.txt", result.Data[0].Path)
	assert.Equal(t, "1024", result.Data[0].Size)
	assert.Equal(t, "text/plain", result.Data[0].Type)
}

func TestBoxFileReadParams(t *testing.T) {
	params := model.BoxFileReadParams{
		Path: "/home/user/test.txt",
	}

	assert.Equal(t, "/home/user/test.txt", params.Path)
}

func TestBoxFileReadResult(t *testing.T) {
	result := model.BoxFileReadResult{
		Content: "Hello World",
	}

	assert.Equal(t, "Hello World", result.Content)
}

func TestBoxFileWriteParams(t *testing.T) {
	params := model.BoxFileWriteParams{
		Path:    "/home/user/test.txt",
		Content: "Hello World",
	}

	assert.Equal(t, "/home/user/test.txt", params.Path)
	assert.Equal(t, "Hello World", params.Content)
}

func TestBoxFileWriteResult(t *testing.T) {
	result := model.BoxFileWriteResult{
		Message: "File written successfully",
	}

	assert.Equal(t, "File written successfully", result.Message)
}
