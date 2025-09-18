package mse

import (
	"context"
	"io"
	"os/exec"
	"syscall"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/core"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/pipeline"
	"github.com/babelcloud/gbox/packages/cli/internal/util"
)

// FFmpegPackager handles H.264 to fMP4 packaging using FFmpeg.
// It subscribes to a pipeline and outputs fMP4 data to a broadcaster.
type FFmpegPackager struct {
	ctx         context.Context
	cancel      context.CancelFunc
	pipeline    *pipeline.Pipeline
	broadcaster *pipeline.Broadcaster
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	running     bool
}

// NewFFmpegPackager creates a new FFmpeg-based H.264 to fMP4 packager.
func NewFFmpegPackager(p *pipeline.Pipeline, broadcaster *pipeline.Broadcaster) *FFmpegPackager {
	ctx, cancel := context.WithCancel(context.Background())
	return &FFmpegPackager{
		ctx:         ctx,
		cancel:      cancel,
		pipeline:    p,
		broadcaster: broadcaster,
	}
}

// Start begins the FFmpeg packaging process.
func (f *FFmpegPackager) Start() error {
	logger := util.GetLogger()

	if f.running {
		return nil
	}

	// Subscribe to the pipeline for video samples
	videoCh := f.pipeline.SubscribeVideo("mse_packager", 100)

	// Create FFmpeg command for H.264 to fMP4 conversion
	f.cmd = exec.CommandContext(f.ctx, "ffmpeg",
		"-f", "h264", // Input format: raw H.264
		"-i", "pipe:0", // Read from stdin
		"-c:v", "copy", // Copy video without re-encoding
		"-f", "mp4", // Output format: MP4
		"-movflags", "frag_keyframe+empty_moov+default_base_moof", // fMP4 flags
		"-fflags", "+genpts", // Generate presentation timestamps
		"-video_track_timescale", "1000", // Stable timescale
		"-reset_timestamps", "1", // Reset timestamps to start from 0
		"pipe:1", // Write to stdout
	)

	// Get stdin and stdout pipes
	stdin, err := f.cmd.StdinPipe()
	if err != nil {
		return err
	}
	f.stdin = stdin

	stdout, err := f.cmd.StdoutPipe()
	if err != nil {
		f.stdin.Close()
		return err
	}
	f.stdout = stdout

	// Start FFmpeg process
	if err := f.cmd.Start(); err != nil {
		f.stdin.Close()
		f.stdout.Close()
		return err
	}

	f.running = true
	logger.Info("FFmpeg MSE packager started", "pid", f.cmd.Process.Pid)

	// Start goroutines for input and output handling
	go f.feedInputLoop(videoCh)
	go f.readOutputLoop()

	return nil
}

// Stop stops the FFmpeg packaging process.
func (f *FFmpegPackager) Stop() error {
	logger := util.GetLogger()

	if !f.running {
		return nil
	}

	f.running = false
	f.cancel()

	// Close stdin to signal FFmpeg to finish
	if f.stdin != nil {
		f.stdin.Close()
	}

	// Wait for process to finish or kill it
	if f.cmd != nil && f.cmd.Process != nil {
		done := make(chan error, 1)
		go func() {
			done <- f.cmd.Wait()
		}()

		select {
		case err := <-done:
			logger.Info("FFmpeg MSE packager stopped", "error", err)
		case <-f.ctx.Done():
			// Force kill if it doesn't stop gracefully
			f.cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-done:
			default:
				f.cmd.Process.Kill()
			}
			logger.Warn("FFmpeg MSE packager force killed")
		}
	}

	// Close stdout
	if f.stdout != nil {
		f.stdout.Close()
	}

	return nil
}

// feedInputLoop reads video samples from the pipeline and feeds them to FFmpeg.
func (f *FFmpegPackager) feedInputLoop(videoCh <-chan core.VideoSample) {
	logger := util.GetLogger()
	defer logger.Info("FFmpeg input feed loop stopped")

	for {
		select {
		case <-f.ctx.Done():
			return
		case sample, ok := <-videoCh:
			if !ok {
				logger.Info("Video channel closed, stopping input feed")
				return
			}

			if len(sample.Data) > 0 && f.stdin != nil {
				_, err := f.stdin.Write(sample.Data)
				if err != nil {
					logger.Error("Failed to write to FFmpeg stdin", "error", err)
					return
				}
			}
		}
	}
}

// readOutputLoop reads fMP4 data from FFmpeg and sends it to the broadcaster.
func (f *FFmpegPackager) readOutputLoop() {
	logger := util.GetLogger()
	defer logger.Info("FFmpeg output read loop stopped")

	// Try to detect and cache the initialization segment
	if initSegment, remainingReader, err := pipeline.DetectInitSegmentFromStream(f.stdout); err == nil {
		if len(initSegment) > 0 {
			f.broadcaster.SetInitSegment(initSegment)
			logger.Info("fMP4 initialization segment cached", "size", len(initSegment))
		}

		// Continue reading and broadcasting the remaining data
		if remainingReader != nil {
			pipeline.StreamToBroadcaster(remainingReader, f.broadcaster, 4096)
		}
	} else {
		logger.Error("Failed to detect init segment", "error", err)
		// Fallback: just stream everything to broadcaster
		pipeline.StreamToBroadcaster(f.stdout, f.broadcaster, 4096)
	}
}
