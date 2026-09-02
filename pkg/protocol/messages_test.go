package protocol

import (
	"bytes"
	"testing"
)

func TestFrameReadWrite(t *testing.T) {
	handshake := HandshakeMsg{
		Version:   1,
		PublicKey: [32]byte{1, 2, 3, 4},
		Device:    "test-device",
	}

	frame, err := EncodeJSONFrame(TypeHandshake, handshake)
	if err != nil {
		t.Fatalf("EncodeJSONFrame failed: %v", err)
	}

	reader := bytes.NewReader(frame)
	msgType, payload, err := ReadFrame(reader)
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}

	if msgType != TypeHandshake {
		t.Fatalf("Expected msgType %d, got %d", TypeHandshake, msgType)
	}

	if len(payload) == 0 {
		t.Fatalf("Payload is empty")
	}
}

func TestBinaryChunkFraming(t *testing.T) {
	fakeCiphertext := []byte("encrypted-payload-bytes-12345")
	seq := uint64(42)

	frame := EncodeChunkBinary(seq, fakeCiphertext)

	reader := bytes.NewReader(frame)
	msgType, payload, err := ReadFrame(reader)
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}

	if msgType != TypeChunk {
		t.Fatalf("Expected TypeChunk, got %d", msgType)
	}

	parsedSeq, parsedCipher, err := DecodeChunkBinary(payload)
	if err != nil {
		t.Fatalf("DecodeChunkBinary failed: %v", err)
	}

	if parsedSeq != seq {
		t.Fatalf("Expected seq %d, got %d", seq, parsedSeq)
	}

	if !bytes.Equal(parsedCipher, fakeCiphertext) {
		t.Fatalf("Ciphertext mismatch")
	}
}
