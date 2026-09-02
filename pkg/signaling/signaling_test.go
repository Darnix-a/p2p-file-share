package signaling

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSignalingServerClient(t *testing.T) {
	// 1. Start test server
	server := NewServer()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", server.HandleWebSocket)
	ts := httptest.NewServer(mux)
	defer ts.Close()
	defer server.Stop()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	roomCode := "test-room-123"

	// 2. Connect Sender client
	senderClient, err := NewClient(wsURL, roomCode, "sender")
	if err != nil {
		t.Fatalf("Sender failed to connect: %v", err)
	}
	defer senderClient.Close()

	// 3. Connect Receiver client
	receiverClient, err := NewClient(wsURL, roomCode, "receiver")
	if err != nil {
		t.Fatalf("Receiver failed to connect: %v", err)
	}
	defer receiverClient.Close()

	// Wait for peer joined notification on both sides
	select {
	case <-senderClient.PeerJoined:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for sender PeerJoined")
	}

	select {
	case <-receiverClient.PeerJoined:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for receiver PeerJoined")
	}

	// 4. Send signal from Sender to Receiver
	testOffer := SignalEnvelope{
		Type: "offer",
		SDP:  "v=0\r\no=- 12345 2 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n",
	}

	if err := senderClient.SendSignal(testOffer); err != nil {
		t.Fatalf("Sender SendSignal failed: %v", err)
	}

	select {
	case sig := <-receiverClient.SignalChan:
		if sig.Type != "offer" || sig.SDP != testOffer.SDP {
			t.Fatalf("Received signal does not match: %+v", sig)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for receiver SignalChan")
	}
}
