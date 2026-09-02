package signaling

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// SignalEnvelope contains WebRTC SDP or ICE candidate info
type SignalEnvelope struct {
	Type      string      `json:"type"` // "offer", "answer", "candidate"
	SDP       string      `json:"sdp,omitempty"`
	Candidate interface{} `json:"candidate,omitempty"`
}

// Client connects to a signaling server over WebSocket
type Client struct {
	ws           *websocket.Conn
	url          string
	room         string
	role         string
	SignalChan   chan SignalEnvelope
	BinaryChan   chan []byte
	PeerJoined   chan struct{}
	PeerLeft     chan struct{}
	ErrorChan    chan error
	closeOnce    sync.Once
	done         chan struct{}
	writeMu      sync.Mutex
	isPeerJoined bool
	mu           sync.Mutex
}

// NewClient creates a new signaling client
func NewClient(serverURL string, roomCode string, role string) (*Client, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid signaling URL: %w", err)
	}

	// Normalize scheme
	if u.Scheme == "http" {
		u.Scheme = "ws"
	} else if u.Scheme == "https" {
		u.Scheme = "wss"
	}

	if u.Path == "" || u.Path == "/" {
		u.Path = "/ws"
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
	}

	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to signaling server at %s: %w", u.String(), err)
	}

	c := &Client{
		ws:         conn,
		url:        serverURL,
		room:       roomCode,
		role:       role,
		SignalChan: make(chan SignalEnvelope, 64),
		BinaryChan: make(chan []byte, 512),
		PeerJoined: make(chan struct{}, 1),
		PeerLeft:   make(chan struct{}, 1),
		ErrorChan:  make(chan error, 8),
		done:       make(chan struct{}),
	}

	go c.readLoop()

	// Send join message
	joinMsg := ClientMessage{
		Type: "join",
		Room: roomCode,
		Role: role,
	}
	if err := c.sendJSON(joinMsg); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("failed to join room: %w", err)
	}

	return c, nil
}

func (c *Client) sendJSON(v interface{}) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.ws.WriteJSON(v)
}

// SendSignal sends a WebRTC signal (SDP offer/answer or ICE candidate) to the peer
func (c *Client) SendSignal(signal SignalEnvelope) error {
	raw, err := json.Marshal(signal)
	if err != nil {
		return err
	}

	msg := ClientMessage{
		Type: "signal",
		Data: raw,
	}
	return c.sendJSON(msg)
}

// SendBinary streams raw binary frames directly to peer over the relay
func (c *Client) SendBinary(data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.ws.WriteMessage(websocket.BinaryMessage, data)
}

func (c *Client) readLoop() {
	defer func() {
		_ = c.Close()
	}()

	c.ws.SetReadLimit(16 * 1024 * 1024) // 16 MB max frame size

	for {
		msgType, rawMsg, err := c.ws.ReadMessage()
		if err != nil {
			select {
			case <-c.done:
				return
			default:
				c.ErrorChan <- fmt.Errorf("signaling connection error: %w", err)
				return
			}
		}

		if msgType == websocket.BinaryMessage {
			select {
			case c.BinaryChan <- rawMsg:
			case <-c.done:
				return
			}
			continue
		}

		var sMsg ServerMessage
		if err := json.Unmarshal(rawMsg, &sMsg); err != nil {
			continue
		}

		switch sMsg.Type {
		case "joined":
			if sMsg.Peers == 2 {
				c.notifyPeerJoined()
			}
		case "peer_joined":
			c.notifyPeerJoined()
		case "peer_left":
			select {
			case c.PeerLeft <- struct{}{}:
			default:
			}
		case "signal":
			var sig SignalEnvelope
			if err := json.Unmarshal(sMsg.Data, &sig); err == nil {
				select {
				case c.SignalChan <- sig:
				default:
				}
			}
		case "error":
			c.ErrorChan <- errors.New(sMsg.Message)
		}
	}
}

func (c *Client) notifyPeerJoined() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.isPeerJoined {
		c.isPeerJoined = true
		select {
		case c.PeerJoined <- struct{}{}:
		default:
		}
	}
}

// Close gracefully closes the signaling connection
func (c *Client) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.done)
		err = c.ws.Close()
	})
	return err
}
