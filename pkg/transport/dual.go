package transport

import (
	"context"
	"fmt"
	"time"

	"p2p-drop/pkg/signaling"
)

// EstablishTransport attempts direct P2P WebRTC connection with automatic fallback to E2EE Relay
func EstablishTransport(ctx context.Context, sigClient *signaling.Client, isSender bool) (Transport, error) {
	fmt.Println("⏳ Negotiating direct P2P connection (WebRTC STUN)...")

	webrtcCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()

	tr, err := ConnectWebRTC(webrtcCtx, sigClient, isSender, nil)
	if err == nil {
		fmt.Println("⚡ Direct P2P WebRTC connection established!")
		return tr, nil
	}

	// WebRTC failed or timed out (strict NAT / blocked UDP) -> Seamless fallback to encrypted relay
	fmt.Printf("🔄 Direct P2P NAT traversal unreachable (%v). Switching to End-to-End Encrypted Relay...\n", err)
	fmt.Println("🔒 Connected via E2EE Relay (files remain 100% encrypted with ChaCha20-Poly1305).")

	return NewRelayTransport(sigClient), nil
}
