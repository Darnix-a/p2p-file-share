package signaling

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for CLI/WebRTC signaling
	},
	ReadBufferSize:  1024 * 1024,
	WriteBufferSize: 1024 * 1024,
}

// ClientMessage represents incoming text messages from CLI clients
type ClientMessage struct {
	Type string          `json:"type"` // "join", "signal", "leave"
	Room string          `json:"room,omitempty"`
	Role string          `json:"role,omitempty"` // "sender" or "receiver"
	Data json.RawMessage `json:"data,omitempty"`
}

// ServerMessage represents text messages sent back to clients
type ServerMessage struct {
	Type    string          `json:"type"` // "joined", "peer_joined", "peer_left", "signal", "error"
	Role    string          `json:"role,omitempty"`
	Peers   int             `json:"peers,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
	Message string          `json:"message,omitempty"`
}

type outMsg struct {
	msgType int
	data    []byte
}

type ClientConn struct {
	ws        *websocket.Conn
	roomID    string
	role      string
	send      chan outMsg
	closeOnce sync.Once
}

func (c *ClientConn) Close() {
	c.closeOnce.Do(func() {
		close(c.send)
		_ = c.ws.Close()
	})
}

type Room struct {
	id      string
	clients map[*ClientConn]bool
	mu      sync.Mutex
}

// Server is the signaling rendezvous hub
type Server struct {
	rooms      map[string]*Room
	mu         sync.RWMutex
	httpServer *http.Server
}

// NewServer creates a new signaling server
func NewServer() *Server {
	return &Server{
		rooms: make(map[string]*Room),
	}
}

// HashRoomCode computes a safe room ID hash from code
func HashRoomCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:16])
}

// Handler returns an http.Handler that can be mounted on any mux/server
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.HandleWebSocket)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	return mux
}

// Start runs the HTTP WebSocket signaling server on the given address (e.g. ":8080")
func (s *Server) Start(addr string) error {
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.Handler(),
	}

	log.Printf("Relay server listening on %s/ws\n", addr)
	return s.httpServer.ListenAndServe()
}

// Stop shuts down the signaling server
func (s *Server) Stop() error {
	if s.httpServer != nil {
		return s.httpServer.Close()
	}
	return nil
}

func (s *Server) removeClient(client *ClientConn) {
	if client.roomID == "" {
		client.Close()
		return
	}

	s.mu.Lock()
	room, ok := s.rooms[client.roomID]
	if ok {
		room.mu.Lock()
		delete(room.clients, client)
		remaining := len(room.clients)
		room.mu.Unlock()

		if remaining == 0 {
			delete(s.rooms, client.roomID)
		} else {
			s.broadcastToRoom(client.roomID, client, ServerMessage{
				Type:  "peer_left",
				Peers: remaining,
			})
		}
	}
	s.mu.Unlock()
	client.Close()
}

func (s *Server) broadcastToRoom(roomID string, sender *ClientConn, msg ServerMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	s.broadcastRawToRoom(roomID, sender, websocket.TextMessage, data)
}

func (s *Server) broadcastRawToRoom(roomID string, sender *ClientConn, msgType int, data []byte) {
	s.mu.RLock()
	room, ok := s.rooms[roomID]
	s.mu.RUnlock()
	if !ok {
		return
	}

	room.mu.Lock()
	defer room.mu.Unlock()
	for client := range room.clients {
		if client != sender {
			select {
			case client.send <- outMsg{msgType: msgType, data: data}:
			default:
				// Buffer full
			}
		}
	}
}

func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v\n", err)
		return
	}

	client := &ClientConn{
		ws:   ws,
		send: make(chan outMsg, 1024),
	}

	// Write pump with heartbeat ping
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer func() {
			ticker.Stop()
			_ = client.ws.Close()
		}()
		for {
			select {
			case msg, ok := <-client.send:
				if !ok {
					_ = client.ws.WriteMessage(websocket.CloseMessage, []byte{})
					return
				}
				if err := client.ws.WriteMessage(msg.msgType, msg.data); err != nil {
					return
				}
			case <-ticker.C:
				if err := client.ws.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second)); err != nil {
					return
				}
			}
		}
	}()

	// Read pump
	defer func() {
		s.removeClient(client)
	}()

	client.ws.SetReadLimit(16 * 1024 * 1024) // 16 MB max frame size
	client.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.ws.SetPongHandler(func(string) error {
		client.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		msgType, rawMsg, err := client.ws.ReadMessage()
		if err != nil {
			break
		}
		client.ws.SetReadDeadline(time.Now().Add(60 * time.Second))

		if msgType == websocket.BinaryMessage {
			if client.roomID != "" {
				s.broadcastRawToRoom(client.roomID, client, websocket.BinaryMessage, rawMsg)
			}
			continue
		}

		var cMsg ClientMessage
		if err := json.Unmarshal(rawMsg, &cMsg); err != nil {
			continue
		}

		switch cMsg.Type {
		case "join":
			if cMsg.Room == "" {
				s.sendError(client, "Room identifier is required")
				continue
			}

			roomID := HashRoomCode(cMsg.Room)
			client.roomID = roomID
			client.role = cMsg.Role

			s.mu.Lock()
			room, exists := s.rooms[roomID]
			if !exists {
				room = &Room{
					id:      roomID,
					clients: make(map[*ClientConn]bool),
				}
				s.rooms[roomID] = room
			}

			room.mu.Lock()
			// Auto-evict stale client with same role
			for existing := range room.clients {
				if existing.role == client.role {
					delete(room.clients, existing)
					existing.Close()
				}
			}

			room.clients[client] = true
			peerCount := len(room.clients)
			room.mu.Unlock()
			s.mu.Unlock()

			// Acknowledge join
			s.sendMsg(client, ServerMessage{
				Type:  "joined",
				Role:  client.role,
				Peers: peerCount,
			})

			// If both peers are in the room, notify both!
			if peerCount == 2 {
				s.broadcastToRoom(roomID, client, ServerMessage{
					Type:  "peer_joined",
					Peers: 2,
				})
			}

		case "signal":
			if client.roomID == "" {
				s.sendError(client, "Must join a room before sending signals")
				continue
			}

			s.broadcastToRoom(client.roomID, client, ServerMessage{
				Type: "signal",
				Data: cMsg.Data,
			})

		case "leave":
			return
		}
	}
}

func (s *Server) sendMsg(client *ClientConn, msg ServerMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case client.send <- outMsg{msgType: websocket.TextMessage, data: data}:
	default:
	}
}

func (s *Server) sendError(client *ClientConn, errMsg string) {
	s.sendMsg(client, ServerMessage{
		Type:    "error",
		Message: errMsg,
	})
}
