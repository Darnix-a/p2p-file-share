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

// ReceiveFile streams and decrypts incoming file/directory from peer
func ReceiveFile(tr transport.Transport, outputDir string, roomCode string, autoAccept bool) error {
	defer tr.Close()

	if outputDir == "" {
		outputDir = "."
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

	// Read sender's handshake
	rawSenderFrame, err := tr.Receive()
	if err != nil {
		return fmt.Errorf("failed to receive sender handshake: %w", err)
	}

	msgType, payload, err := protocol.ReadFrame(bytes.NewReader(rawSenderFrame))
	if err != nil {
		return fmt.Errorf("failed to parse sender handshake: %w", err)
	}
	if msgType != protocol.TypeHandshake {
		return fmt.Errorf("expected handshake message, got type 0x%x", msgType)
	}

	var senderHandshake protocol.HandshakeMsg
	if err := json.Unmarshal(payload, &senderHandshake); err != nil {
		return fmt.Errorf("failed to unmarshal sender handshake: %w", err)
	}

	// Send our handshake back
	if err := tr.Send(handshakeBytes); err != nil {
		return fmt.Errorf("failed to send handshake response: %w", err)
	}

	// Derive shared session key
	sessionKey, err := crypto.DeriveSessionKey(ourKeyPair.PrivateKey, senderHandshake.PublicKey, roomCode, false)
	if err != nil {
		return fmt.Errorf("failed to derive session key: %w", err)
	}

	sessionCipher, err := crypto.NewSessionCipher(sessionKey)
	if err != nil {
		return fmt.Errorf("failed to initialize AEAD cipher: %w", err)
	}

	// 2. Receive File Metadata
	rawMetaFrame, err := tr.Receive()
	if err != nil {
		return fmt.Errorf("failed to receive file metadata: %w", err)
	}

	metaType, metaPayload, err := protocol.ReadFrame(bytes.NewReader(rawMetaFrame))
	if err != nil {
		return fmt.Errorf("failed to parse file metadata: %w", err)
	}
	if metaType != protocol.TypeFileMeta {
		return fmt.Errorf("expected file meta message, got 0x%x", metaType)
	}

	var meta protocol.FileMetaMsg
	if err := json.Unmarshal(metaPayload, &meta); err != nil {
		return fmt.Errorf("invalid file metadata: %w", err)
	}

	typeStr := "File"
	if meta.IsDir {
		typeStr = "Directory"
	}

	fmt.Printf("\n📦 Incoming %s: %s (%s)\n", typeStr, meta.Name, ui.FormatBytes(meta.Size))
	if senderHandshake.Device != "" {
		fmt.Printf("💻 From Peer: %s\n", senderHandshake.Device)
	}

	// 3. User Confirmation Prompt
	accepted := autoAccept
	if !accepted {
		accepted = ui.PromptConfirm("Do you want to accept this transfer?")
	}

	metaResp := protocol.MetaResponseMsg{
		Accepted: accepted,
		Reason:   "Declined by receiver",
	}
	if accepted {
		metaResp.Reason = ""
	}

	respBytes, err := protocol.EncodeJSONFrame(protocol.TypeMetaResponse, metaResp)
	if err != nil {
		return err
	}
	if err := tr.Send(respBytes); err != nil {
		return fmt.Errorf("failed to send meta response: %w", err)
	}

	if !accepted {
		fmt.Println("❌ Transfer declined.")
		return nil
	}

	// 4. Prepare Destination
	var writer io.Writer
	var closeWriter func() error
	var extractDone chan error

	if meta.IsDir {
		pr, pw := io.Pipe()
		writer = pw
		extractDone = make(chan error, 1)
		go func() {
			err := ExtractTarGz(pr, outputDir)
			extractDone <- err
		}()
		closeWriter = func() error {
			_ = pw.Close()
			return <-extractDone
		}
	} else {
		targetFilePath := filepath.Join(outputDir, filepath.Base(meta.Name))
		if err := os.MkdirAll(filepath.Dir(targetFilePath), 0755); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}

		outFile, err := os.OpenFile(targetFilePath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("failed to open output file %s: %w", targetFilePath, err)
		}
		writer = outFile
		closeWriter = outFile.Close
	}

	// 5. Receive, Decrypt Chunks and Calculate Checksum
	hasher := sha256.New()
	bar := ui.NewProgressBar(meta.Size, fmt.Sprintf("Receiving %s", meta.Name))

	for {
		rawChunkFrame, err := tr.Receive()
		if err != nil {
			_ = closeWriter()
			return fmt.Errorf("failed receiving chunk: %w", err)
		}

		chunkReader := bytes.NewReader(rawChunkFrame)
		frameType, payload, err := protocol.ReadFrame(chunkReader)
		if err != nil {
			_ = closeWriter()
			return fmt.Errorf("failed to parse chunk frame: %w", err)
		}

		if frameType == protocol.TypeTransferDone {
			// All chunks received
			var doneMsg protocol.TransferDoneMsg
			if err := json.Unmarshal(payload, &doneMsg); err != nil {
				_ = closeWriter()
				return fmt.Errorf("failed to parse transfer done message: %w", err)
			}

			if err := closeWriter(); err != nil {
				_ = sendAck(tr, false, fmt.Sprintf("Failed to finalize write: %v", err))
				return fmt.Errorf("failed to finalize file write: %w", err)
			}

			computedChecksum := fmt.Sprintf("%x", hasher.Sum(nil))
			if doneMsg.SHA256Checksum != "" && doneMsg.SHA256Checksum != computedChecksum {
				_ = sendAck(tr, false, "SHA-256 checksum mismatch")
				return fmt.Errorf("checksum mismatch!\nExpected: %s\nActual:   %s", doneMsg.SHA256Checksum, computedChecksum)
			}

			// Send Ack
			if err := sendAck(tr, true, "Transfer successful"); err != nil {
				return fmt.Errorf("failed to send transfer ack: %w", err)
			}

			fmt.Printf("\n🎉 Saved to: %s\n", outputDir)
			fmt.Printf("🔒 Verified SHA-256: %s\n", computedChecksum)
			return nil
		}

		if frameType != protocol.TypeChunk {
			_ = closeWriter()
			return fmt.Errorf("unexpected frame type: 0x%x", frameType)
		}

		seq, ciphertext, err := protocol.DecodeChunkBinary(payload)
		if err != nil {
			_ = closeWriter()
			return fmt.Errorf("failed to decode chunk binary: %w", err)
		}

		plaintext, err := sessionCipher.DecryptChunk(seq, ciphertext, nil)
		if err != nil {
			_ = closeWriter()
			_ = sendAck(tr, false, fmt.Sprintf("Decryption failed at chunk %d", seq))
			return fmt.Errorf("failed to decrypt chunk %d: %w", seq, err)
		}

		if _, err := writer.Write(plaintext); err != nil {
			_ = closeWriter()
			return fmt.Errorf("failed writing decrypted chunk to disk: %w", err)
		}

		hasher.Write(plaintext)
		_ = bar.Add(len(plaintext))
	}
}

func sendAck(tr transport.Transport, success bool, message string) error {
	ack := protocol.TransferAckMsg{
		Success: success,
		Message: message,
	}
	ackBytes, err := protocol.EncodeJSONFrame(protocol.TypeTransferAck, ack)
	if err != nil {
		return err
	}
	return tr.Send(ackBytes)
}
