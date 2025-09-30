package pipeline

import (
	"sync"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/core"
	"github.com/babelcloud/gbox/packages/cli/internal/util"
)

// Pipeline manages video/audio sample distribution.
type Pipeline struct {
	mu     sync.RWMutex
	spsPps []byte

	// Video subscribers
	videoSubs map[string]chan core.VideoSample

	// Audio subscribers
	audioSubs map[string]chan core.AudioSample
}

// NewPipeline creates a new pipeline.
func NewPipeline() *Pipeline {
	return &Pipeline{
		videoSubs: make(map[string]chan core.VideoSample),
		audioSubs: make(map[string]chan core.AudioSample),
	}
}

// CacheSpsPps caches SPS/PPS data for H.264 streams.
func (p *Pipeline) CacheSpsPps(spsPps []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.spsPps = spsPps
	util.GetLogger().Debug("Pipeline SPS/PPS cached", "size", len(spsPps))
}

// GetSpsPps returns cached SPS/PPS data.
func (p *Pipeline) GetSpsPps() []byte {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.spsPps
}

// SubscribeVideo adds a video subscriber.
func (p *Pipeline) SubscribeVideo(id string, bufferSize int) <-chan core.VideoSample {
	p.mu.Lock()
	defer p.mu.Unlock()

	ch := make(chan core.VideoSample, bufferSize)
	p.videoSubs[id] = ch
	util.GetLogger().Debug("Video subscriber added", "id", id, "total", len(p.videoSubs))
	return ch
}

// UnsubscribeVideo removes a video subscriber.
func (p *Pipeline) UnsubscribeVideo(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if ch, exists := p.videoSubs[id]; exists {
		close(ch)
		delete(p.videoSubs, id)
		util.GetLogger().Info("Video subscriber removed", "id", id, "total", len(p.videoSubs))
	}
}

// PublishVideo publishes a video sample to all subscribers.
func (p *Pipeline) PublishVideo(sample core.VideoSample) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for id, ch := range p.videoSubs {
		select {
		case ch <- sample:
			// Sample sent successfully
		default:
			// Channel is full, skip
			util.GetLogger().Warn("Video channel full, dropping sample", "subscriber", id)
		}
	}
}

// SubscribeAudio adds an audio subscriber.
func (p *Pipeline) SubscribeAudio(id string, bufferSize int) <-chan core.AudioSample {
	p.mu.Lock()
	defer p.mu.Unlock()

	ch := make(chan core.AudioSample, bufferSize)
	p.audioSubs[id] = ch
	util.GetLogger().Debug("Audio subscriber added", "id", id, "total", len(p.audioSubs))
	return ch
}

// UnsubscribeAudio removes an audio subscriber.
func (p *Pipeline) UnsubscribeAudio(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if ch, exists := p.audioSubs[id]; exists {
		close(ch)
		delete(p.audioSubs, id)
		util.GetLogger().Info("Audio subscriber removed", "id", id, "total", len(p.audioSubs))
	}
}

// PublishAudio publishes an audio sample to all subscribers.
func (p *Pipeline) PublishAudio(sample core.AudioSample) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.audioSubs) == 0 {
		util.GetLogger().Debug("🎵 No audio subscribers, dropping sample", "size", len(sample.Data))
		return
	}

	// Track successful sends
	successfulSends := 0
	totalSubscribers := len(p.audioSubs)

	// Create a list of subscribers to remove (dead channels)
	var deadSubscribers []string

	for id, ch := range p.audioSubs {
		select {
		case ch <- sample:
			util.GetLogger().Debug("🎵 Audio sample sent to subscriber", "subscriber", id, "size", len(sample.Data))
			successfulSends++
		default:
			// Channel is full, check if it's a dead channel
			select {
			case <-ch:
				// Channel was closed, mark for removal
				deadSubscribers = append(deadSubscribers, id)
				util.GetLogger().Warn("🎵 Dead audio subscriber detected, marking for removal", "subscriber", id)
			default:
				// Channel is just full, not dead
				util.GetLogger().Warn("🎵 Audio channel full, dropping sample", "subscriber", id)
			}
		}
	}

	// Remove dead subscribers
	if len(deadSubscribers) > 0 {
		p.mu.RUnlock() // Release read lock
		p.mu.Lock()    // Acquire write lock
		for _, id := range deadSubscribers {
			if ch, exists := p.audioSubs[id]; exists {
				close(ch)
				delete(p.audioSubs, id)
				util.GetLogger().Info("🎵 Dead audio subscriber removed", "id", id, "total", len(p.audioSubs))
			}
		}
		p.mu.Unlock() // Release write lock
		p.mu.RLock()  // Re-acquire read lock for consistency
	}

	// If no subscribers could accept the sample, log a warning
	if successfulSends == 0 {
		util.GetLogger().Warn("🎵 All audio channels full, dropping sample",
			"total_subscribers", totalSubscribers,
			"sample_size", len(sample.Data))
	}
}
