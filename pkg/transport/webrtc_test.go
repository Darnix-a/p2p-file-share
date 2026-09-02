package transport

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"p2p-drop/pkg/signaling"
)

func TestWebRTCTransportTransfer(t *testing.T) {
	// 1. Spin up signaling server
	sigServer := signaling.NewServer()
	ts := httptest.NewServer(sigServer.Handler())
	defer ts.Close()
	defer sigServer.Stop()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	roomCode := "test-webrtc-room-99"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 2. Sender Client
	senderSig, err := signaling.NewClient(wsURL, roomCode, "sender")
	if err != nil {
		t.Fatalf("Sender signaling connect failed: %v", err)
	}
	defer senderSig.Close()

	// 3. Receiver Client
	receiverSig, err := signaling.NewClient(wsURL, roomCode, "receiver")
	if err != nil {
		t.Fatalf("Receiver signaling connect failed: %v", err)
	}
	defer receiverSig.Close()

	// 4. Establish WebRTC connection from both ends
	type result struct {
		tr  *WebRTCTransport
		err error
	}

	senderChan := make(chan result, 1)
	receiverChan := make(chan result, 1)

	go func() {
		tr, err := ConnectWebRTC(ctx, senderSig, true, nil)
		senderChan <- result{tr: tr, err: err}
	}()

	go func() {
		tr, err := ConnectWebRTC(ctx, receiverSig, false, nil)
		receiverChan <- result{tr: tr, err: err}
	}()

	var senderTr, receiverTr *WebRTCTransport
	res1 := <-senderChan
	if res1.err != nil {
		t.Fatalf("Sender WebRTC connection failed: %v", res1.err)
	}
	senderTr = res1.tr
	defer senderTr.Close()

	res2 := <-receiverChan
	if res2.err != nil {
		t.Fatalf("Receiver WebRTC connection failed: %v", res2.err)
	}
	receiverTr = res2.tr
	defer receiverTr.Close()

	// 5. Send message across WebRTC DataChannel
	testMessage := []byte("Hello WebRTC DataChannel from P2P-Drop!")
	if err := senderTr.Send(testMessage); err != nil {
		t.Fatalf("Sender Send failed: %v", err)
	}

	recvData, err := receiverTr.Receive()
	if err != nil {
		t.Fatalf("Receiver Receive failed: %v", err)
	}

	if string(recvData) != string(testMessage) {
		t.Fatalf("Received data mismatch: got %s, want %s", string(recvData), string(testMessage))
	}
}
