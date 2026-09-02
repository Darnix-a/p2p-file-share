package transport

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"p2p-drop/pkg/signaling"
)

func TestDualTransportRelayFallback(t *testing.T) {
	// 1. Spin up signaling server
	sigServer := signaling.NewServer()
	ts := httptest.NewServer(sigServer.Handler())
	defer ts.Close()
	defer sigServer.Stop()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	roomCode := "test-dual-room-42"

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// 2. Connect Sender client
	senderSig, err := signaling.NewClient(wsURL, roomCode, "sender")
	if err != nil {
		t.Fatalf("Sender signaling connect failed: %v", err)
	}
	defer senderSig.Close()

	// 3. Connect Receiver client
	receiverSig, err := signaling.NewClient(wsURL, roomCode, "receiver")
	if err != nil {
		t.Fatalf("Receiver signaling connect failed: %v", err)
	}
	defer receiverSig.Close()

	type result struct {
		tr  Transport
		err error
	}

	senderChan := make(chan result, 1)
	receiverChan := make(chan result, 1)

	go func() {
		tr, err := EstablishTransport(ctx, senderSig, true)
		senderChan <- result{tr: tr, err: err}
	}()

	go func() {
		tr, err := EstablishTransport(ctx, receiverSig, false)
		receiverChan <- result{tr: tr, err: err}
	}()

	res1 := <-senderChan
	if res1.err != nil {
		t.Fatalf("Sender transport failed: %v", res1.err)
	}
	senderTr := res1.tr
	defer senderTr.Close()

	res2 := <-receiverChan
	if res2.err != nil {
		t.Fatalf("Receiver transport failed: %v", res2.err)
	}
	receiverTr := res2.tr
	defer receiverTr.Close()

	// Send 1 MB of test binary payload
	testPayload := make([]byte, 1024*1024)
	for i := range testPayload {
		testPayload[i] = byte(i % 256)
	}

	go func() {
		_ = senderTr.Send(testPayload)
	}()

	recvData, err := receiverTr.Receive()
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	if len(recvData) != len(testPayload) {
		t.Fatalf("Payload length mismatch: got %d, want %d", len(recvData), len(testPayload))
	}
}
