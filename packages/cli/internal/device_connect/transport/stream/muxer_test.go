package stream

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 测试用的样本数据
var (
	testVideoSample = VideoSample{
		Data:       append(append([]byte{}, testSPS...), append(testPPS, testIDR...)...),
		PTS:        0,
		IsKeyFrame: true,
	}

	testAudioSample = AudioSample{
		Data: testAACFrame,
		PTS:  0,
	}
)

// TestMuxerInterface 测试 Muxer 接口的通用行为
func TestMuxerInterface(t *testing.T) {
	tests := []struct {
		name    string
		factory func(io.Writer) Muxer
	}{
		{
			name: "WebMMuxer",
			factory: func(w io.Writer) Muxer {
				return NewWebMMuxer(w)
			},
		},
		{
			name: "FMP4Muxer",
			factory: func(w io.Writer) Muxer {
				logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
				return NewFMP4Muxer(w, logger)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := tt.factory(&buf)

			// 测试 Initialize
			codecParams := &CodecParams{
				VideoSPS: testSPS,
				VideoPPS: testPPS,
			}

			// 为 FMP4 设置音频配置
			if tt.name == "FMP4MuxedWriter" {
				codecParams.AudioConfig = mpeg4audio.AudioSpecificConfig{
					Type:         2,
					SampleRate:   48000,
					ChannelCount: 2,
				}
			}

			err := writer.Initialize(1920, 1080, codecParams)
			require.NoError(t, err, "Initialize should succeed")

			// 测试 Stream
			videoCh := make(chan VideoSample, 10)
			audioCh := make(chan AudioSample, 10)

			// 发送测试数据
			go func() {
				defer close(videoCh)
				defer close(audioCh)

				for i := 0; i < 5; i++ {
					videoSample := testVideoSample
					videoSample.PTS = int64(i * 33000) // 30fps
					videoCh <- videoSample

					audioSample := testAudioSample
					audioSample.PTS = int64(i * 21333) // 48kHz
					audioCh <- audioSample

					time.Sleep(10 * time.Millisecond) // 模拟实时流
				}
			}()

			// 启动流处理
			err = writer.Stream(videoCh, audioCh)
			require.NoError(t, err, "Stream should complete successfully")

			// 验证输出数据
			assert.Greater(t, buf.Len(), 0, "Should have written data")

			// 测试 Close
			err = writer.Close()
			require.NoError(t, err, "Close should succeed")
		})
	}
}

// TestMuxerErrorHandling 测试错误处理
func TestMuxerErrorHandling(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	writer := NewFMP4Muxer(&buf, logger)

	// 测试未初始化就 Stream
	videoCh := make(chan VideoSample)
	audioCh := make(chan AudioSample)
	close(videoCh)
	close(audioCh)

	err := writer.Stream(videoCh, audioCh)
	assert.Error(t, err, "Should error when not initialized")
}

// TestMuxerConcurrency 测试并发安全性
func TestMuxerConcurrency(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	writer := NewFMP4Muxer(&buf, logger)

	codecParams := &CodecParams{
		VideoSPS: testSPS,
		VideoPPS: testPPS,
		AudioConfig: mpeg4audio.AudioSpecificConfig{
			Type:         2,
			SampleRate:   48000,
			ChannelCount: 2,
		},
	}

	err := writer.Initialize(1920, 1080, codecParams)
	require.NoError(t, err)

	// 并发发送数据
	videoCh := make(chan VideoSample, 100)
	audioCh := make(chan AudioSample, 100)

	// 启动多个发送者
	for i := 0; i < 3; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				videoSample := testVideoSample
				videoSample.PTS = int64((id*10 + j) * 33000)
				videoCh <- videoSample

				audioSample := testAudioSample
				audioSample.PTS = int64((id*10 + j) * 21333)
				audioCh <- audioSample
			}
		}(i)
	}

	// 启动流处理
	go func() {
		time.Sleep(100 * time.Millisecond)
		close(videoCh)
		close(audioCh)
	}()

	err = writer.Stream(videoCh, audioCh)
	require.NoError(t, err)

	// 验证输出
	assert.Greater(t, buf.Len(), 0, "Should have written data under concurrent load")
}

// BenchmarkMuxer 性能测试
func BenchmarkMuxer(b *testing.B) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	writer := NewFMP4Muxer(&buf, logger)

	codecParams := &CodecParams{
		VideoSPS: testSPS,
		VideoPPS: testPPS,
		AudioConfig: mpeg4audio.AudioSpecificConfig{
			Type:         2,
			SampleRate:   48000,
			ChannelCount: 2,
		},
	}

	writer.Initialize(1920, 1080, codecParams)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		videoCh := make(chan VideoSample, 10)
		audioCh := make(chan AudioSample, 10)

		// 发送数据
		go func() {
			defer close(videoCh)
			defer close(audioCh)
			for j := 0; j < 10; j++ {
				videoCh <- VideoSample{Data: testIDR, PTS: int64(j * 33000), IsKeyFrame: true}
				audioCh <- AudioSample{Data: testAACFrame, PTS: int64(j * 21333)}
			}
		}()

		writer.Stream(videoCh, audioCh)
	}
}
