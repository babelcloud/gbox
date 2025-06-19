package model_test

import (
	"testing"

	model "github.com/babelcloud/gbox/packages/api-server/pkg/browser"
	"github.com/stretchr/testify/assert"
)

func TestNewCdpURLResult(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected model.CdpURLResult
	}{
		{
			name: "Creates result with URL",
			url:  "ws://localhost:9222/devtools/browser/abc123",
			expected: model.CdpURLResult{
				URL: "ws://localhost:9222/devtools/browser/abc123",
			},
		},
		{
			name: "Creates result with empty URL",
			url:  "",
			expected: model.CdpURLResult{
				URL: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := model.NewCdpURLResult(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}
