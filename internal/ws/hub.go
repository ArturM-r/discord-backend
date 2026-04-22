package ws

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/you/discord-backend/internal/model"
)

type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	channelID int64
	userID    int64
	username  string
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*Client]struct{})}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

func (h *Hub) Broadcast(channelID int64, event model.WSEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if c.channelID == channelID {
			select {
			case c.send <- data:
			default:
				// slow client — drop
			}
		}
	}
}

func (h *Hub) BroadcastPresence(channelID int64, event model.WSEvent) {
	h.Broadcast(channelID, event)
}

func NewClient(hub *Hub, conn *websocket.Conn, channelID, userID int64, username string) *Client {
	return &Client{
		hub:       hub,
		conn:      conn,
		send:      make(chan []byte, 256),
		channelID: channelID,
		userID:    userID,
		username:  username,
	}
}

func (c *Client) WritePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

func (c *Client) ReadPump(onMessage func(c *Client, raw []byte)) {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()
	c.conn.SetReadLimit(4096)
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("ws read error: %v", err)
			}
			return
		}
		onMessage(c, raw)
	}
}
