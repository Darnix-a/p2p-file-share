package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
	"p2p-drop/pkg/signaling"
)

var DefaultICEServers = []webrtc.ICEServer{
	{
		URLs: []string{
			"stun:stun.l.google.com:19302",
			"stun:stun1.l.google.com:19302",
			"stun:stun2.l.google.com:19302",
			"stun:stun.cloudflare.com:3478",
		},
	},
}

// WebRTCTransport implements Transport over WebRTC DataChannel
type WebRTCTransport struct {
	peerConn    *webrtc.PeerConnection
	dataChannel *webrtc.DataChannel
	recvChan    chan []byte
	errChan     chan error
	closeOnce   sync.Once
	closed      chan struct{}
	readyChan   chan struct{}
	bufferedLow chan struct{}
}

// ConnectWebRTC establishes a peer-to-peer WebRTC DataChannel connection using the signaling client
func ConnectWebRTC(ctx context.Context, sigClient *signaling.Client, isSender bool, customICEServers []webrtc.ICEServer) (*WebRTCTransport, error) {
	iceServers := DefaultICEServers
	if len(customICEServers) > 0 {
		iceServers = customICEServers
	}

	config := webrtc.Configuration{
		ICEServers: iceServers,
	}

	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create PeerConnection: %w", err)
	}

	t := &WebRTCTransport{
		peerConn:    pc,
		recvChan:    make(chan []byte, 256),
		errChan:     make(chan error, 4),
		closed:      make(chan struct{}),
		readyChan:   make(chan struct{}),
		bufferedLow: make(chan struct{}, 1),
	}

	// Forward ICE candidates to signaling peer
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		candidateJSON := c.ToJSON()
		_ = sigClient.SendSignal(signaling.SignalEnvelope{
			Type:      "candidate",
			Candidate: candidateJSON,
		})
	})

	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		if s == webrtc.PeerConnectionStateFailed || s == webrtc.PeerConnectionStateClosed {
			select {
			case t.errChan <- errors.New("WebRTC connection closed or failed"):
			default:
			}
		}
	})

	setupDataChannel := func(dc *webrtc.DataChannel) {
		t.dataChannel = dc
		dc.SetBufferedAmountLowThreshold(512 * 1024) // 512 KB

		dc.OnBufferedAmountLow(func() {
			select {
			case t.bufferedLow <- struct{}{}:
			default:
			}
		})

		dc.OnOpen(func() {
			close(t.readyChan)
		})

		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			select {
			case t.recvChan <- msg.Data:
			case <-t.closed:
			}
		})

		dc.OnError(func(err error) {
			select {
			case t.errChan <- err:
			default:
			}
		})

		dc.OnClose(func() {
			select {
			case <-t.closed:
			default:
				_ = t.Close()
			}
		})
	}

	if isSender {
		// Sender creates DataChannel
		ordered := true
		dc, err := pc.CreateDataChannel("p2p-drop-data", &webrtc.DataChannelInit{
			Ordered: &ordered,
		})
		if err != nil {
			_ = pc.Close()
			return nil, fmt.Errorf("failed to create DataChannel: %w", err)
		}
		setupDataChannel(dc)

		// Create Offer
		offer, err := pc.CreateOffer(nil)
		if err != nil {
			_ = pc.Close()
			return nil, fmt.Errorf("failed to create offer: %w", err)
		}

		if err := pc.SetLocalDescription(offer); err != nil {
			_ = pc.Close()
			return nil, fmt.Errorf("failed to set local description: %w", err)
		}

		// Send Offer
		if err := sigClient.SendSignal(signaling.SignalEnvelope{
			Type: "offer",
			SDP:  offer.SDP,
		}); err != nil {
			_ = pc.Close()
			return nil, fmt.Errorf("failed to send offer: %w", err)
		}
	} else {
		// Receiver waits for DataChannel
		pc.OnDataChannel(func(dc *webrtc.DataChannel) {
			setupDataChannel(dc)
		})
	}

	// Handle incoming signaling messages in background
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.closed:
				return
			case sig, ok := <-sigClient.SignalChan:
				if !ok {
					return
				}
				switch sig.Type {
				case "offer":
					if !isSender {
						desc := webrtc.SessionDescription{
							Type: webrtc.SDPTypeOffer,
							SDP:  sig.SDP,
						}
						if err := pc.SetRemoteDescription(desc); err != nil {
							continue
						}
						answer, err := pc.CreateAnswer(nil)
						if err != nil {
							continue
						}
						if err := pc.SetLocalDescription(answer); err != nil {
							continue
						}
						_ = sigClient.SendSignal(signaling.SignalEnvelope{
							Type: "answer",
							SDP:  answer.SDP,
						})
					}

				case "answer":
					if isSender {
						desc := webrtc.SessionDescription{
							Type: webrtc.SDPTypeAnswer,
							SDP:  sig.SDP,
						}
						_ = pc.SetRemoteDescription(desc)
					}

				case "candidate":
					candBytes, err := json.Marshal(sig.Candidate)
					if err == nil {
						var init webrtc.ICECandidateInit
						if err := json.Unmarshal(candBytes, &init); err == nil {
							_ = pc.AddICECandidate(init)
						}
					}
				}
			}
		}
	}()

	// Wait for DataChannel to be ready
	select {
	case <-t.readyChan:
		return t, nil
	case err := <-t.errChan:
		_ = t.Close()
		return nil, err
	case <-ctx.Done():
		_ = t.Close()
		return nil, ctx.Err()
	}
}

// Send writes data to the WebRTC DataChannel with flow control / backpressure
func (t *WebRTCTransport) Send(data []byte) error {
	select {
	case <-t.closed:
		return errors.New("transport closed")
	default:
	}

	// If buffered amount exceeds 1.5MB, pause and wait for buffer to drain
	for t.dataChannel.BufferedAmount() > 1536*1024 {
		select {
		case <-t.bufferedLow:
		case <-time.After(100 * time.Millisecond):
		case <-t.closed:
			return errors.New("transport closed")
		}
	}

	return t.dataChannel.Send(data)
}

// Receive reads the next frame from the DataChannel
func (t *WebRTCTransport) Receive() ([]byte, error) {
	select {
	case data, ok := <-t.recvChan:
		if !ok {
			return nil, errors.New("channel closed")
		}
		return data, nil
	case err := <-t.errChan:
		return nil, err
	case <-t.closed:
		return nil, errors.New("transport closed")
	}
}

// Type returns transport name
func (t *WebRTCTransport) Type() string {
	return "webrtc"
}

// Close terminates DataChannel and PeerConnection
func (t *WebRTCTransport) Close() error {
	var err error
	t.closeOnce.Do(func() {
		close(t.closed)
		if t.dataChannel != nil {
			_ = t.dataChannel.Close()
		}
		if t.peerConn != nil {
			err = t.peerConn.Close()
		}
	})
	return err
}
