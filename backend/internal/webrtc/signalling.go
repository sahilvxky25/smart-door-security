package webrtc

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all origins for local network
	},
}

// Client represents a single WebSocket connection with a role.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	role   string // "door" or "owner"
	userID uint   // 0 if not authenticated
	send   chan []byte
}

// Hub maintains active clients and relays messages between door and owner.
type Hub struct {
	mu            sync.RWMutex
	clients       map[*Client]bool
	register      chan *Client
	unregister    chan *Client
	localDoorRecv chan []byte // in-process door peer reads from this channel
	activeOwnerID *uint       // user ID of the currently connected owner
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run processes register/unregister events. Must be started as a goroutine.
func (h *Hub) Run() {
	log.Println("[SignalingHub] Running")
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			if client.role == "owner" && client.userID != 0 {
				h.activeOwnerID = &client.userID
			}
			h.mu.Unlock()
			log.Printf("[SignalingHub] Client registered: role=%s userID=%d addr=%s total=%d",
				client.role, client.userID, client.conn.RemoteAddr(), len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("[SignalingHub] Client unregistered: role=%s total=%d",
				client.role, len(h.clients))
		}
	}
}

// broadcast sends a message to all clients of the specified role.
func (h *Hub) broadcast(message []byte, targetRole string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Deliver to local door peer when targeting "door"
	if targetRole == "door" && h.localDoorRecv != nil {
		select {
		case h.localDoorRecv <- message:
		default:
			log.Println("[SignalingHub] Local door peer recv buffer full, dropping message")
		}
	}

	for client := range h.clients {
		if client.role == targetRole {
			select {
			case client.send <- message:
			default:
				// buffer full, drop client
				close(client.send)
				delete(h.clients, client)
			}
		}
	}
}

// RegisterLocalDoor creates a channel pair for an in-process door peer.
// recv: the door peer reads incoming messages from this channel.
// send: the door peer writes outgoing messages via this function, which relays them to all owner clients.
func (h *Hub) RegisterLocalDoor() (recv <-chan []byte, send func([]byte)) {
	ch := make(chan []byte, 256)
	h.mu.Lock()
	h.localDoorRecv = ch
	h.mu.Unlock()

	sendFn := func(msg []byte) {
		h.broadcast(msg, "owner")
	}

	log.Println("[SignalingHub] Local door peer registered")
	return ch, sendFn
}

// UnregisterLocalDoor closes the local door peer channel.
func (h *Hub) UnregisterLocalDoor() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.localDoorRecv != nil {
		close(h.localDoorRecv)
		h.localDoorRecv = nil
		log.Println("[SignalingHub] Local door peer unregistered")
	}
}

// BroadcastAlert sends a typed security alert to all connected owner clients.
func (h *Hub) BroadcastAlert(eventType, title, body, imageURL string) {
	msg := map[string]string{
		"type":       "alert",
		"event_type": eventType,
		"title":      title,
		"body":       body,
		"image_url":  imageURL,
		"timestamp":  time.Now().Format(time.RFC3339),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[SignalingHub] Failed to marshal alert: %v", err)
		return
	}
	h.broadcast(data, "owner")
	log.Printf("[SignalingHub] Sent %s alert to owner clients", eventType)
}

// BroadcastIncomingCall sends an incoming_call notification to all owner clients.
// This triggers the accept/decline UI on the Flutter app.
func (h *Hub) BroadcastIncomingCall(imageURL string) {
	msg := map[string]string{
		"type":      "incoming_call",
		"image_url": imageURL,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[SignalingHub] Failed to marshal incoming call: %v", err)
		return
	}
	h.broadcast(data, "owner")
	log.Printf("[SignalingHub] Sent incoming_call to owner clients")
}

// BroadcastEventUpdate notifies all connected owner clients that a new event
// has been created, so dashboards can refresh in real-time.
func (h *Hub) BroadcastEventUpdate(eventType string) {
	msg := map[string]string{
		"type":       "event_update",
		"event_type": eventType,
		"timestamp":  time.Now().Format(time.RFC3339),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.broadcast(data, "owner")
	log.Printf("[SignalingHub] Sent event_update (%s) to owner clients", eventType)
}

// GetActiveOwnerID returns the user ID of the currently connected owner.
// Returns nil if no owner is connected.
func (h *Hub) GetActiveOwnerID() *uint {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.activeOwnerID
}

// HandleWebSocket is a Gin handler that upgrades HTTP to WebSocket.
func (h *Hub) HandleWebSocket(c *gin.Context) {
	role := c.Query("role")
	if role != "door" && role != "owner" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query param 'role' must be 'door' or 'owner'"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[SignalingHub] WebSocket upgrade failed: %v", err)
		return
	}

	// Parse optional user_id from query
	var userID uint
	if uidStr := c.Query("user_id"); uidStr != "" {
		if parsed, err := strconv.ParseUint(uidStr, 10, 32); err == nil {
			userID = uint(parsed)
		}
	}

	client := &Client{
		hub:    h,
		conn:   conn,
		role:   role,
		userID: userID,
		send:   make(chan []byte, 256),
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

// readPump reads messages from the WebSocket and relays to the opposite role.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(65536)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("[SignalingHub] Read error from %s: %v", c.role, err)
			}
			break
		}

		// Relay to opposite role
		var targetRole string
		if c.role == "door" {
			targetRole = "owner"
		} else {
			targetRole = "door"
		}
		c.hub.broadcast(message, targetRole)
	}
}

// writePump sends messages from the send channel to the WebSocket.
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
