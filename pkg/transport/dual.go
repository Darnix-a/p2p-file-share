package transport

import (
	"context"
	"fmt"
	"time"

	"p2p-drop/pkg/signaling"
)

// EstablishTransport attempts direct P2P WebRTC connection with automatic fallback to E2EE Relay
func EstablishTransport(ctx context.Context, sigClient *signaling.Client, isSender bool) (Transport, error) {
	fmt.Println("⏳ Gathering STUN candidates & establishing direct P2P WebRTC connection...")

	webrtcCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	tr, err := ConnectWebRTC(webrtcCtx, sigClient, isSender, nil)
	if err == nil {
		fmt.Println("⚡ Direct P2P WebRTC connection established (maximum speed direct transfer)!")
		return tr, nil
	}

	// WebRTC failed or timed out (strict NAT / blocked UDP) -> Seamless fallback to encrypted relay
	fmt.Printf("🔄 Direct P2P NAT hole-punching timed out (%v).\n", err)
	fmt.Println("🔒 Falling back to End-to-End Encrypted Relay (files remain 100% encrypted with ChaCha20-Poly1305).")

	return NewRelayTransport(sigClient), nil
}
