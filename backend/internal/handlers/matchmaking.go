package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"chess-game/internal/db"
	"chess-game/internal/game"
	"chess-game/internal/matchmaking"
	"chess-game/internal/middleware"
	"chess-game/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MatchmakingHandler struct {
	db    *db.MongoDB
	queue *matchmaking.Queue
}

func NewMatchmakingHandler(database *db.MongoDB, queue *matchmaking.Queue) *MatchmakingHandler {
	return &MatchmakingHandler{
		db:    database,
		queue: queue,
	}
}

type JoinQueueRequest struct {
	ConnectionID   string   `json:"connectionId"`
	DisplayName    string   `json:"displayName"`
	AgentName      string   `json:"agentName,omitempty"`
	EngineName     string   `json:"engineName,omitempty"`
	ClientSoftware string   `json:"clientSoftware,omitempty"`
	IsRanked       bool     `json:"isRanked"`
	PreferredColor *string  `json:"preferredColor,omitempty"` // "white", "black", or null
	OpponentType   string   `json:"opponentType"`             // "human", "ai", or "either"
	TimeControls   []string `json:"timeControls,omitempty"`   // ["unlimited", "standard", "blitz", etc.]
}

type QueueStatusResponse struct {
	Position         int    `json:"position"`
	EstimatedWait    string `json:"estimatedWait"`
	Status           string `json:"status"`
	MatchedSessionID string `json:"matchedSessionId,omitempty"`
}

// JoinQueue adds a player to the matchmaking queue
func (h *MatchmakingHandler) JoinQueue(w http.ResponseWriter, r *http.Request) {
	var req JoinQueueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate input
	if req.ConnectionID == "" || req.DisplayName == "" {
		respondWithError(w, http.StatusBadRequest, "Connection ID and display name are required")
		return
	}

	if req.OpponentType == "" {
		req.OpponentType = "either"
	}

	// Get user from context (optional auth)
	user, _ := middleware.GetUserFromContext(r.Context())

	// If ranked game, require authentication
	if req.IsRanked && user == nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required for ranked games")
		return
	}

	// Email verification gate: authenticated users must verify email before playing
	if user != nil && !user.EmailVerified {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Email verification required",
			"code":  "EMAIL_NOT_VERIFIED",
		})
		return
	}

	// Parse and validate time controls
	var timeControls []game.TimeControlMode
	if len(req.TimeControls) > 0 {
		for _, tc := range req.TimeControls {
			if game.IsValidTimeControlMode(tc) {
				timeControls = append(timeControls, game.TimeControlMode(tc))
			}
		}
	}
	// Default to unlimited and standard if none specified
	if len(timeControls) == 0 {
		timeControls = []game.TimeControlMode{game.TimeUnlimited, game.TimeStandard}
	}

	// Create queue entry
	entry := &models.MatchmakingQueue{
		ConnectionID:   req.ConnectionID,
		DisplayName:    req.DisplayName,
		AgentName:      req.AgentName,
		EngineName:     req.EngineName,
		ClientSoftware: req.ClientSoftware,
		IsRanked:       req.IsRanked,
		CurrentElo:     models.DefaultEloRating,
		OpponentType:   models.OpponentType(req.OpponentType),
		TimeControls:   timeControls,
	}

	if user != nil {
		entry.UserID = &user.ID
		entry.CurrentElo = user.EloRating

		// For agents, use their agent-specific Elo if they have one
		if req.AgentName != "" {
			var agentRating models.AgentRating
			err := h.db.AgentRatings().FindOne(r.Context(), bson.M{
				"ownerUserId": user.ID,
				"agentName":   req.AgentName,
			}).Decode(&agentRating)
			if err == nil {
				entry.CurrentElo = agentRating.EloRating
			}
		}
	}

	if req.PreferredColor != nil {
		color := models.PlayerColor(*req.PreferredColor)
		entry.PreferredColor = &color
	}

	// Add to queue
	if err := h.queue.AddToQueue(r.Context(), entry); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to join queue")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Successfully joined matchmaking queue",
		"queueId": entry.ID.Hex(),
	})
}

// LeaveQueue removes a player from the matchmaking queue
func (h *MatchmakingHandler) LeaveQueue(w http.ResponseWriter, r *http.Request) {
	connectionID := r.URL.Query().Get("connectionId")
	if connectionID == "" {
		respondWithError(w, http.StatusBadRequest, "Connection ID required")
		return
	}

	if err := h.queue.RemoveFromQueue(r.Context(), connectionID); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to leave queue")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Successfully left matchmaking queue",
	})
}

// GetQueueStatus returns the current queue status for a player
func (h *MatchmakingHandler) GetQueueStatus(w http.ResponseWriter, r *http.Request) {
	connectionID := r.URL.Query().Get("connectionId")
	if connectionID == "" {
		respondWithError(w, http.StatusBadRequest, "Connection ID required")
		return
	}

	entry, err := h.queue.GetQueueStatus(r.Context(), connectionID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Queue entry not found")
		return
	}

	response := QueueStatusResponse{
		Position:      h.queue.GetQueuePosition(r.Context(), entry),
		EstimatedWait: "Finding opponent...",
		Status:        string(entry.Status),
	}

	// If matched, include the game session ID
	if entry.Status == models.QueueStatusMatched {
		response.EstimatedWait = "Match found!"
		response.MatchedSessionID = entry.MatchedSessionID
	}

	respondWithJSON(w, http.StatusOK, response)
}

// LobbyEntry represents a player waiting in the matchmaking queue (public view)
type LobbyEntry struct {
	DisplayName    string                 `json:"displayName"`
	AgentName      string                 `json:"agentName,omitempty"`
	EngineName     string                 `json:"engineName,omitempty"`
	IsRanked       bool                   `json:"isRanked"`
	CurrentElo     int                    `json:"currentElo"`
	OpponentType   string                 `json:"opponentType"`
	TimeControls   []game.TimeControlMode `json:"timeControls"`
	PreferredColor *string                `json:"preferredColor,omitempty"`
	WaitingSince   time.Time              `json:"waitingSince"`
}

// GetLobby returns all players currently waiting in the matchmaking queue
func (h *MatchmakingHandler) GetLobby(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	filter := bson.M{"status": string(models.QueueStatusWaiting)}
	opts := options.Find().SetSort(bson.M{"joinedAt": 1})

	cursor, err := h.db.MatchmakingQueue().Find(ctx, filter, opts)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch lobby")
		return
	}
	defer cursor.Close(ctx)

	entries := []LobbyEntry{}
	for cursor.Next(ctx) {
		var q models.MatchmakingQueue
		if err := cursor.Decode(&q); err != nil {
			continue
		}
		entry := LobbyEntry{
			DisplayName:  q.DisplayName,
			AgentName:    q.AgentName,
			EngineName:   q.EngineName,
			IsRanked:     q.IsRanked,
			CurrentElo:   q.CurrentElo,
			OpponentType: string(q.OpponentType),
			TimeControls: q.TimeControls,
			WaitingSince: q.JoinedAt,
		}
		if q.PreferredColor != nil {
			s := string(*q.PreferredColor)
			entry.PreferredColor = &s
		}
		entries = append(entries, entry)
	}

	respondWithJSON(w, http.StatusOK, entries)
}
