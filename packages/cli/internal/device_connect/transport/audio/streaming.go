package audio

import (
	"io"
	"net/http"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/core"
)

// AudioStreamingService 音频流服务 - 保持向后兼容
type AudioStreamingService struct {
	source  core.Source
	service AudioService // 使用新的服务实现
}

// NewAudioStreamingService creates a new audio streaming service
func NewAudioStreamingService() *AudioStreamingService {
	factory := NewAudioServiceFactory()
	return &AudioStreamingService{
		service: factory.CreateAudioService(),
	}
}

// SetSource sets the audio source
func (s *AudioStreamingService) SetSource(source core.Source) {
	s.source = source
}

// StreamOpus 流式处理 Opus 音频 - 只支持WebM格式
// 保持向后兼容，委托给新的服务实现
func (s *AudioStreamingService) StreamOpus(deviceSerial string, writer io.Writer, format string) error {
	return s.service.StreamOpus(deviceSerial, writer, format)
}

// StreamWebM streams Opus audio as WebM with HTTP response handling
// 保持向后兼容，委托给新的服务实现
func (s *AudioStreamingService) StreamWebM(deviceSerial string, w http.ResponseWriter, r *http.Request) error {
	return s.service.StreamWebM(deviceSerial, w, r)
}

// 全局音频流服务实例
var audioService *AudioStreamingService

// GetAudioService 获取音频流服务实例
func GetAudioService() *AudioStreamingService {
	if audioService == nil {
		audioService = NewAudioStreamingService()
	}
	return audioService
}
