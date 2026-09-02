package transport

import (
	"context"
	"fmt"
	"time"

	"p2p-drop/pkg/signaling"
)

// EstablishTransport attempts direct P2P WebRTC connection with synchronized fallback to E2EE Relay
func EstablishTransport(ctx context.Context, sigClient *signaling.Client, isSender bool) (Transport, error) {
	fmt.Println("Negotiating direct WebRTC connection...")

	webrtcCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tr, err := ConnectWebRTC(webrtcCtx, sigClient, isSender, nil)
	if err == nil {
		fmt.Println("Direct P2P WebRTC connection established.")
		return tr, nil
	}

	// WebRTC timed out or failed on both sides -> Synchronized fallback to encrypted relay
	fmt.Println("Direct P2P unreachable. Falling back to encrypted relay...")
	fmt.Println("Connected via encrypted relay (ChaCha20-Poly1305 E2EE).")

	return NewRelayTransport(sigClient), nil
}
