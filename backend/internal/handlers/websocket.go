package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"chess-game/internal/db"
	"chess-game/internal/models"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

type WebSocketHandler struct {
	db    *db.MongoDB
	hub   *Hub
}

func NewWebSocketHandler(database *db.MongoDB) *WebSocketHandler {
	hub := NewHub()
	go hub.Run()
	return &WebSocketHandler{db: database, hub: hub}
}

// Hub maintains active connections and broadcasts messages
type Hub struct {
	// Map of sessionId -> map of playerId -> connection
	sessions map[string]map[string]*Client
	mu       sync.RWMutex

	register   chan *Client
	unregister chan *Client
	broadcast  chan *BroadcastMessage
}

type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	sessionId string
	playerId  string
	send      chan []byte
}

type BroadcastMessage struct {
	SessionId string
	Message   []byte
	ExcludePlayerId string
}

type WSMessage struct {
	Type           string          `json:"type"`
	Game           *models.Game    `json:"game,omitempty"`
	Move           *models.Move    `json:"move,omitempty"`
	ResigningColor string          `json:"resigningColor,omitempty"`
}

func NewHub() *Hub {
	return &Hub{
		sessions:   make(map[string]map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *BroadcastMessage),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.sessions[client.sessionId] == nil {
				h.sessions[client.sessionId] = make(map[string]*Client)
			}
			h.sessions[client.sessionId][client.playerId] = client
			h.mu.Unlock()
			log.Printf("Client registered: session=%s player=%s", client.sessionId, client.playerId)

		case client := <-h.unregister:
			h.mu.Lock()
			if session, ok := h.sessions[client.sessionId]; ok {
				if _, ok := session[client.playerId]; ok {
					delete(session, client.playerId)
					close(client.send)
					if len(session) == 0 {
						delete(h.sessions, client.sessionId)
					}
				}
			}
			h.mu.Unlock()
			log.Printf("Client unregistered: session=%s player=%s", client.sessionId, client.playerId)

		case msg := <-h.broadcast:
			h.mu.RLock()
			if session, ok := h.sessions[msg.SessionId]; ok {
				for playerId, client := range session {
					if playerId != msg.ExcludePlayerId {
						select {
						case client.send <- msg.Message:
						default:
							close(client.send)
							delete(session, playerId)
						}
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) BroadcastToSession(sessionId string, message []byte, excludePlayerId string) {
	h.broadcast <- &BroadcastMessage{
		SessionId:       sessionId,
		Message:         message,
		ExcludePlayerId: excludePlayerId,
	}
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
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

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
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

func (h *WebSocketHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionId := vars["sessionId"]
	playerId := r.URL.Query().Get("playerId")

	if sessionId == "" || playerId == "" {
		http.Error(w, "Missing sessionId or playerId", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &Client{
		hub:       h.hub,
		conn:      conn,
		sessionId: sessionId,
		playerId:  playerId,
		send:      make(chan []byte, 256),
	}

	h.hub.register <- client

	go client.writePump()
	go client.readPump()
}

// BroadcastGameUpdate sends game update to all players in a session
func (h *WebSocketHandler) BroadcastGameUpdate(sessionId string, game *models.Game, excludePlayerId string) {
	msg := WSMessage{
		Type: "game_update",
		Game: game,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal game update: %v", err)
		return
	}
	h.hub.BroadcastToSession(sessionId, data, excludePlayerId)
}

// BroadcastMove sends a move to all players in a session
func (h *WebSocketHandler) BroadcastMove(sessionId string, game *models.Game, move *models.Move, excludePlayerId string) {
	msg := WSMessage{
		Type: "move",
		Game: game,
		Move: move,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal move: %v", err)
		return
	}
	h.hub.BroadcastToSession(sessionId, data, excludePlayerId)
}

// BroadcastPlayerJoined notifies that a player has joined
func (h *WebSocketHandler) BroadcastPlayerJoined(sessionId string, game *models.Game) {
	msg := WSMessage{
		Type: "player_joined",
		Game: game,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal player joined: %v", err)
		return
	}
	h.hub.BroadcastToSession(sessionId, data, "")
}

// GetHub returns the hub for use by other handlers
func (h *WebSocketHandler) GetHub() *Hub {
	return h.hub
}

// BroadcastResignation notifies that a player has resigned
func (h *WebSocketHandler) BroadcastResignation(sessionId string, game *models.Game, resigningColor string, excludePlayerId string) {
	msg := WSMessage{
		Type:           "resignation",
		Game:           game,
		ResigningColor: resigningColor,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal resignation: %v", err)
		return
	}
	h.hub.BroadcastToSession(sessionId, data, excludePlayerId)
}

// Helper to fetch game from DB
func (h *WebSocketHandler) GetGame(sessionId string) (*models.Game, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var game models.Game
	err := h.db.Games().FindOne(ctx, bson.M{"sessionId": sessionId}).Decode(&game)
	if err != nil {
		return nil, err
	}
	return &game, nil
}
