package protocol

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	CurrentVersion = 1
	DefaultChunkSize = 64 * 1024 // 64 KB per chunk

	TypeHandshake    byte = 0x01
	TypeFileMeta     byte = 0x02
	TypeMetaResponse byte = 0x03
	TypeChunk        byte = 0x04
	TypeTransferDone byte = 0x05
	TypeTransferAck  byte = 0x06
	TypeError        byte = 0xFF
)

// HandshakeMsg contains the ephemeral public key and version
type HandshakeMsg struct {
	Version   int      `json:"version"`
	PublicKey [32]byte `json:"public_key"`
	Device    string   `json:"device,omitempty"`
}

// FileMetaMsg contains metadata of the file or directory being sent
type FileMetaMsg struct {
	Name           string `json:"name"`
	Size           int64  `json:"size"`
	IsDir          bool   `json:"is_dir"`
	TotalChunks    uint64 `json:"total_chunks"`
	ChunkSize      uint32 `json:"chunk_size"`
	SHA256Checksum string `json:"sha256"`
}

// MetaResponseMsg indicates if the receiver accepts the transfer
type MetaResponseMsg struct {
	Accepted bool   `json:"accepted"`
	Reason   string `json:"reason,omitempty"`
}

// ChunkHeader contains chunk index and size
type ChunkHeader struct {
	Seq       uint64
	CipherLen uint32
}

// TransferDoneMsg is sent by the sender when all chunks have been emitted
type TransferDoneMsg struct {
	SHA256Checksum string `json:"sha256"`
}

// TransferAckMsg is sent by the receiver after decrypting and verifying the full file
type TransferAckMsg struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// ErrorMsg indicates a protocol error
type ErrorMsg struct {
	Message string `json:"message"`
}

// EncodeFrame serializes a message type and payload with 1-byte type + 4-byte big-endian length prefix
func EncodeFrame(msgType byte, payload []byte) []byte {
	buf := make([]byte, 5+len(payload))
	buf[0] = msgType
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(payload)))
	copy(buf[5:], payload)
	return buf
}

// EncodeJSONFrame serializes a struct to JSON and wraps in a frame
func EncodeJSONFrame(msgType byte, v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON frame: %w", err)
	}
	return EncodeFrame(msgType, data), nil
}

// ReadFrame reads a complete frame from an io.Reader
func ReadFrame(r io.Reader) (byte, []byte, error) {
	header := make([]byte, 5)
	if _, err := io.ReadFull(r, header); err != nil {
		return 0, nil, err
	}

	msgType := header[0]
	length := binary.BigEndian.Uint32(header[1:5])

	// Limit frame length to 10MB to prevent memory exhaustion
	if length > 10*1024*1024 {
		return 0, nil, errors.New("frame size exceeds maximum limit")
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}

	return msgType, payload, nil
}

// EncodeChunkBinary creates a binary-efficient frame for encrypted chunks:
// [TypeChunk: 1 byte][FrameLen: 4 bytes][Seq: 8 bytes][Ciphertext: N bytes]
func EncodeChunkBinary(seq uint64, ciphertext []byte) []byte {
	totalLen := 8 + len(ciphertext)
	buf := make([]byte, 5+totalLen)
	buf[0] = TypeChunk
	binary.BigEndian.PutUint32(buf[1:5], uint32(totalLen))
	binary.BigEndian.PutUint64(buf[5:13], seq)
	copy(buf[13:], ciphertext)
	return buf
}

// DecodeChunkBinary parses an encoded chunk payload
func DecodeChunkBinary(payload []byte) (seq uint64, ciphertext []byte, err error) {
	if len(payload) < 8 {
		return 0, nil, errors.New("chunk payload too short")
	}
	seq = binary.BigEndian.Uint64(payload[0:8])
	ciphertext = payload[8:]
	return seq, ciphertext, nil
}
