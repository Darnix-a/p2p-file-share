package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"p2p-drop/pkg/crypto"
	"p2p-drop/pkg/discovery"
	"p2p-drop/pkg/signaling"
	"p2p-drop/pkg/transfer"
	"p2p-drop/pkg/transport"
	"p2p-drop/pkg/ui"
)

var (
	relayURL   string
	lanMode    bool
	autoAccept bool
	outputDir  string
	customCode string
	relayPort  string
)

const defaultRelayURL = "ws://127.0.0.1:8080"

func main() {
	rootCmd := &cobra.Command{
		Use:   "p2p-drop",
		Short: "End-to-end encrypted peer-to-peer file transfer tool",
	}

	rootCmd.PersistentFlags().StringVar(&relayURL, "relay", defaultRelayURL, "Signaling relay server URL")

	// Send Command
	sendCmd := &cobra.Command{
		Use:   "send <file-or-dir-path>",
		Short: "Send a file or folder to a peer",
		Args:  cobra.ExactArgs(1),
		Run:   runSend,
	}
	sendCmd.Flags().StringVar(&customCode, "code", "", "Custom pairing code")
	sendCmd.Flags().BoolVar(&lanMode, "lan", false, "Use direct local network discovery and transfer")

	// Receive Command
	receiveCmd := &cobra.Command{
		Use:   "receive [room-code]",
		Short: "Receive a file or folder from a peer",
		Args:  cobra.MaximumNArgs(1),
		Run:   runReceive,
	}
	receiveCmd.Flags().StringVarP(&outputDir, "output", "o", ".", "Destination directory for received files")
	receiveCmd.Flags().BoolVarP(&autoAccept, "yes", "y", false, "Accept file transfer without confirmation prompt")
	receiveCmd.Flags().BoolVar(&lanMode, "lan", false, "Listen for local network broadcast drops")

	// Relay Server Command
	relayCmd := &cobra.Command{
		Use:   "relay",
		Short: "Start signaling relay server",
		Run:   runRelay,
	}
	relayCmd.Flags().StringVarP(&relayPort, "port", "p", "8080", "Port to listen on")

	rootCmd.AddCommand(sendCmd, receiveCmd, relayCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runSend(cmd *cobra.Command, args []string) {
	targetPath := args[0]

	fileInfo, err := os.Stat(targetPath)
	if err != nil {
		log.Fatalf("Error accessing %s: %v\n", targetPath, err)
	}

	roomCode := customCode
	if roomCode == "" {
		code, err := crypto.GenerateCode(3)
		if err != nil {
			log.Fatalf("Failed to generate pairing code: %v\n", err)
		}
		roomCode = code
	}
	roomCode = crypto.SanitizeCode(roomCode)

	var totalSize int64
	if fileInfo.IsDir() {
		totalSize, _ = transfer.GetDirectorySize(targetPath)
		fmt.Printf("Preparing directory: %s (%s)\n", targetPath, ui.FormatBytes(totalSize))
	} else {
		totalSize = fileInfo.Size()
		fmt.Printf("Preparing file: %s (%s)\n", targetPath, ui.FormatBytes(totalSize))
	}

	fmt.Printf("\nPairing code: %s\n", roomCode)
	fmt.Println("Run this on the receiving machine:")
	if relayURL != defaultRelayURL {
		fmt.Printf("  p2p-drop receive %s --relay %s\n\n", roomCode, relayURL)
	} else {
		fmt.Printf("  p2p-drop receive %s\n\n", roomCode)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nTransfer cancelled.")
		cancel()
		os.Exit(0)
	}()

	if lanMode {
		tcpPort := 9876
		listener, acceptFunc, err := transport.ListenAndAcceptTCP(tcpPort, 5*time.Minute)
		if err != nil {
			log.Fatalf("Failed to start LAN listener: %v\n", err)
		}
		defer listener.Close()

		broadcaster := discovery.NewBroadcaster(roomCode, tcpPort, fileInfo.Name(), totalSize)
		_ = broadcaster.Start()
		defer broadcaster.Stop()

		fmt.Println("Broadcasting on local network... Waiting for receiver to connect...")

		tr, err := acceptFunc()
		if err != nil {
			log.Fatalf("LAN connection error: %v\n", err)
		}
		defer tr.Close()

		fmt.Println("Connected via LAN. Starting encrypted transfer...")
		if err := transfer.SendFile(tr, targetPath, roomCode); err != nil {
			log.Fatalf("Transfer failed: %v\n", err)
		}
		return
	}

	// Internet Mode
	fmt.Printf("Connecting to relay (%s)...\n", relayURL)
	sigClient, err := signaling.NewClient(relayURL, roomCode, "sender")
	if err != nil {
		log.Fatalf("Signaling error: %v\n(Check relay URL or ensure relay is running)\n", err)
	}
	defer sigClient.Close()

	fmt.Println("Waiting for receiver to connect...")
	select {
	case <-sigClient.PeerJoined:
		fmt.Println("Peer connected.")
	case err := <-sigClient.ErrorChan:
		log.Fatalf("Signaling error: %v\n", err)
	case <-ctx.Done():
		return
	}

	tr, err := transport.EstablishTransport(ctx, sigClient, true)
	if err != nil {
		log.Fatalf("Connection failed: %v\n", err)
	}
	defer tr.Close()

	if err := transfer.SendFile(tr, targetPath, roomCode); err != nil {
		log.Fatalf("Transfer failed: %v\n", err)
	}
}

func runReceive(cmd *cobra.Command, args []string) {
	var roomCode string
	if len(args) > 0 {
		roomCode = crypto.SanitizeCode(args[0])
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nTransfer cancelled.")
		cancel()
		os.Exit(0)
	}()

	if lanMode || roomCode == "" {
		fmt.Println("Scanning local network for active drops...")
		listener := discovery.NewListener()
		if err := listener.Start(); err != nil {
			log.Printf("LAN discovery error: %v\n", err)
		} else {
			defer listener.Stop()
			select {
			case beacon := <-listener.FoundChan:
				if roomCode == "" || roomCode == beacon.RoomCode {
					fmt.Printf("Discovered drop from %s (%s, %s)\n", beacon.HostName, beacon.FileName, ui.FormatBytes(beacon.FileSize))
					roomCode = beacon.RoomCode
					addr := fmt.Sprintf("%s:%d", beacon.HostName, beacon.Port)
					tr, err := transport.DialTCP(addr, 10*time.Second)
					if err == nil {
						defer tr.Close()
						fmt.Println("Connected via LAN. Starting encrypted transfer...")
						if err := transfer.ReceiveFile(tr, outputDir, roomCode, autoAccept); err != nil {
							log.Fatalf("Transfer failed: %v\n", err)
						}
						return
					}
				}
			case <-time.After(3 * time.Second):
				if roomCode == "" {
					log.Fatal("No LAN drops discovered. Please provide a pairing code: p2p-drop receive <code>")
				}
			}
		}
	}

	if roomCode == "" {
		log.Fatal("Please specify pairing code: p2p-drop receive <code>")
	}

	fmt.Printf("Connecting for code: %s\n", roomCode)
	fmt.Printf("Connecting to relay (%s)...\n", relayURL)

	sigClient, err := signaling.NewClient(relayURL, roomCode, "receiver")
	if err != nil {
		log.Fatalf("Signaling error: %v\n(Check relay URL or ensure relay is running)\n", err)
	}
	defer sigClient.Close()

	tr, err := transport.EstablishTransport(ctx, sigClient, false)
	if err != nil {
		log.Fatalf("Connection failed: %v\n", err)
	}
	defer tr.Close()

	if err := transfer.ReceiveFile(tr, outputDir, roomCode, autoAccept); err != nil {
		log.Fatalf("Transfer failed: %v\n", err)
	}
}

func runRelay(cmd *cobra.Command, args []string) {
	addr := fmt.Sprintf("0.0.0.0:%s", relayPort)
	server := signaling.NewServer()
	fmt.Printf("Starting signaling relay on %s/ws\n", addr)
	fmt.Printf("Pass --relay ws://<your-ip>:%s on clients\n", relayPort)
	if err := server.Start(addr); err != nil {
		log.Fatalf("Relay server error: %v\n", err)
	}
}
