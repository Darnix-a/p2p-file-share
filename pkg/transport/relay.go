package transport

import (
	"errors"
	"sync"

	"p2p-drop/pkg/signaling"
)

// RelayTransport implements Transport over the encrypted signaling WebSocket connection
type RelayTransport struct {
	client    *signaling.Client
	closeOnce sync.Once
	closed    chan struct{}
}

// NewRelayTransport creates a Transport running over the signaling WebSocket
func NewRelayTransport(client *signaling.Client) *RelayTransport {
	return &RelayTransport{
		client: client,
		closed: make(chan struct{}),
	}
}

// Send sends binary frames to the peer via the WebSocket relay
func (t *RelayTransport) Send(data []byte) error {
	select {
	case <-t.closed:
		return errors.New("relay transport closed")
	default:
	}
	return t.client.SendBinary(data)
}

// Receive reads incoming binary frames from the WebSocket relay
func (t *RelayTransport) Receive() ([]byte, error) {
	select {
	case <-t.closed:
		return nil, errors.New("relay transport closed")
	case data, ok := <-t.client.BinaryChan:
		if !ok {
			return nil, errors.New("relay data channel closed")
		}
		return data, nil
	case err := <-t.client.ErrorChan:
		return nil, err
	}
}

// Type returns transport name
func (t *RelayTransport) Type() string {
	return "e2ee-relay"
}

// Close closes the relay transport
func (t *RelayTransport) Close() error {
	t.closeOnce.Do(func() {
		close(t.closed)
	})
	return nil
}
