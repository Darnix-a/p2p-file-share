package transfer

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"p2p-drop/pkg/crypto"
	"p2p-drop/pkg/protocol"
	"p2p-drop/pkg/transport"
	"p2p-drop/pkg/ui"
)

// SendFile streams a file or directory over the transport with E2EE
func SendFile(tr transport.Transport, path string, roomCode string) error {
	defer tr.Close()

	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	isDir := fileInfo.IsDir()
	fileName := filepath.Base(path)
	var totalSize int64

	if isDir {
		totalSize, err = GetDirectorySize(path)
		if err != nil {
			return fmt.Errorf("failed to calculate directory size: %w", err)
		}
	} else {
		totalSize = fileInfo.Size()
	}

	// 1. Cryptographic Handshake
	ourKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate keypair: %w", err)
	}

	hostname, _ := os.Hostname()
	ourHandshake := protocol.HandshakeMsg{
		Version:   protocol.CurrentVersion,
		PublicKey: ourKeyPair.PublicKey,
		Device:    hostname,
	}

	handshakeBytes, err := protocol.EncodeJSONFrame(protocol.TypeHandshake, ourHandshake)
	if err != nil {
		return err
	}

	if err := tr.Send(handshakeBytes); err != nil {
		return fmt.Errorf("failed to send handshake: %w", err)
	}

	// Receive peer handshake
	rawPeerFrame, err := tr.Receive()
	if err != nil {
		return fmt.Errorf("failed to receive peer handshake: %w", err)
	}

	msgType, payload, err := protocol.ReadFrame(bytes.NewReader(rawPeerFrame))
	if err != nil {
		return fmt.Errorf("failed to parse peer handshake: %w", err)
	}
	if msgType != protocol.TypeHandshake {
		return fmt.Errorf("expected handshake message, got type 0x%x", msgType)
	}

	var peerHandshake protocol.HandshakeMsg
	if err := json.Unmarshal(payload, &peerHandshake); err != nil {
		return fmt.Errorf("failed to unmarshal peer handshake: %w", err)
	}

	// Derive shared session key
	sessionKey, err := crypto.DeriveSessionKey(ourKeyPair.PrivateKey, peerHandshake.PublicKey, roomCode, true)
	if err != nil {
		return fmt.Errorf("failed to derive session key: %w", err)
	}

	sessionCipher, err := crypto.NewSessionCipher(sessionKey)
	if err != nil {
		return fmt.Errorf("failed to initialize AEAD cipher: %w", err)
	}

	// 2. Send File Metadata
	metaMsg := protocol.FileMetaMsg{
		Name:        fileName,
		Size:        totalSize,
		IsDir:       isDir,
		ChunkSize:   protocol.DefaultChunkSize,
		TotalChunks: uint64((totalSize + protocol.DefaultChunkSize - 1) / protocol.DefaultChunkSize),
	}

	metaBytes, err := protocol.EncodeJSONFrame(protocol.TypeFileMeta, metaMsg)
	if err != nil {
		return err
	}

	if err := tr.Send(metaBytes); err != nil {
		return fmt.Errorf("failed to send metadata: %w", err)
	}

	fmt.Printf("Waiting for receiver to accept (%s - %s)...\n", fileName, ui.FormatBytes(totalSize))

	// 3. Wait for MetaResponse
	rawRespFrame, err := tr.Receive()
	if err != nil {
		return fmt.Errorf("failed to receive response from receiver: %w", err)
	}

	respType, respPayload, err := protocol.ReadFrame(bytes.NewReader(rawRespFrame))
	if err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if respType != protocol.TypeMetaResponse {
		return fmt.Errorf("unexpected response type: 0x%x", respType)
	}

	var metaResp protocol.MetaResponseMsg
	if err := json.Unmarshal(respPayload, &metaResp); err != nil {
		return fmt.Errorf("invalid meta response: %w", err)
	}

	if !metaResp.Accepted {
		return fmt.Errorf("receiver declined transfer: %s", metaResp.Reason)
	}

	fmt.Println("Receiver accepted. Streaming encrypted data...")

	// 4. Stream and Encrypt Chunks
	hasher := sha256.New()
	bar := ui.NewProgressBar(totalSize, fmt.Sprintf("Sending %s", fileName))

	var dataReader io.Reader
	if isDir {
		pr, pw := io.Pipe()
		go func() {
			err := StreamTarGz(path, pw)
			_ = pw.CloseWithError(err)
		}()
		dataReader = pr
	} else {
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer f.Close()
		dataReader = f
	}

	buf := make([]byte, protocol.DefaultChunkSize)
	var seq uint64 = 0

	for {
		n, err := dataReader.Read(buf)
		if n > 0 {
			chunkData := buf[:n]
			hasher.Write(chunkData)

			ciphertext, encErr := sessionCipher.EncryptChunk(seq, chunkData, nil)
			if encErr != nil {
				return fmt.Errorf("encryption error on chunk %d: %w", seq, encErr)
			}

			frame := protocol.EncodeChunkBinary(seq, ciphertext)
			if sendErr := tr.Send(frame); sendErr != nil {
				return fmt.Errorf("failed to send chunk %d: %w", seq, sendErr)
			}

			_ = bar.Add(n)
			seq++
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read error: %w", err)
		}
	}

	// 5. Send Transfer Done with Checksum
	finalChecksum := fmt.Sprintf("%x", hasher.Sum(nil))
	doneMsg := protocol.TransferDoneMsg{
		SHA256Checksum: finalChecksum,
	}

	doneBytes, err := protocol.EncodeJSONFrame(protocol.TypeTransferDone, doneMsg)
	if err != nil {
		return err
	}

	if err := tr.Send(doneBytes); err != nil {
		return fmt.Errorf("failed to send transfer completion frame: %w", err)
	}

	// 6. Wait for Receiver Acknowledgement
	rawAckFrame, err := tr.Receive()
	if err != nil {
		return fmt.Errorf("failed to receive transfer ack: %w", err)
	}

	ackType, ackPayload, err := protocol.ReadFrame(bytes.NewReader(rawAckFrame))
	if err != nil {
		return fmt.Errorf("failed to read ack: %w", err)
	}

	if ackType != protocol.TypeTransferAck {
		return fmt.Errorf("unexpected ack frame type: 0x%x", ackType)
	}

	var ack protocol.TransferAckMsg
	if err := json.Unmarshal(ackPayload, &ack); err != nil {
		return fmt.Errorf("failed to unmarshal transfer ack: %w", err)
	}

	if !ack.Success {
		return fmt.Errorf("receiver reported transfer failed: %s", ack.Message)
	}

	fmt.Printf("\nTransfer complete. Verified SHA-256: %s\n", finalChecksum)
	return nil
}
