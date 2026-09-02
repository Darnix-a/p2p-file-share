package discovery

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

const (
	DiscoveryPort = 9002
)

// BeaconMessage is the payload broadcasted across the LAN
type BeaconMessage struct {
	RoomCode string `json:"room_code"`
	Port     int    `json:"port"`
	HostName string `json:"host_name"`
	FileName string `json:"file_name,omitempty"`
	FileSize int64  `json:"file_size,omitempty"`
	Time     int64  `json:"time"`
}

// Broadcaster sends periodic LAN UDP broadcast beacons
type Broadcaster struct {
	beacon BeaconMessage
	stop   chan struct{}
	wg     sync.WaitGroup
}

// NewBroadcaster creates a LAN broadcaster
func NewBroadcaster(roomCode string, port int, fileName string, fileSize int64) *Broadcaster {
	hostname, _ := os.Hostname()
	return &Broadcaster{
		beacon: BeaconMessage{
			RoomCode: roomCode,
			Port:     port,
			HostName: hostname,
			FileName: fileName,
			FileSize: fileSize,
		},
		stop: make(chan struct{}),
	}
}

// Start begins broadcasting beacons every second
func (b *Broadcaster) Start() error {
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("255.255.255.255:%d", DiscoveryPort))
	if err != nil {
		return err
	}

	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return err
	}

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		defer conn.Close()

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			b.beacon.Time = time.Now().Unix()
			data, err := json.Marshal(b.beacon)
			if err == nil {
				_, _ = conn.Write(data)
			}

			select {
			case <-b.stop:
				return
			case <-ticker.C:
			}
		}
	}()

	return nil
}

// Stop halts the broadcast
func (b *Broadcaster) Stop() {
	close(b.stop)
	b.wg.Wait()
}

// Listener discovers available drop beacons on the LAN
type Listener struct {
	FoundChan chan BeaconMessage
	stop      chan struct{}
	wg        sync.WaitGroup
}

// NewListener creates a new LAN discovery listener
func NewListener() *Listener {
	return &Listener{
		FoundChan: make(chan BeaconMessage, 16),
		stop:      make(chan struct{}),
	}
}

// Start listens for broadcast packets
func (l *Listener) Start() error {
	addr := &net.UDPAddr{
		Port: DiscoveryPort,
		IP:   net.IPv4zero,
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return err
	}

	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		defer conn.Close()

		buf := make([]byte, 2048)
		for {
			_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				select {
				case <-l.stop:
					return
				default:
					continue
				}
			}

			var beacon BeaconMessage
			if err := json.Unmarshal(buf[:n], &beacon); err == nil {
				select {
				case l.FoundChan <- beacon:
				default:
				}
			}
		}
	}()

	return nil
}

// Stop halts the listener
func (l *Listener) Stop() {
	close(l.stop)
	l.wg.Wait()
}
