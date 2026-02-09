package handlers

import (
	"encoding/json"
	"net/http"

	"chess-game/internal/db"
	"chess-game/internal/matchmaking"
	"chess-game/internal/middleware"
	"chess-game/internal/models"
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
	ConnectionID   string  `json:"connectionId"`
	DisplayName    string  `json:"displayName"`
	AgentName      string  `json:"agentName,omitempty"`
	IsRanked       bool    `json:"isRanked"`
	PreferredColor *string `json:"preferredColor,omitempty"` // "white", "black", or null
	OpponentType   string  `json:"opponentType"`              // "human", "ai", or "either"
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

	// Create queue entry
	entry := &models.MatchmakingQueue{
		ConnectionID: req.ConnectionID,
		DisplayName:  req.DisplayName,
		AgentName:    req.AgentName,
		IsRanked:     req.IsRanked,
		CurrentElo:   models.DefaultEloRating,
		OpponentType: models.OpponentType(req.OpponentType),
	}

	if user != nil {
		entry.UserID = &user.ID
		entry.CurrentElo = user.EloRating
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
		Position:      1, // TODO: Calculate actual position
		EstimatedWait: "Finding opponent...",
		Status:        string(entry.Status),
	}

	// If matched, find the game session ID
	if entry.Status == models.QueueStatusMatched {
		// TODO: Retrieve the matched game session ID from the game
		response.EstimatedWait = "Match found!"
	}

	respondWithJSON(w, http.StatusOK, response)
}
