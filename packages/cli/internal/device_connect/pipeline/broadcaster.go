package pipeline

import (
	"bytes"
	"io"
	"sync"

	"github.com/babelcloud/gbox/packages/cli/internal/util"
)

// Broadcaster provides a generic pub/sub mechanism that can cache
// initialization segments and distribute data to multiple subscribers.
type Broadcaster struct {
	mu          sync.RWMutex
	subscribers map[string]chan<- []byte
	initSegment []byte // Cached initialization segment (e.g., fMP4 header)
	hasInit     bool
	closed      bool
}

// NewBroadcaster creates a new broadcaster instance.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subscribers: make(map[string]chan<- []byte),
	}
}

// SetInitSegment caches the initialization segment that will be sent
// immediately to new subscribers.
func (b *Broadcaster) SetInitSegment(data []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.initSegment = make([]byte, len(data))
	copy(b.initSegment, data)
	b.hasInit = true

	util.GetLogger().Info("Broadcaster init segment cached", "size", len(data))
}

// Subscribe adds a new subscriber with the given ID and returns a channel
// that will receive broadcasted data. If an init segment is cached,
// it will be sent immediately.
func (b *Broadcaster) Subscribe(subscriberID string, bufferSize int) <-chan []byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		// Return a closed channel for closed broadcaster
		ch := make(chan []byte)
		close(ch)
		return ch
	}

	ch := make(chan []byte, bufferSize)
	b.subscribers[subscriberID] = ch

	// Send cached init segment immediately if available
	if b.hasInit && len(b.initSegment) > 0 {
		select {
		case ch <- b.initSegment:
			util.GetLogger().Debug("Init segment sent to new subscriber", "id", subscriberID, "size", len(b.initSegment))
		default:
			util.GetLogger().Warn("Failed to send init segment to new subscriber (channel full)", "id", subscriberID)
		}
	}

	util.GetLogger().Info("New subscriber added", "id", subscriberID, "total", len(b.subscribers))
	return ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (b *Broadcaster) Unsubscribe(subscriberID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ch, exists := b.subscribers[subscriberID]; exists {
		close(ch)
		delete(b.subscribers, subscriberID)
		util.GetLogger().Info("Subscriber removed", "id", subscriberID, "remaining", len(b.subscribers))
	}
}

// Broadcast sends data to all current subscribers. If a subscriber's
// channel is full, that subscriber will be dropped.
func (b *Broadcaster) Broadcast(data []byte) {
	if len(data) == 0 {
		return
	}

	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return
	}

	// Create a copy of subscriber map to avoid holding the lock during broadcast
	subscribers := make(map[string]chan<- []byte, len(b.subscribers))
	for id, ch := range b.subscribers {
		subscribers[id] = ch
	}
	b.mu.RUnlock()

	// Broadcast to all subscribers
	var droppedSubscribers []string
	for id, ch := range subscribers {
		select {
		case ch <- data:
			// Successfully sent
		default:
			// Channel is full, mark for removal
			droppedSubscribers = append(droppedSubscribers, id)
			util.GetLogger().Warn("Dropping subscriber due to full channel", "id", id)
		}
	}

	// Remove dropped subscribers
	if len(droppedSubscribers) > 0 {
		b.mu.Lock()
		for _, id := range droppedSubscribers {
			if ch, exists := b.subscribers[id]; exists {
				close(ch)
				delete(b.subscribers, id)
			}
		}
		b.mu.Unlock()
	}
}

// Close shuts down the broadcaster and closes all subscriber channels.
func (b *Broadcaster) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}

	b.closed = true
	for id, ch := range b.subscribers {
		close(ch)
		util.GetLogger().Debug("Closed subscriber channel", "id", id)
	}
	b.subscribers = make(map[string]chan<- []byte)
	util.GetLogger().Info("Broadcaster closed")
}

// GetSubscriberCount returns the current number of subscribers.
func (b *Broadcaster) GetSubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}

// StreamToBroadcaster is a helper function that reads from an io.Reader
// and broadcasts the data. This is useful for piping FFmpeg output
// directly to the broadcaster.
func StreamToBroadcaster(reader io.Reader, broadcaster *Broadcaster, bufferSize int) error {
	logger := util.GetLogger()
	buffer := make([]byte, bufferSize)

	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			// Make a copy of the data before broadcasting
			data := make([]byte, n)
			copy(data, buffer[:n])
			broadcaster.Broadcast(data)
		}

		if err != nil {
			if err == io.EOF {
				logger.Info("Stream ended normally")
				return nil
			}
			logger.Error("Stream read error", "error", err)
			return err
		}
	}
}

// ExtractInitSegment attempts to extract the fMP4 initialization segment
// from the beginning of a stream. This looks for the 'ftyp' and 'moov' boxes.
func ExtractInitSegment(data []byte) (initSegment []byte, remaining []byte, found bool) {
	if len(data) < 8 {
		return nil, data, false
	}

	var offset int
	var foundFtyp, foundMoov bool

	// Look for ftyp and moov boxes
	for offset < len(data)-8 {
		if offset+8 > len(data) {
			break
		}

		// Read box size (big-endian)
		size := int(data[offset])<<24 | int(data[offset+1])<<16 | int(data[offset+2])<<8 | int(data[offset+3])
		if size < 8 || offset+size > len(data) {
			break
		}

		// Read box type
		boxType := string(data[offset+4 : offset+8])

		switch boxType {
		case "ftyp":
			foundFtyp = true
		case "moov":
			foundMoov = true
			// moov box completes the init segment
			initSegmentEnd := offset + size
			return data[:initSegmentEnd], data[initSegmentEnd:], true
		case "moof":
			// moof indicates start of media segments
			if foundFtyp && foundMoov {
				return data[:offset], data[offset:], true
			}
			// If we hit moof without complete init, something's wrong
			return nil, data, false
		}

		offset += size
	}

	// Haven't found complete init segment yet
	return nil, data, false
}

// DetectInitSegmentFromStream reads from a stream until it can extract
// the initialization segment, then returns both the init segment and
// a reader for the remaining data.
func DetectInitSegmentFromStream(reader io.Reader) (initSegment []byte, remainingReader io.Reader, err error) {
	var buffer bytes.Buffer
	tempBuf := make([]byte, 4096)

	for {
		n, readErr := reader.Read(tempBuf)
		if n > 0 {
			buffer.Write(tempBuf[:n])

			// Try to extract init segment
			if init, remaining, found := ExtractInitSegment(buffer.Bytes()); found {
				// Create a reader that contains the remaining data plus future reads
				remainingReader = io.MultiReader(bytes.NewReader(remaining), reader)
				return init, remainingReader, nil
			}
		}

		if readErr != nil {
			if readErr == io.EOF && buffer.Len() > 0 {
				// Return whatever we have as remaining data
				return nil, bytes.NewReader(buffer.Bytes()), nil
			}
			return nil, nil, readErr
		}

		// Prevent unbounded buffer growth
		if buffer.Len() > 1024*1024 { // 1MB limit
			return nil, bytes.NewReader(buffer.Bytes()), nil
		}
	}
}
