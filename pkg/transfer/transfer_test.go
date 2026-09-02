package transfer

import (
	"bytes"
	"crypto/rand"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"p2p-drop/pkg/transport"
)

func TestEndToEndFileTransfer(t *testing.T) {
	// 1. Create temporary test directories
	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, "test-file.bin")
	destDir := filepath.Join(tempDir, "received")

	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("Failed to create destDir: %v", err)
	}

	// Generate 2.5 MB random test payload
	testSize := int64(2500 * 1024)
	payload := make([]byte, testSize)
	if _, err := io.ReadFull(rand.Reader, payload); err != nil {
		t.Fatalf("Failed to generate test payload: %v", err)
	}

	if err := os.WriteFile(sourceFile, payload, 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// 2. Setup in-memory net.Pipe connection
	senderConn, receiverConn := net.Pipe()

	senderTransport := transport.NewTCPTransport(senderConn)
	receiverTransport := transport.NewTCPTransport(receiverConn)

	roomCode := "42-test-crystal-falcon"

	errChan := make(chan error, 2)

	// Start Receiver
	go func() {
		err := ReceiveFile(receiverTransport, destDir, roomCode, true)
		errChan <- err
	}()

	// Start Sender
	go func() {
		err := SendFile(senderTransport, sourceFile, roomCode)
		errChan <- err
	}()

	// Wait for both to finish
	for i := 0; i < 2; i++ {
		select {
		case err := <-errChan:
			if err != nil {
				t.Fatalf("Transfer failed: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for transfer completion")
		}
	}

	// 3. Verify received file matches source exactly
	receivedFilePath := filepath.Join(destDir, "test-file.bin")
	receivedBytes, err := os.ReadFile(receivedFilePath)
	if err != nil {
		t.Fatalf("Failed to read received file: %v", err)
	}

	if !bytes.Equal(payload, receivedBytes) {
		t.Fatalf("Payload mismatch: got %d bytes, want %d bytes", len(receivedBytes), len(payload))
	}
}

func TestEndToEndDirectoryTransfer(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "my-folder")
	destDir := filepath.Join(tempDir, "received-dir")

	if err := os.MkdirAll(filepath.Join(sourceDir, "sub"), 0755); err != nil {
		t.Fatalf("Failed to create sub dir: %v", err)
	}

	file1 := filepath.Join(sourceDir, "file1.txt")
	file2 := filepath.Join(sourceDir, "sub", "file2.txt")
	_ = os.WriteFile(file1, []byte("Content of file 1"), 0644)
	_ = os.WriteFile(file2, []byte("Content of file 2 in subfolder"), 0644)

	senderConn, receiverConn := net.Pipe()
	senderTransport := transport.NewTCPTransport(senderConn)
	receiverTransport := transport.NewTCPTransport(receiverConn)

	roomCode := "99-dir-transfer-room"

	errChan := make(chan error, 2)

	go func() {
		err := ReceiveFile(receiverTransport, destDir, roomCode, true)
		errChan <- err
	}()

	go func() {
		err := SendFile(senderTransport, sourceDir, roomCode)
		errChan <- err
	}()

	for i := 0; i < 2; i++ {
		select {
		case err := <-errChan:
			if err != nil {
				t.Fatalf("Directory transfer failed: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for directory transfer")
		}
	}

	// Check extracted files
	extracted1 := filepath.Join(destDir, "my-folder", "file1.txt")
	extracted2 := filepath.Join(destDir, "my-folder", "sub", "file2.txt")

	data1, err := os.ReadFile(extracted1)
	if err != nil || string(data1) != "Content of file 1" {
		t.Fatalf("Extracted file 1 mismatch: %s, err: %v", string(data1), err)
	}

	data2, err := os.ReadFile(extracted2)
	if err != nil || string(data2) != "Content of file 2 in subfolder" {
		t.Fatalf("Extracted file 2 mismatch: %s, err: %v", string(data2), err)
	}
}
