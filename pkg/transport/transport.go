package transport

import "io"

// Transport defines the common interface for peer-to-peer data transport
type Transport interface {
	io.Closer
	Send(data []byte) error
	Receive() ([]byte, error)
	Type() string
}
