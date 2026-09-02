package transport

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// TCPTransport implements direct socket transport
type TCPTransport struct {
	conn      net.Conn
	closeOnce sync.Once
	closed    chan struct{}
}

// NewTCPTransport creates a Transport from an existing net.Conn
func NewTCPTransport(conn net.Conn) *TCPTransport {
	return &TCPTransport{
		conn:   conn,
		closed: make(chan struct{}),
	}
}

// DialTCP connects to a remote TCP endpoint
func DialTCP(addr string, timeout time.Duration) (*TCPTransport, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("TCP dial failed: %w", err)
	}
	return NewTCPTransport(conn), nil
}

// ListenAndAcceptTCP binds a TCP port and waits for 1 incoming connection
func ListenAndAcceptTCP(port int, timeout time.Duration) (net.Listener, func() (*TCPTransport, error), error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return nil, nil, fmt.Errorf("TCP listen failed: %w", err)
	}

	acceptFunc := func() (*TCPTransport, error) {
		if timeout > 0 {
			if tcpListener, ok := listener.(*net.TCPListener); ok {
				_ = tcpListener.SetDeadline(time.Now().Add(timeout))
			}
		}
		conn, err := listener.Accept()
		if err != nil {
			return nil, fmt.Errorf("TCP accept failed: %w", err)
		}
		return NewTCPTransport(conn), nil
	}

	return listener, acceptFunc, nil
}

// Send sends a packet with 4-byte length prefix
func (t *TCPTransport) Send(data []byte) error {
	select {
	case <-t.closed:
		return errors.New("transport closed")
	default:
	}

	length := uint32(len(data))
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, length)

	if _, err := t.conn.Write(header); err != nil {
		return err
	}
	if _, err := t.conn.Write(data); err != nil {
		return err
	}
	return nil
}

// Receive reads the next length-prefixed packet
func (t *TCPTransport) Receive() ([]byte, error) {
	select {
	case <-t.closed:
		return nil, errors.New("transport closed")
	default:
	}

	header := make([]byte, 4)
	if _, err := io.ReadFull(t.conn, header); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(header)
	if length > 16*1024*1024 { // 16MB max
		return nil, errors.New("packet length exceeds max size")
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(t.conn, buf); err != nil {
		return nil, err
	}

	return buf, nil
}

// Type returns transport name
func (t *TCPTransport) Type() string {
	return "tcp-lan"
}

// Close closes the underlying TCP connection
func (t *TCPTransport) Close() error {
	var err error
	t.closeOnce.Do(func() {
		close(t.closed)
		err = t.conn.Close()
	})
	return err
}
