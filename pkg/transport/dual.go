package transport

import (
	"context"
	"fmt"
	"time"

	"p2p-drop/pkg/signaling"
)

// EstablishTransport attempts direct P2P WebRTC connection with automatic fallback to E2EE Relay
func EstablishTransport(ctx context.Context, sigClient *signaling.Client, isSender bool) (Transport, error) {
	fmt.Println("Negotiating direct WebRTC connection...")

	webrtcCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	tr, err := ConnectWebRTC(webrtcCtx, sigClient, isSender, nil)
	if err == nil {
		fmt.Println("Direct P2P WebRTC connection established.")
		return tr, nil
	}

	// WebRTC failed or timed out -> Seamless fallback to encrypted relay
	fmt.Printf("Direct P2P unreachable (%v). Falling back to encrypted relay...\n", err)
	fmt.Println("Connected via encrypted relay (ChaCha20-Poly1305 E2EE).")

	return NewRelayTransport(sigClient), nil
}
