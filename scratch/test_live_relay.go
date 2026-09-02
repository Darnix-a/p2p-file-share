package main

import (
	"fmt"
	"log"
	"time"

	"p2p-drop/pkg/signaling"
)

func main() {
	relayURL := "https://sufficiently-following-gis-womens.trycloudflare.com"
	roomCode := "diagnostic-test-room-123"

	fmt.Printf("1. Connecting Client A to %s...\n", relayURL)
	clientA, err := signaling.NewClient(relayURL, roomCode, "sender")
	if err != nil {
		log.Fatalf("Client A failed to connect: %v\n", err)
	}
	defer clientA.Close()
	fmt.Println("Client A connected and joined.")

	time.Sleep(1 * time.Second)

	fmt.Printf("2. Connecting Client B to %s...\n", relayURL)
	clientB, err := signaling.NewClient(relayURL, roomCode, "receiver")
	if err != nil {
		log.Fatalf("Client B failed to connect: %v\n", err)
	}
	defer clientB.Close()
	fmt.Println("Client B connected and joined.")

	// Wait for peer presence on both sides
	timeout := time.After(5 * time.Second)
	aJoined := false
	bJoined := false

	for !aJoined || !bJoined {
		select {
		case <-clientA.PeerJoined:
			fmt.Println("Client A detected Client B!")
			aJoined = true
		case <-clientB.PeerJoined:
			fmt.Println("Client B detected Client A!")
			bJoined = true
		case err := <-clientA.ErrorChan:
			log.Fatalf("Client A error: %v\n", err)
		case err := <-clientB.ErrorChan:
			log.Fatalf("Client B error: %v\n", err)
		case <-timeout:
			log.Fatalf("TIMEOUT: A_joined=%v, B_joined=%v\n", aJoined, bJoined)
		}
	}

	fmt.Println("SUCCESS: Both clients paired in room!")
}
