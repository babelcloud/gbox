package adb_expose

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	TypeOpen = iota + 1
	TypeData
	TypeClose
	TypeError
	TypeAck
)

type Config struct {
	APIKey      string
	BoxID       string
	GboxURL     string
	LocalAddr   string
	TargetPorts []int
}

type PortForwardRequest struct {
	Ports []int `json:"ports"`
}

type PortForwardResponse struct {
	URL string `json:"url"`
}

type Stream struct {
	id        uint32
	localConn net.Conn
	closeCh   chan struct{}
	readyCh   chan struct{}
	mu        sync.Mutex
	closed    bool
	ready     bool
}

type MultiplexClient struct {
	ws      *websocket.Conn
	streams map[uint32]*Stream
	mu      sync.RWMutex
	nextID  uint32
	muID    sync.Mutex
	closeCh chan struct{}
	writeMu sync.Mutex
}

func NewMultiplexClient(ws *websocket.Conn) *MultiplexClient {
	return &MultiplexClient{
		ws:      ws,
		streams: make(map[uint32]*Stream),
		closeCh: make(chan struct{}),
	}
}

func (m *MultiplexClient) Close() {
	select {
	case <-m.closeCh:
	default:
		close(m.closeCh)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, stream := range m.streams {
		stream.Close()
	}
	m.streams = nil
}

func (m *MultiplexClient) Run() error {
	for {
		select {
		case <-m.closeCh:
			return nil
		default:
			messageType, data, err := m.ws.ReadMessage()
			if err != nil {
				return fmt.Errorf("websocket read error: %v", err)
			}

			if messageType != websocket.BinaryMessage {
				continue
			}

			msgType, streamID, payload, err := parseMessage(data)
			if err != nil {
				log.Printf("parse message error: %v", err)
				continue
			}

			switch msgType {
			case TypeData:
				m.HandleData(streamID, payload)
			case TypeClose:
				m.HandleClose(streamID)
			case TypeError:
				m.HandleError(streamID, payload)
			case TypeAck:
				m.HandleAck(streamID)
			default:
				log.Printf("unknown message type: %d", msgType)
			}
		}
	}
}

func (m *MultiplexClient) HandleData(streamID uint32, payload []byte) {
	m.mu.RLock()
	stream, exists := m.streams[streamID]
	m.mu.RUnlock()

	if !exists {
		log.Printf("stream %d not found", streamID)
		return
	}

	_, err := stream.localConn.Write(payload)
	if err != nil {
		log.Printf("localConn.Write error: %v", err)
		stream.Close()
		m.RemoveStream(streamID)
	}
}

func (m *MultiplexClient) HandleClose(streamID uint32) {
	m.mu.RLock()
	stream, exists := m.streams[streamID]
	m.mu.RUnlock()

	if exists {
		stream.Close()
		m.RemoveStream(streamID)
	}
}

func (m *MultiplexClient) HandleError(streamID uint32, payload []byte) {
	log.Printf("server error for stream %d: %s", streamID, string(payload))
	m.HandleClose(streamID)
}

func (m *MultiplexClient) HandleAck(streamID uint32) {
	m.mu.RLock()
	stream, exists := m.streams[streamID]
	m.mu.RUnlock()

	if !exists {
		log.Printf("received ack for unknown stream %d", streamID)
		return
	}

	stream.mu.Lock()
	if !stream.ready {
		stream.ready = true
		close(stream.readyCh)
	}
	stream.mu.Unlock()
}

func (m *MultiplexClient) NewStreamID() uint32 {
	m.muID.Lock()
	defer m.muID.Unlock()
	// client use even id, server use odd id
	// if future need client to access server, server use odd id
	m.nextID += 2
	return m.nextID
}

func (m *MultiplexClient) SendMessage(msgType byte, streamID uint32, payload []byte) error {
	m.writeMu.Lock()
	defer m.writeMu.Unlock()

	message := make([]byte, 5+len(payload))
	message[0] = msgType
	binary.BigEndian.PutUint32(message[1:5], streamID)
	copy(message[5:], payload)

	return m.ws.WriteMessage(websocket.BinaryMessage, message)
}

func (m *MultiplexClient) HandleStream(stream *Stream) {
	defer func() {
		stream.Close()
		m.RemoveStream(stream.id)
	}()

	select {
	case <-stream.readyCh:
	case <-stream.closeCh:
		return
	case <-time.After(10 * time.Second):
		log.Printf("timeout waiting for server ack for stream %d", stream.id)
		m.SendMessage(TypeClose, stream.id, nil)
		return
	}

	buf := make([]byte, 4096)
	for {
		select {
		case <-stream.closeCh:
			return
		default:
			n, err := stream.localConn.Read(buf)
			if err != nil {
				m.SendMessage(TypeClose, stream.id, nil)
				return
			}

			err = m.SendMessage(TypeData, stream.id, buf[:n])
			if err != nil {
				log.Printf("sendMessage error: %v", err)
				return
			}
		}
	}
}

func (m *MultiplexClient) RemoveStream(streamID uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.streams, streamID)
}

func (m *MultiplexClient) AddStream(streamID uint32, localConn net.Conn) *Stream {
	stream := &Stream{
		id:        streamID,
		localConn: localConn,
		closeCh:   make(chan struct{}),
		readyCh:   make(chan struct{}),
		ready:     false,
	}

	m.mu.Lock()
	if m.streams == nil {
		m.mu.Unlock()
		log.Printf("streams map is nil, client may be closed")
		stream.Close()
		return stream
	}
	m.streams[streamID] = stream
	m.mu.Unlock()

	go m.HandleStream(stream)

	return stream
}

func (s *Stream) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.closeCh)
		if !s.ready {
			close(s.readyCh)
		}
		s.localConn.Close()
	}
}

func HandleLocalConnWithClient(localConn net.Conn, client *MultiplexClient, remotePort int) {
	defer func() {
		localConn.Close()
	}()

	streamID := client.NewStreamID()
	stream := client.AddStream(streamID, localConn)

	// start multiplexing
	// the payload is <any_valid_ip>:<remote_port>
	// remote server limit the ip, so we use any valid ip as payload
	// And the <remote_port> must be in the port-forward-url response, remote server will check it
	err := client.SendMessage(TypeOpen, streamID, []byte(fmt.Sprintf("127.0.0.1:%d", remotePort)))
	if err != nil {
		log.Printf("send open message error: %v", err)
		return
	}

	<-stream.closeCh
}
