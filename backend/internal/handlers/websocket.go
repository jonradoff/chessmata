package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"chess-game/internal/auth"
	"chess-game/internal/db"
	"chess-game/internal/eventbus"
	"chess-game/internal/models"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

type WebSocketHandler struct {
	db         *db.MongoDB
	hub        *Hub
	eventBus   *eventbus.EventBus
	jwtService *auth.JWTService
}

// SetEventBus attaches the cross-machine event bus.
func (h *WebSocketHandler) SetEventBus(eb *eventbus.EventBus) {
	h.eventBus = eb
}

// NotifyMatchLocalFunc returns the local-only match notification function
// for use as the EventBus callback.
func (h *WebSocketHandler) NotifyMatchLocalFunc() func(string, string) {
	return func(connectionId string, sessionId string) {
		h.notifyMatchLocal(connectionId, sessionId)
	}
}

// broadcastAndPublish delivers a message locally and publishes to the event bus
// for cross-machine propagation.
func (h *WebSocketHandler) broadcastAndPublish(sessionId string, data []byte, excludePlayerId string) {
	h.hub.BroadcastToSession(sessionId, data, excludePlayerId)
	if h.eventBus != nil {
		go h.eventBus.PublishBroadcast(sessionId, data, excludePlayerId)
	}
}

func NewWebSocketHandler(database *db.MongoDB, jwtService *auth.JWTService) *WebSocketHandler {
	hub := NewHub()
	go hub.Run()
	return &WebSocketHandler{db: database, hub: hub, jwtService: jwtService}
}

// Hub maintains active connections and broadcasts messages
type Hub struct {
	// Map of sessionId -> map of playerId -> connection
	sessions map[string]map[string]*Client
	mu       sync.RWMutex

	// Spectators: sessionId -> list of spectator clients
	spectators          map[string][]*Client
	registerSpectator   chan *Client
	unregisterSpectator chan *Client

	// Matchmaking clients: connectionId -> *websocket.Conn
	matchmakingClients map[string]*websocket.Conn
	mmMu               sync.RWMutex

	// Lobby WebSocket clients
	lobbyClients map[*websocket.Conn]bool
	lobbyMu      sync.RWMutex

	register   chan *Client
	unregister chan *Client
	broadcast  chan *BroadcastMessage
}

type Client struct {
	hub         *Hub
	conn        *websocket.Conn
	sessionId   string
	playerId    string
	isSpectator bool
	send        chan []byte
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
	DrawFromColor  string          `json:"drawFromColor,omitempty"`
	Winner         string          `json:"winner,omitempty"`
	Reason         string          `json:"reason,omitempty"`
	WhiteTimeMs    int64           `json:"whiteTimeMs,omitempty"`
	BlackTimeMs    int64           `json:"blackTimeMs,omitempty"`
	ActiveColor    string          `json:"activeColor,omitempty"`
	AutoDeclined   bool            `json:"autoDeclined,omitempty"`
	ServerTime     int64           `json:"serverTime,omitempty"` // Server timestamp for clock sync
}

func NewHub() *Hub {
	return &Hub{
		sessions:            make(map[string]map[string]*Client),
		spectators:          make(map[string][]*Client),
		registerSpectator:   make(chan *Client),
		unregisterSpectator: make(chan *Client),
		matchmakingClients:  make(map[string]*websocket.Conn),
		lobbyClients:        make(map[*websocket.Conn]bool),
		register:            make(chan *Client),
		unregister:          make(chan *Client),
		broadcast:           make(chan *BroadcastMessage),
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

		case client := <-h.registerSpectator:
			h.mu.Lock()
			h.spectators[client.sessionId] = append(h.spectators[client.sessionId], client)
			h.mu.Unlock()
			log.Printf("Spectator registered: session=%s", client.sessionId)

		case client := <-h.unregisterSpectator:
			h.mu.Lock()
			specs := h.spectators[client.sessionId]
			for i, s := range specs {
				if s == client {
					h.spectators[client.sessionId] = append(specs[:i], specs[i+1:]...)
					close(client.send)
					break
				}
			}
			if len(h.spectators[client.sessionId]) == 0 {
				delete(h.spectators, client.sessionId)
			}
			h.mu.Unlock()
			log.Printf("Spectator unregistered: session=%s", client.sessionId)

		case msg := <-h.broadcast:
			// Collect dead clients under read lock, then remove under write lock
			type deadPlayer struct {
				sessionId string
				playerId  string
				client    *Client
			}
			var deadPlayers []deadPlayer
			var deadSpecs []*Client

			h.mu.RLock()
			// Send to players
			if session, ok := h.sessions[msg.SessionId]; ok {
				for playerId, client := range session {
					if playerId != msg.ExcludePlayerId {
						select {
						case client.send <- msg.Message:
						default:
							deadPlayers = append(deadPlayers, deadPlayer{msg.SessionId, playerId, client})
						}
					}
				}
			}
			// Send to all spectators (no exclusion)
			if specs, ok := h.spectators[msg.SessionId]; ok {
				for _, spec := range specs {
					select {
					case spec.send <- msg.Message:
					default:
						deadSpecs = append(deadSpecs, spec)
					}
				}
			}
			h.mu.RUnlock()

			// Clean up dead clients under write lock
			if len(deadPlayers) > 0 || len(deadSpecs) > 0 {
				h.mu.Lock()
				for _, dp := range deadPlayers {
					if session, ok := h.sessions[dp.sessionId]; ok {
						if _, exists := session[dp.playerId]; exists {
							close(dp.client.send)
							delete(session, dp.playerId)
							if len(session) == 0 {
								delete(h.sessions, dp.sessionId)
							}
						}
					}
				}
				for _, spec := range deadSpecs {
					specs := h.spectators[msg.SessionId]
					for i, s := range specs {
						if s == spec {
							close(spec.send)
							h.spectators[msg.SessionId] = append(specs[:i], specs[i+1:]...)
							break
						}
					}
					if len(h.spectators[msg.SessionId]) == 0 {
						delete(h.spectators, msg.SessionId)
					}
				}
				h.mu.Unlock()
			}
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
		if c.isSpectator {
			c.hub.unregisterSpectator <- c
		} else {
			c.hub.unregister <- c
		}
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
	spectator := r.URL.Query().Get("spectator")

	if sessionId == "" {
		http.Error(w, "Missing sessionId", http.StatusBadRequest)
		return
	}

	// Spectator mode: no playerId required
	isSpectator := spectator == "true" || playerId == ""

	if !isSpectator && playerId == "" {
		http.Error(w, "Missing playerId", http.StatusBadRequest)
		return
	}

	// Authenticate via optional JWT token query param for player connections
	if !isSpectator {
		tokenStr := r.URL.Query().Get("token")
		game, err := h.GetGame(sessionId)
		if err == nil && game != nil {
			// Find the player in the game
			for _, p := range game.Players {
				if p.ID == playerId && p.UserID != nil {
					// This player slot belongs to an authenticated user — require matching token
					if tokenStr == "" {
						http.Error(w, "Authentication required", http.StatusUnauthorized)
						return
					}
					claims, err := h.jwtService.ValidateAccessToken(tokenStr)
					if err != nil {
						http.Error(w, "Invalid token", http.StatusUnauthorized)
						return
					}
					claimUID, err := primitive.ObjectIDFromHex(claims.UserID)
					if err != nil {
						http.Error(w, "Invalid token claims", http.StatusUnauthorized)
						return
					}
					if claimUID != *p.UserID {
						http.Error(w, "Not authorized for this player", http.StatusForbidden)
						return
					}
					break
				}
			}
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &Client{
		hub:         h.hub,
		conn:        conn,
		sessionId:   sessionId,
		playerId:    playerId,
		isSpectator: isSpectator,
		send:        make(chan []byte, 256),
	}

	if isSpectator {
		h.hub.registerSpectator <- client
	} else {
		h.hub.register <- client
	}

	go client.writePump()
	go client.readPump()
}

// BroadcastGameUpdate sends game update to all players in a session
func (h *WebSocketHandler) BroadcastGameUpdate(sessionId string, game *models.Game, excludePlayerId string) {
	msg := WSMessage{
		Type:       "game_update",
		Game:       game,
		ServerTime: time.Now().UnixMilli(),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal game update: %v", err)
		return
	}
	h.broadcastAndPublish(sessionId, data, excludePlayerId)
}

// BroadcastMove sends a move to all players in a session
func (h *WebSocketHandler) BroadcastMove(sessionId string, game *models.Game, move *models.Move, excludePlayerId string) {
	msg := WSMessage{
		Type:       "move",
		Game:       game,
		Move:       move,
		ServerTime: time.Now().UnixMilli(),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal move: %v", err)
		return
	}
	h.broadcastAndPublish(sessionId, data, excludePlayerId)
}

// BroadcastPlayerJoined notifies that a player has joined
func (h *WebSocketHandler) BroadcastPlayerJoined(sessionId string, game *models.Game) {
	msg := WSMessage{
		Type:       "player_joined",
		Game:       game,
		ServerTime: time.Now().UnixMilli(),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal player joined: %v", err)
		return
	}
	h.broadcastAndPublish(sessionId, data, "")
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
		ServerTime:     time.Now().UnixMilli(),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal resignation: %v", err)
		return
	}
	h.broadcastAndPublish(sessionId, data, excludePlayerId)
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

// BroadcastDrawOffer notifies that a player has offered a draw
func (h *WebSocketHandler) BroadcastDrawOffer(sessionId string, fromColor string, game *models.Game) {
	msg := WSMessage{
		Type:          "draw_offered",
		DrawFromColor: fromColor,
		Game:          game,
		ServerTime:    time.Now().UnixMilli(),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal draw offer: %v", err)
		return
	}
	h.broadcastAndPublish(sessionId, data, "")
}

// BroadcastDrawDeclined notifies that a draw offer was declined
func (h *WebSocketHandler) BroadcastDrawDeclined(sessionId string, game *models.Game, autoDeclined bool) {
	msg := WSMessage{
		Type:         "draw_declined",
		Game:         game,
		AutoDeclined: autoDeclined,
		ServerTime:   time.Now().UnixMilli(),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal draw declined: %v", err)
		return
	}
	h.broadcastAndPublish(sessionId, data, "")
}

// BroadcastGameOver notifies that the game has ended
func (h *WebSocketHandler) BroadcastGameOver(sessionId string, game *models.Game, winner string, reason string) {
	msg := WSMessage{
		Type:       "game_over",
		Game:       game,
		Winner:     winner,
		Reason:     reason,
		ServerTime: time.Now().UnixMilli(),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal game over: %v", err)
		return
	}
	h.broadcastAndPublish(sessionId, data, "")
}

// BroadcastTimeUpdate sends current time state to all players
func (h *WebSocketHandler) BroadcastTimeUpdate(sessionId string, whiteTimeMs, blackTimeMs int64, activeColor string) {
	msg := WSMessage{
		Type:        "time_update",
		WhiteTimeMs: whiteTimeMs,
		BlackTimeMs: blackTimeMs,
		ActiveColor: activeColor,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal time update: %v", err)
		return
	}
	h.hub.BroadcastToSession(sessionId, data, "")
}

// HandleMatchmakingWebSocket handles WebSocket connections for matchmaking queue status.
// Clients connect with their connectionId and receive push notifications when matched.
func (h *WebSocketHandler) HandleMatchmakingWebSocket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	connectionId := vars["connectionId"]

	if connectionId == "" {
		http.Error(w, "Missing connectionId", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Matchmaking WebSocket upgrade failed: %v", err)
		return
	}

	// Register the matchmaking client
	h.hub.mmMu.Lock()
	h.hub.matchmakingClients[connectionId] = conn
	h.hub.mmMu.Unlock()
	log.Printf("Matchmaking client connected: %s", connectionId)

	// Keep connection alive with pings; clean up on disconnect
	defer func() {
		h.hub.mmMu.Lock()
		delete(h.hub.matchmakingClients, connectionId)
		h.hub.mmMu.Unlock()
		conn.Close()
		log.Printf("Matchmaking client disconnected: %s", connectionId)
	}()

	conn.SetReadLimit(512)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Ping ticker to keep connection alive
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Read pump — blocks until client disconnects
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// NotifyMatchFound sends a match notification to a matchmaking client.
// Called by the matchmaking queue when two players are paired.
// Delivers locally and publishes to EventBus for cross-machine delivery.
func (h *WebSocketHandler) NotifyMatchFound(connectionId string, sessionId string) {
	delivered := h.notifyMatchLocal(connectionId, sessionId)

	if h.eventBus != nil {
		go h.eventBus.PublishMatchNotification(connectionId, sessionId)
	}

	if !delivered {
		log.Printf("Matchmaking client not connected locally: %s (published to EventBus)", connectionId)
	}
}

// HandleLobbyWebSocket handles WebSocket connections for lobby updates.
// Connected clients receive the full lobby list whenever it changes.
func (h *WebSocketHandler) HandleLobbyWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Lobby WebSocket upgrade failed: %v", err)
		return
	}

	// Register lobby client
	h.hub.lobbyMu.Lock()
	h.hub.lobbyClients[conn] = true
	h.hub.lobbyMu.Unlock()
	log.Printf("Lobby client connected (total: %d)", len(h.hub.lobbyClients))

	defer func() {
		h.hub.lobbyMu.Lock()
		delete(h.hub.lobbyClients, conn)
		h.hub.lobbyMu.Unlock()
		conn.Close()
		log.Printf("Lobby client disconnected")
	}()

	// Send initial lobby state
	h.sendLobbyTo(conn)

	conn.SetReadLimit(512)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Ping ticker to keep connection alive
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// BroadcastLobbyUpdate sends the current lobby state to all connected lobby clients.
func (h *WebSocketHandler) BroadcastLobbyUpdate() {
	h.hub.lobbyMu.RLock()
	clients := make([]*websocket.Conn, 0, len(h.hub.lobbyClients))
	for conn := range h.hub.lobbyClients {
		clients = append(clients, conn)
	}
	h.hub.lobbyMu.RUnlock()

	if len(clients) == 0 {
		return
	}

	data := h.buildLobbyPayload()
	if data == nil {
		return
	}

	var dead []*websocket.Conn
	for _, conn := range clients {
		conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			dead = append(dead, conn)
		}
	}

	if len(dead) > 0 {
		h.hub.lobbyMu.Lock()
		for _, conn := range dead {
			delete(h.hub.lobbyClients, conn)
			conn.Close()
		}
		h.hub.lobbyMu.Unlock()
	}
}

// sendLobbyTo sends the current lobby state to a single connection.
func (h *WebSocketHandler) sendLobbyTo(conn *websocket.Conn) {
	data := h.buildLobbyPayload()
	if data == nil {
		return
	}
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	conn.WriteMessage(websocket.TextMessage, data)
}

// buildLobbyPayload queries the DB for waiting entries and marshals to JSON.
func (h *WebSocketHandler) buildLobbyPayload() []byte {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := h.db.MatchmakingQueue().Find(ctx, bson.M{
		"status": string(models.QueueStatusWaiting),
	})
	if err != nil {
		log.Printf("Failed to query lobby: %v", err)
		return nil
	}
	defer cursor.Close(ctx)

	type lobbyEntry struct {
		DisplayName    string   `json:"displayName"`
		AgentName      string   `json:"agentName,omitempty"`
		EngineName     string   `json:"engineName,omitempty"`
		IsRanked       bool     `json:"isRanked"`
		CurrentElo     int      `json:"currentElo"`
		OpponentType   string   `json:"opponentType"`
		TimeControls   []string `json:"timeControls"`
		PreferredColor *string  `json:"preferredColor,omitempty"`
		WaitingSince   string   `json:"waitingSince"`
	}

	entries := []lobbyEntry{}
	for cursor.Next(ctx) {
		var q models.MatchmakingQueue
		if err := cursor.Decode(&q); err != nil {
			continue
		}
		e := lobbyEntry{
			DisplayName:  q.DisplayName,
			AgentName:    q.AgentName,
			EngineName:   q.EngineName,
			IsRanked:     q.IsRanked,
			CurrentElo:   q.CurrentElo,
			OpponentType: string(q.OpponentType),
			WaitingSince: q.JoinedAt.Format(time.RFC3339),
		}
		for _, tc := range q.TimeControls {
			e.TimeControls = append(e.TimeControls, string(tc))
		}
		if len(e.TimeControls) == 0 {
			e.TimeControls = []string{}
		}
		if q.PreferredColor != nil {
			s := string(*q.PreferredColor)
			e.PreferredColor = &s
		}
		entries = append(entries, e)
	}

	msg := map[string]interface{}{
		"type":    "lobby_update",
		"entries": entries,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal lobby update: %v", err)
		return nil
	}
	return data
}

// notifyMatchLocal attempts to deliver a match notification to a locally
// connected matchmaking client. Returns true if the client was found.
func (h *WebSocketHandler) notifyMatchLocal(connectionId string, sessionId string) bool {
	h.hub.mmMu.RLock()
	conn, ok := h.hub.matchmakingClients[connectionId]
	h.hub.mmMu.RUnlock()

	if !ok {
		return false
	}

	msg := map[string]string{
		"type":      "match_found",
		"sessionId": sessionId,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal match notification: %v", err)
		return false
	}

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("Failed to send match notification to %s: %v", connectionId, err)
		return false
	}
	return true
}
