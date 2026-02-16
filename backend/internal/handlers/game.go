package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"chess-game/internal/db"
	"chess-game/internal/game"
	"chess-game/internal/middleware"
	"chess-game/internal/models"
	"chess-game/internal/services"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AgentTurnNotifier is called when it becomes an agent's turn to move.
type AgentTurnNotifier func(sessionID string)

type GameHandler struct {
	db                    *db.MongoDB
	ws                    *WebSocketHandler
	gameCompletionService *services.GameCompletionService
	agentTurnNotifier     AgentTurnNotifier
	maxPositionHistory    int
}

func NewGameHandler(database *db.MongoDB, wsHandler *WebSocketHandler, completionService *services.GameCompletionService, maxPositionHistory int) *GameHandler {
	if maxPositionHistory <= 0 {
		maxPositionHistory = 300
	}
	return &GameHandler{
		db:                    database,
		ws:                    wsHandler,
		gameCompletionService: completionService,
		maxPositionHistory:    maxPositionHistory,
	}
}

// authorizePlayer verifies that the authenticated user (if any) is allowed to
// act as the given playerID. Returns the player, or writes an error response
// and returns nil.
func (h *GameHandler) authorizePlayer(w http.ResponseWriter, r *http.Request, game *models.Game, playerID string) *models.Player {
	var player *models.Player
	for i := range game.Players {
		if game.Players[i].ID == playerID {
			player = &game.Players[i]
			break
		}
	}
	if player == nil {
		respondWithError(w, http.StatusBadRequest, "Player not in game")
		return nil
	}

	// Check authorization: if the player slot has a UserID, the authenticated
	// user must match. Anonymous casual-game players (no UserID) can act freely.
	if player.UserID != nil {
		user, ok := middleware.GetUserFromContext(r.Context())
		if !ok || user.ID != *player.UserID {
			respondWithError(w, http.StatusForbidden, "Not authorized to act as this player")
			return nil
		}
	}

	return player
}

// SetAgentTurnNotifier registers a callback invoked when it's an agent's turn.
func (h *GameHandler) SetAgentTurnNotifier(fn AgentTurnNotifier) {
	h.agentTurnNotifier = fn
}

type CreateGameRequest struct {
	TimeControl string `json:"timeControl,omitempty"` // Time control mode (unlimited, casual, standard, quick, blitz, tournament)
}

type CreateGameResponse struct {
	SessionID string `json:"sessionId"`
	PlayerID  string `json:"playerId"`
	ShareLink string `json:"shareLink"`
}

type JoinGameResponse struct {
	SessionID  string             `json:"sessionId"`
	PlayerID   string             `json:"playerId"`
	Color      models.PlayerColor `json:"color"`
	Game       *models.Game       `json:"game"`
	ServerTime int64              `json:"serverTime"` // Server timestamp in milliseconds for clock sync
}

type MakeMoveRequest struct {
	PlayerID  string `json:"playerId"`
	From      string `json:"from"`
	To        string `json:"to"`
	Promotion string `json:"promotion,omitempty"`
}

type MakeMoveResponse struct {
	Success    bool          `json:"success"`
	Move       *models.Move  `json:"move,omitempty"`
	BoardState string        `json:"boardState"`
	Check      bool          `json:"check"`
	Checkmate  bool          `json:"checkmate"`
	Stalemate  bool          `json:"stalemate"`
	Draw       bool          `json:"draw,omitempty"`
	DrawReason string        `json:"drawReason,omitempty"`
	Error      string        `json:"error,omitempty"`
}

type GetMovesResponse struct {
	Moves []models.Move `json:"moves"`
}

func generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func (h *GameHandler) CreateGame(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Email verification gate: authenticated users must verify email before playing
	if user, ok := middleware.GetUserFromContext(r.Context()); ok && !user.EmailVerified {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Email verification required",
			"code":  "EMAIL_NOT_VERIFIED",
		})
		return
	}

	// Parse optional request body for time control
	var req CreateGameRequest
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req) // Ignore errors - optional body
	}

	sessionID := generateID()
	playerID := generateID()

	// Determine time control
	timeControlMode := game.TimeUnlimited
	if req.TimeControl != "" && game.IsValidTimeControlMode(req.TimeControl) {
		timeControlMode = game.TimeControlMode(req.TimeControl)
	}
	timeControl := game.GetTimeControl(timeControlMode)

	// Initialize player times based on time control
	var playerTimes *game.PlayerTimes
	if !timeControl.IsUnlimited() {
		playerTimes = &game.PlayerTimes{
			White: game.PlayerTime{RemainingMs: timeControl.BaseTimeMs, LastMoveAt: 0},
			Black: game.PlayerTime{RemainingMs: timeControl.BaseTimeMs, LastMoveAt: 0},
		}
	}

	newGame := &models.Game{
		SessionID:       sessionID,
		Players:         []models.Player{{ID: playerID, Color: models.White, JoinedAt: time.Now()}},
		Status:          models.GameStatusWaiting,
		CurrentTurn:     models.White,
		BoardState:      models.InitialBoardFEN,
		TimeControl:     &timeControl,
		PlayerTimes:     playerTimes,
		DrawOffers:      &game.DrawOffers{},
		PositionHistory: []string{models.InitialBoardFEN},
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	_, err := h.db.Games().InsertOne(ctx, newGame)
	if err != nil {
		http.Error(w, "Failed to create game", http.StatusInternalServerError)
		return
	}

	response := CreateGameResponse{
		SessionID: sessionID,
		PlayerID:  playerID,
		ShareLink: "/game/" + sessionID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *GameHandler) JoinGame(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Email verification gate: authenticated users must verify email before playing
	if user, ok := middleware.GetUserFromContext(r.Context()); ok && !user.EmailVerified {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Email verification required",
			"code":  "EMAIL_NOT_VERIFIED",
		})
		return
	}

	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	var existingGame models.Game
	err := h.db.Games().FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&existingGame)
	if err != nil {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	// Check if player is already in the game
	var playerIDHeader = r.Header.Get("X-Player-ID")
	for _, p := range existingGame.Players {
		if p.ID == playerIDHeader {
			// Return game with server time - frontend calculates actual remaining time
			response := JoinGameResponse{
				SessionID:  sessionID,
				PlayerID:   p.ID,
				Color:      p.Color,
				Game:       &existingGame,
				ServerTime: time.Now().UnixMilli(),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// Game full
	if len(existingGame.Players) >= 2 {
		http.Error(w, "Game is full", http.StatusBadRequest)
		return
	}

	// Add second player
	playerID := generateID()
	newPlayer := models.Player{
		ID:       playerID,
		Color:    models.Black,
		JoinedAt: time.Now(),
	}

	now := time.Now()
	nowMs := now.UnixMilli()

	// Set up the update - include startedAt and initialize player time lastMoveAt
	updateSet := bson.M{
		"status":    models.GameStatusActive,
		"startedAt": now,
		"updatedAt": now,
	}

	// Initialize lastMoveAt for player times when game starts (for timed games)
	if existingGame.PlayerTimes != nil {
		updateSet["playerTimes.white.lastMoveAt"] = nowMs
		updateSet["playerTimes.black.lastMoveAt"] = nowMs
	}

	update := bson.M{
		"$push": bson.M{"players": newPlayer},
		"$set":  updateSet,
	}

	_, err = h.db.Games().UpdateOne(ctx, bson.M{"sessionId": sessionID}, update)
	if err != nil {
		http.Error(w, "Failed to join game", http.StatusInternalServerError)
		return
	}

	// Fetch updated game
	err = h.db.Games().FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&existingGame)
	if err != nil {
		http.Error(w, "Failed to fetch game", http.StatusInternalServerError)
		return
	}

	response := JoinGameResponse{
		SessionID:  sessionID,
		PlayerID:   playerID,
		Color:      models.Black,
		Game:       &existingGame,
		ServerTime: time.Now().UnixMilli(),
	}

	// Broadcast player joined to existing players
	if h.ws != nil {
		h.ws.BroadcastPlayerJoined(sessionID, &existingGame)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GameWithServerTime wraps game data with server timestamp for clock sync
type GameWithServerTime struct {
	*models.Game
	ServerTime int64 `json:"serverTime"` // Server timestamp in milliseconds for clock sync
}

// enrichDrawClaimStatus computes whether threefold repetition or fifty-move
// draw claims are currently available and sets the computed fields on the game.
func enrichDrawClaimStatus(g *models.Game) {
	if g.Status != models.GameStatusActive {
		return
	}
	// Threefold repetition: current position appeared 3+ times
	if game.IsThreefoldRepetition(g.PositionHistory, g.BoardState) {
		g.CanClaimThreefold = true
	}
	// Fifty-move rule: halfmove clock >= 100
	if board, err := game.ParseFEN(g.BoardState); err == nil {
		if game.IsFiftyMoveRule(board.HalfMoveClock) {
			g.CanClaimFiftyMoves = true
		}
	}
}

func (h *GameHandler) GetGame(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	var existingGame models.Game
	err := h.db.Games().FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&existingGame)
	if err != nil {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	// Check for timeout on active timed games — detect expired clocks on read
	if existingGame.Status == models.GameStatusActive &&
		existingGame.TimeControl != nil && !existingGame.TimeControl.IsUnlimited() && existingGame.PlayerTimes != nil {
		nowMs := time.Now().UnixMilli()
		var currentPlayerTime *game.PlayerTime
		if existingGame.CurrentTurn == models.White {
			currentPlayerTime = &existingGame.PlayerTimes.White
		} else {
			currentPlayerTime = &existingGame.PlayerTimes.Black
		}
		if currentPlayerTime.LastMoveAt > 0 {
			elapsedMs := nowMs - currentPlayerTime.LastMoveAt
			remainingMs := currentPlayerTime.RemainingMs - elapsedMs
			if remainingMs <= 0 {
				if updated, err := h.processTimeout(ctx, &existingGame, existingGame.CurrentTurn); err == nil && updated != nil {
					existingGame = *updated
				}
			}
		}
	}

	enrichDrawClaimStatus(&existingGame)

	// Return game with server time - frontend will calculate actual remaining time as:
	// actualRemaining = remainingMs - (serverTime - lastMoveAt) for the active player
	response := GameWithServerTime{
		Game:       &existingGame,
		ServerTime: time.Now().UnixMilli(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *GameHandler) MakeMove(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	var req MakeMoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Fetch the game
	var existingGame models.Game
	err := h.db.Games().FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&existingGame)
	if err != nil {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	// Verify player is in the game and authorized
	player := h.authorizePlayer(w, r, &existingGame, req.PlayerID)
	if player == nil {
		return
	}

	// Verify it's player's turn
	if player.Color != existingGame.CurrentTurn {
		// Before returning "not your turn", check if the opponent's clock has expired
		if existingGame.TimeControl != nil && !existingGame.TimeControl.IsUnlimited() && existingGame.PlayerTimes != nil {
			nowMs := time.Now().UnixMilli()
			var opponentTime *game.PlayerTime
			if existingGame.CurrentTurn == models.White {
				opponentTime = &existingGame.PlayerTimes.White
			} else {
				opponentTime = &existingGame.PlayerTimes.Black
			}
			if opponentTime.LastMoveAt > 0 {
				elapsedMs := nowMs - opponentTime.LastMoveAt
				remainingMs := opponentTime.RemainingMs - elapsedMs
				if remainingMs <= 0 {
					h.endGameByTimeout(ctx, w, &existingGame, existingGame.CurrentTurn)
					return
				}
			}
		}
		respondWithError(w, http.StatusBadRequest, "Not your turn")
		return
	}

	nowMs := time.Now().UnixMilli()

	// Check for timeout (if timed game)
	if existingGame.TimeControl != nil && !existingGame.TimeControl.IsUnlimited() && existingGame.PlayerTimes != nil {
		var playerTime *game.PlayerTime
		if player.Color == models.White {
			playerTime = &existingGame.PlayerTimes.White
		} else {
			playerTime = &existingGame.PlayerTimes.Black
		}

		// Calculate remaining time
		elapsedMs := int64(0)
		if playerTime.LastMoveAt > 0 {
			elapsedMs = nowMs - playerTime.LastMoveAt
		}
		remainingMs := playerTime.RemainingMs - elapsedMs

		if remainingMs <= 0 {
			// Player timed out - end game
			h.endGameByTimeout(ctx, w, &existingGame, player.Color)
			return
		}
	}

	// Parse board state
	board, err := game.ParseFEN(existingGame.BoardState)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Invalid board state")
		return
	}

	// Parse positions
	fromPos, err := game.ParsePosition(req.From)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid from position")
		return
	}
	toPos, err := game.ParsePosition(req.To)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid to position")
		return
	}

	// Validate move
	if err := board.ValidateMove(fromPos, toPos); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Validate and parse promotion piece
	var promotionRune rune
	if req.Promotion != "" {
		p := strings.ToLower(req.Promotion)
		if p != "q" && p != "r" && p != "b" && p != "n" {
			respondWithError(w, http.StatusBadRequest, "Invalid promotion piece")
			return
		}
		promotionRune = rune(p[0])
	}
	notation := board.GenerateNotation(fromPos, toPos, promotionRune)

	// Make the move
	piece := board.GetPiece(fromPos)
	isCapture := board.GetPiece(toPos) != 0 || toPos.String() == board.EnPassantSquare
	newBoard := board.MakeMove(fromPos, toPos, promotionRune)
	newFEN := newBoard.ToFEN()

	// Use the game's stored move count (avoids a separate CountDocuments query)
	moveCount := existingGame.MoveCount

	// Create move record
	move := &models.Move{
		GameID:     existingGame.ID,
		SessionID:  sessionID,
		PlayerID:   req.PlayerID,
		MoveNumber: moveCount + 1,
		From:       req.From,
		To:         req.To,
		Piece:      string(piece),
		Notation:   notation,
		Capture:    isCapture,
		Check:      newBoard.IsInCheck(!board.WhiteToMove),
		Checkmate:  newBoard.IsCheckmate(),
		Promotion:  req.Promotion,
		CreatedAt:  time.Now(),
	}

	_, err = h.db.Moves().InsertOne(ctx, move)
	if err != nil {
		http.Error(w, "Failed to record move", http.StatusInternalServerError)
		return
	}

	// Update game state
	nextTurn := models.White
	if existingGame.CurrentTurn == models.White {
		nextTurn = models.Black
	}

	status := existingGame.Status
	var drawReason game.DrawReason
	updateFields := bson.M{
		"boardState":  newFEN,
		"currentTurn": nextTurn,
		"status":      status,
		"updatedAt":   time.Now(),
	}

	// Add position to history for repetition detection
	positionHistory := existingGame.PositionHistory
	if positionHistory == nil {
		positionHistory = []string{}
	}
	positionHistory = append(positionHistory, newFEN)
	// Cap position history to prevent unbounded growth
	if h.maxPositionHistory > 0 && len(positionHistory) > h.maxPositionHistory {
		positionHistory = positionHistory[len(positionHistory)-h.maxPositionHistory:]
	}
	updateFields["positionHistory"] = positionHistory

	// Clear any pending draw offer when a move is made
	if existingGame.DrawOffers != nil && existingGame.DrawOffers.PendingFrom != "" {
		updateFields["drawOffers.pendingFrom"] = ""
	}

	// Update player times for timed games
	if existingGame.TimeControl != nil && !existingGame.TimeControl.IsUnlimited() && existingGame.PlayerTimes != nil {
		var playerTimeField string
		var opponentTimeField string
		var playerTime game.PlayerTime

		if player.Color == models.White {
			playerTimeField = "playerTimes.white"
			opponentTimeField = "playerTimes.black"
			playerTime = existingGame.PlayerTimes.White
		} else {
			playerTimeField = "playerTimes.black"
			opponentTimeField = "playerTimes.white"
			playerTime = existingGame.PlayerTimes.Black
		}

		// Calculate time used
		elapsedMs := int64(0)
		if playerTime.LastMoveAt > 0 {
			elapsedMs = nowMs - playerTime.LastMoveAt
		}

		// Deduct time used and add increment
		newRemainingMs := playerTime.RemainingMs - elapsedMs + existingGame.TimeControl.IncrementMs
		if newRemainingMs < 0 {
			newRemainingMs = 0
		}

		updateFields[playerTimeField+".remainingMs"] = newRemainingMs
		updateFields[playerTimeField+".lastMoveAt"] = nowMs

		// Set opponent's lastMoveAt to now (their clock starts)
		updateFields[opponentTimeField+".lastMoveAt"] = nowMs
	}

	// Check for game end conditions
	if newBoard.IsCheckmate() {
		status = models.GameStatusComplete
		updateFields["status"] = status
		updateFields["winner"] = existingGame.CurrentTurn // Current player (who just moved) wins
		updateFields["winReason"] = "checkmate"
		now := time.Now()
		updateFields["completedAt"] = now
	} else if newBoard.IsStalemate() {
		status = models.GameStatusComplete
		updateFields["status"] = status
		updateFields["winReason"] = "stalemate"
		drawReason = game.DrawByStalemate
		now := time.Now()
		updateFields["completedAt"] = now
	} else {
		// Check for automatic draw conditions
		if isDraw, reason := game.IsAutomaticDraw(newBoard, positionHistory, newFEN); isDraw {
			status = models.GameStatusComplete
			updateFields["status"] = status
			updateFields["winReason"] = string(reason)
			drawReason = reason
			now := time.Now()
			updateFields["completedAt"] = now
		}
	}

	update := bson.M{
		"$set": updateFields,
		"$inc": bson.M{"moveCount": 1},
	}

	// Atomic update: only succeeds if currentTurn still matches (prevents race conditions)
	filter := bson.M{
		"sessionId":   sessionID,
		"currentTurn": existingGame.CurrentTurn,
		"moveCount":   existingGame.MoveCount,
	}

	var updatedGame models.Game
	err = h.db.Games().FindOneAndUpdate(ctx, filter, update,
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updatedGame)
	if err != nil {
		// If no document matched, another move was processed first (race condition prevented)
		respondWithError(w, http.StatusConflict, "Move conflict - game state changed, please retry")
		return
	}
	enrichDrawClaimStatus(&updatedGame)

	// Process game completion for ranked games (Elo updates, match history)
	if status == models.GameStatusComplete && h.gameCompletionService != nil {
		eloResult, err := h.gameCompletionService.ProcessGameCompletion(ctx, &updatedGame)
		if err != nil {
			log.Printf("Failed to process game completion for %s: %v", sessionID, err)
		} else if eloResult != nil {
			// Re-fetch to get EloChanges written by completion service
			h.db.Games().FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&updatedGame)
		}
	}

	// Broadcast move to other player
	if h.ws != nil {
		h.ws.BroadcastMove(sessionID, &updatedGame, move, req.PlayerID)
	}

	// Notify builtin agent if it's now an agent's turn
	if h.agentTurnNotifier != nil && updatedGame.Status == models.GameStatusActive {
		for _, p := range updatedGame.Players {
			if p.Color == updatedGame.CurrentTurn && p.AgentName != "" {
				go h.agentTurnNotifier(sessionID)
				break
			}
		}
	}

	response := MakeMoveResponse{
		Success:    true,
		Move:       move,
		BoardState: newFEN,
		Check:      move.Check,
		Checkmate:  move.Checkmate,
		Stalemate:  newBoard.IsStalemate(),
		Draw:       drawReason != "",
		DrawReason: string(drawReason),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// processTimeout handles the core logic of ending a game by timeout: updates DB,
// processes game completion, and broadcasts. Returns the updated game or an error.
func (h *GameHandler) processTimeout(ctx context.Context, existingGame *models.Game, timedOutColor models.PlayerColor) (*models.Game, error) {
	var winnerColor models.PlayerColor
	if timedOutColor == models.White {
		winnerColor = models.Black
	} else {
		winnerColor = models.White
	}

	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"status":      models.GameStatusComplete,
			"winner":      winnerColor,
			"winReason":   "timeout",
			"completedAt": now,
			"updatedAt":   now,
		},
	}

	_, err := h.db.Games().UpdateOne(ctx, bson.M{"sessionId": existingGame.SessionID}, update)
	if err != nil {
		return nil, err
	}

	var updatedGame models.Game
	h.db.Games().FindOne(ctx, bson.M{"sessionId": existingGame.SessionID}).Decode(&updatedGame)

	if h.gameCompletionService != nil {
		h.gameCompletionService.ProcessGameCompletion(ctx, &updatedGame)
		h.db.Games().FindOne(ctx, bson.M{"sessionId": existingGame.SessionID}).Decode(&updatedGame)
	}

	if h.ws != nil {
		h.ws.BroadcastGameOver(existingGame.SessionID, &updatedGame, string(winnerColor), "timeout")
	}

	return &updatedGame, nil
}

// endGameByTimeout handles ending the game when a player times out (called from MakeMove).
func (h *GameHandler) endGameByTimeout(ctx context.Context, w http.ResponseWriter, existingGame *models.Game, timedOutColor models.PlayerColor) {
	_, err := h.processTimeout(ctx, existingGame, timedOutColor)
	if err != nil {
		http.Error(w, "Failed to update game", http.StatusInternalServerError)
		return
	}
	respondWithError(w, http.StatusBadRequest, "Time expired")
}

func (h *GameHandler) GetMoves(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	opts := options.Find().SetSort(bson.M{"moveNumber": 1})
	cursor, err := h.db.Moves().Find(ctx, bson.M{"sessionId": sessionID}, opts)
	if err != nil {
		http.Error(w, "Failed to fetch moves", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	moves := []models.Move{}
	if err := cursor.All(ctx, &moves); err != nil {
		http.Error(w, "Failed to decode moves", http.StatusInternalServerError)
		return
	}

	response := GetMovesResponse{Moves: moves}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// respondWithError is defined in auth.go

type ResignRequest struct {
	PlayerID string `json:"playerId"`
}

type ResignResponse struct {
	Success bool   `json:"success"`
	Winner  string `json:"winner,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Draw-related request/response types
type DrawOfferRequest struct {
	PlayerID string `json:"playerId"`
}

type DrawOfferResponse struct {
	Success         bool   `json:"success"`
	OffersRemaining int    `json:"offersRemaining,omitempty"`
	AutoDeclined    bool   `json:"autoDeclined,omitempty"`
	AutoDeclineMsg  string `json:"autoDeclineMessage,omitempty"`
	Error           string `json:"error,omitempty"`
}

type DrawResponseRequest struct {
	PlayerID string `json:"playerId"`
	Accept   bool   `json:"accept"`
}

type DrawResponseResp struct {
	Success bool   `json:"success"`
	Draw    bool   `json:"draw,omitempty"`
	Error   string `json:"error,omitempty"`
}

type ClaimDrawRequest struct {
	PlayerID string `json:"playerId"`
	Reason   string `json:"reason"` // "threefold" or "fifty_moves"
}

type ClaimDrawResponse struct {
	Success bool   `json:"success"`
	Draw    bool   `json:"draw,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (h *GameHandler) ResignGame(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	var req ResignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Fetch the game
	var existingGame models.Game
	err := h.db.Games().FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&existingGame)
	if err != nil {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	// Verify game is active
	if existingGame.Status != models.GameStatusActive {
		response := ResignResponse{Success: false, Error: "Game is not active"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Verify player is in the game and authorized
	resigningPlayer := h.authorizePlayer(w, r, &existingGame, req.PlayerID)
	if resigningPlayer == nil {
		return
	}

	// Winner is the opposite color
	var winnerColor models.PlayerColor
	if resigningPlayer.Color == models.White {
		winnerColor = models.Black
	} else {
		winnerColor = models.White
	}

	// Update game with resignation
	update := bson.M{
		"$set": bson.M{
			"status":    models.GameStatusComplete,
			"winner":    winnerColor,
			"winReason": "resignation",
			"updatedAt": time.Now(),
		},
	}

	_, err = h.db.Games().UpdateOne(ctx, bson.M{"sessionId": sessionID}, update)
	if err != nil {
		http.Error(w, "Failed to update game", http.StatusInternalServerError)
		return
	}

	// Record resignation as a move (for move history)
	moveCount, err := h.db.Moves().CountDocuments(ctx, bson.M{"sessionId": sessionID})
	if err != nil {
		log.Printf("Failed to count moves for game %s: %v", sessionID, err)
	}
	resignMove := &models.Move{
		GameID:     existingGame.ID,
		SessionID:  sessionID,
		PlayerID:   req.PlayerID,
		MoveNumber: int(moveCount) + 1,
		From:       "",
		To:         "",
		Piece:      "",
		Notation:   string(resigningPlayer.Color) + " resigns",
		Capture:    false,
		Check:      false,
		Checkmate:  false,
		CreatedAt:  time.Now(),
	}
	h.db.Moves().InsertOne(ctx, resignMove)

	// Fetch updated game for broadcast
	var updatedGame models.Game
	h.db.Games().FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&updatedGame)

	// Process game completion for ranked games (Elo updates, match history)
	if h.gameCompletionService != nil {
		eloResult, err := h.gameCompletionService.ProcessGameCompletion(ctx, &updatedGame)
		if err != nil {
			log.Printf("Failed to process game completion for %s: %v", sessionID, err)
		} else if eloResult != nil {
			// Re-fetch game to get updated EloChanges
			h.db.Games().FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&updatedGame)
		}
	}

	// Broadcast resignation to other player
	if h.ws != nil {
		h.ws.BroadcastResignation(sessionID, &updatedGame, string(resigningPlayer.Color), req.PlayerID)
	}

	response := ResignResponse{
		Success: true,
		Winner:  string(winnerColor),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// OfferDraw handles a player offering a draw
func (h *GameHandler) OfferDraw(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	var req DrawOfferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Fetch the game
	var existingGame models.Game
	err := h.db.Games().FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&existingGame)
	if err != nil {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	// Verify game is active
	if existingGame.Status != models.GameStatusActive {
		response := DrawOfferResponse{Success: false, Error: "Game is not active"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Verify player is in the game and authorized
	player := h.authorizePlayer(w, r, &existingGame, req.PlayerID)
	if player == nil {
		return
	}

	// Check draw offer count
	if existingGame.DrawOffers == nil {
		existingGame.DrawOffers = &game.DrawOffers{}
	}

	var offersUsed int
	if player.Color == models.White {
		offersUsed = existingGame.DrawOffers.WhiteOffers
	} else {
		offersUsed = existingGame.DrawOffers.BlackOffers
	}

	if offersUsed >= game.MaxDrawOffers {
		response := DrawOfferResponse{Success: false, Error: "Maximum draw offers reached"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Check if there's already a pending offer
	if existingGame.DrawOffers.PendingFrom != "" {
		response := DrawOfferResponse{Success: false, Error: "Draw offer already pending"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Update draw offers
	updateFields := bson.M{
		"drawOffers.pendingFrom": string(player.Color),
		"updatedAt":              time.Now(),
	}
	if player.Color == models.White {
		updateFields["drawOffers.whiteOffers"] = offersUsed + 1
	} else {
		updateFields["drawOffers.blackOffers"] = offersUsed + 1
	}

	_, err = h.db.Games().UpdateOne(ctx, bson.M{"sessionId": sessionID}, bson.M{"$set": updateFields})
	if err != nil {
		http.Error(w, "Failed to update game", http.StatusInternalServerError)
		return
	}

	// Fetch updated game for broadcast
	var updatedDrawGame models.Game
	h.db.Games().FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&updatedDrawGame)

	// Find the opponent player
	var opponent *models.Player
	for _, p := range existingGame.Players {
		if p.ID != req.PlayerID {
			opponent = &p
			break
		}
	}

	// Check if opponent has auto-decline draws enabled
	autoDeclined := false
	autoDeclineMsg := ""
	if opponent != nil && opponent.UserID != nil {
		var opponentUser models.User
		if err := h.db.Users().FindOne(ctx, bson.M{"_id": *opponent.UserID}).Decode(&opponentUser); err == nil {
			if opponentUser.Preferences != nil && opponentUser.Preferences.AutoDeclineDraws {
				autoDeclined = true
				autoDeclineMsg = "Your opponent has draw offers set to auto-decline."
			}
		}
	}

	if autoDeclined {
		// Auto-decline: clear the pending offer immediately
		h.db.Games().UpdateOne(ctx, bson.M{"sessionId": sessionID}, bson.M{
			"$set": bson.M{
				"drawOffers.pendingFrom": "",
				"updatedAt":              time.Now(),
			},
		})

		// Fetch game after clearing and broadcast declined
		var declinedGame models.Game
		h.db.Games().FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&declinedGame)

		if h.ws != nil {
			h.ws.BroadcastDrawDeclined(sessionID, &declinedGame, true)
		}

		response := DrawOfferResponse{
			Success:         true,
			OffersRemaining: game.MaxDrawOffers - (offersUsed + 1),
			AutoDeclined:    true,
			AutoDeclineMsg:  autoDeclineMsg,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Broadcast draw offer to opponent
	if h.ws != nil {
		h.ws.BroadcastDrawOffer(sessionID, string(player.Color), &updatedDrawGame)
	}

	response := DrawOfferResponse{
		Success:         true,
		OffersRemaining: game.MaxDrawOffers - (offersUsed + 1),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// RespondToDraw handles accepting or declining a draw offer
func (h *GameHandler) RespondToDraw(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	var req DrawResponseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Fetch the game
	var existingGame models.Game
	err := h.db.Games().FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&existingGame)
	if err != nil {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	// Verify game is active
	if existingGame.Status != models.GameStatusActive {
		response := DrawResponseResp{Success: false, Error: "Game is not active"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Check if there's a pending offer
	if existingGame.DrawOffers == nil || existingGame.DrawOffers.PendingFrom == "" {
		response := DrawResponseResp{Success: false, Error: "No draw offer pending"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Verify player is in the game and authorized
	player := h.authorizePlayer(w, r, &existingGame, req.PlayerID)
	if player == nil {
		return
	}

	// Verify it's the opponent responding
	if string(player.Color) == existingGame.DrawOffers.PendingFrom {
		response := DrawResponseResp{Success: false, Error: "Cannot respond to your own draw offer"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	if req.Accept {
		// Accept draw - end game
		now := time.Now()
		update := bson.M{
			"$set": bson.M{
				"status":             models.GameStatusComplete,
				"winReason":          string(game.DrawByAgreement),
				"drawOffers.pendingFrom": "",
				"completedAt":        now,
				"updatedAt":          now,
			},
		}

		_, err = h.db.Games().UpdateOne(ctx, bson.M{"sessionId": sessionID}, update)
		if err != nil {
			http.Error(w, "Failed to update game", http.StatusInternalServerError)
			return
		}

		// Fetch updated game
		var updatedGame models.Game
		h.db.Games().FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&updatedGame)

		// Process game completion
		if h.gameCompletionService != nil {
			h.gameCompletionService.ProcessGameCompletion(ctx, &updatedGame)
		}

		// Broadcast draw accepted
		if h.ws != nil {
			h.ws.BroadcastGameOver(sessionID, &updatedGame, "", string(game.DrawByAgreement))
		}

		response := DrawResponseResp{Success: true, Draw: true}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	} else {
		// Decline draw
		update := bson.M{
			"$set": bson.M{
				"drawOffers.pendingFrom": "",
				"updatedAt":              time.Now(),
			},
		}

		_, err = h.db.Games().UpdateOne(ctx, bson.M{"sessionId": sessionID}, update)
		if err != nil {
			http.Error(w, "Failed to update game", http.StatusInternalServerError)
			return
		}

		// Fetch updated game for broadcast
		var declinedGame models.Game
		h.db.Games().FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&declinedGame)

		// Broadcast draw declined
		if h.ws != nil {
			h.ws.BroadcastDrawDeclined(sessionID, &declinedGame, false)
		}

		response := DrawResponseResp{Success: true, Draw: false}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// ClaimDraw handles claiming a draw (threefold repetition or 50-move rule)
func (h *GameHandler) ClaimDraw(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	var req ClaimDrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Fetch the game
	var existingGame models.Game
	err := h.db.Games().FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&existingGame)
	if err != nil {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	// Verify game is active
	if existingGame.Status != models.GameStatusActive {
		response := ClaimDrawResponse{Success: false, Error: "Game is not active"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Verify player is in the game and authorized
	player := h.authorizePlayer(w, r, &existingGame, req.PlayerID)
	if player == nil {
		return
	}

	// Parse the board to check the claim
	board, err := game.ParseFEN(existingGame.BoardState)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Invalid board state")
		return
	}

	var drawReason game.DrawReason
	var valid bool

	switch req.Reason {
	case "threefold", "threefold_repetition":
		if game.IsThreefoldRepetition(existingGame.PositionHistory, existingGame.BoardState) {
			valid = true
			drawReason = game.DrawByThreefoldRepetition
		}
	case "fifty_moves":
		if game.IsFiftyMoveRule(board.HalfMoveClock) {
			valid = true
			drawReason = game.DrawByFiftyMoves
		}
	default:
		response := ClaimDrawResponse{Success: false, Error: "Invalid draw reason"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	if !valid {
		response := ClaimDrawResponse{Success: false, Error: "Draw claim is not valid"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Valid claim - end game as draw
	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"status":      models.GameStatusComplete,
			"winReason":   string(drawReason),
			"completedAt": now,
			"updatedAt":   now,
		},
	}

	_, err = h.db.Games().UpdateOne(ctx, bson.M{"sessionId": sessionID}, update)
	if err != nil {
		http.Error(w, "Failed to update game", http.StatusInternalServerError)
		return
	}

	// Fetch updated game
	var updatedGame models.Game
	h.db.Games().FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&updatedGame)

	// Process game completion
	if h.gameCompletionService != nil {
		h.gameCompletionService.ProcessGameCompletion(ctx, &updatedGame)
	}

	// Broadcast game over
	if h.ws != nil {
		h.ws.BroadcastGameOver(sessionID, &updatedGame, "", string(drawReason))
	}

	response := ClaimDrawResponse{
		Success: true,
		Draw:    true,
		Reason:  string(drawReason),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ListActiveGames returns the most recent active games
func (h *GameHandler) ListActiveGames(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	limit := int64(10)
	if l, err := parseIntParam(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 50 {
		limit = int64(l)
	}

	// Inactivity filter: exclude games not updated within N minutes (default 10)
	inactiveMins := 10
	if m, err := parseIntParam(r.URL.Query().Get("inactiveMins")); err == nil && m > 0 && m <= 1440 {
		inactiveMins = m
	}
	cutoff := time.Now().Add(-time.Duration(inactiveMins) * time.Minute)

	opts := options.Find().
		SetSort(bson.M{"startedAt": -1}).
		SetLimit(limit).
		SetProjection(bson.M{
			"sessionId":   1,
			"players":     1,
			"status":      1,
			"currentTurn": 1,
			"isRanked":    1,
			"timeControl": 1,
			"startedAt":   1,
		})

	activeFilter := bson.M{
		"status":    models.GameStatusActive,
		"updatedAt": bson.M{"$gte": cutoff},
	}

	// Optional ranked filter
	rankedParam := r.URL.Query().Get("ranked")
	switch rankedParam {
	case "true":
		activeFilter["isRanked"] = true
	case "false":
		activeFilter["isRanked"] = bson.M{"$ne": true}
	}

	cursor, err := h.db.Games().Find(ctx, activeFilter, opts)
	if err != nil {
		http.Error(w, "Failed to fetch active games", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	games := []models.Game{}
	if err := cursor.All(ctx, &games); err != nil {
		http.Error(w, "Failed to decode games", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(games)
}

// ListCompletedGames returns the most recent completed games
func (h *GameHandler) ListCompletedGames(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	limit := int64(10)
	if l, err := parseIntParam(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 50 {
		limit = int64(l)
	}

	opts := options.Find().
		SetSort(bson.M{"completedAt": -1}).
		SetLimit(limit).
		SetProjection(bson.M{
			"sessionId":   1,
			"players":     1,
			"status":      1,
			"winner":      1,
			"winReason":   1,
			"isRanked":    1,
			"boardState":  1,
			"completedAt": 1,
		})

	completedFilter := bson.M{"status": models.GameStatusComplete}

	// Optional ranked filter
	rankedParam := r.URL.Query().Get("ranked")
	switch rankedParam {
	case "true":
		completedFilter["isRanked"] = true
	case "false":
		completedFilter["isRanked"] = bson.M{"$ne": true}
	}

	cursor, err := h.db.Games().Find(ctx, completedFilter, opts)
	if err != nil {
		http.Error(w, "Failed to fetch completed games", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	games := []models.Game{}
	if err := cursor.All(ctx, &games); err != nil {
		http.Error(w, "Failed to decode games", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(games)
}

// GetUserGameHistory returns paginated game history for a user.
// Ranked games are visible to anyone. Unranked games are only visible to the
// authenticated owner of that history.
func (h *GameHandler) GetUserGameHistory(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	userIdStr := vars["userId"]

	userObjID, err := primitive.ObjectIDFromHex(userIdStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Pagination params
	page := 1
	limit := 20
	if p, err := parseIntParam(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}
	if l, err := parseIntParam(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 50 {
		limit = l
	}

	resultFilter := r.URL.Query().Get("result") // "wins", "losses", "draws"
	rankedFilter := r.URL.Query().Get("ranked")  // "true", "false", or "" (all)

	// Determine if the requester is the owner of this history
	authUser, _ := middleware.GetUserFromContext(r.Context())
	isOwner := authUser != nil && authUser.ID == userObjID

	// Non-owners can only see ranked games
	if !isOwner {
		rankedFilter = "true"
	}

	// Build query using $and to avoid key conflicts in bson.M
	// Base condition: user is either white or black
	conditions := []bson.M{
		{"$or": []bson.M{
			{"whiteUserId": userObjID},
			{"blackUserId": userObjID},
		}},
	}

	// Apply ranked filter
	switch rankedFilter {
	case "true":
		conditions = append(conditions, bson.M{"isRanked": true})
	case "false":
		conditions = append(conditions, bson.M{"isRanked": bson.M{"$ne": true}})
	}

	// Apply result filter
	switch resultFilter {
	case "wins":
		conditions = append(conditions, bson.M{
			"$or": []bson.M{
				{"whiteUserId": userObjID, "winner": "white"},
				{"blackUserId": userObjID, "winner": "black"},
			},
		})
	case "losses":
		conditions = append(conditions, bson.M{
			"$or": []bson.M{
				{"whiteUserId": userObjID, "winner": "black"},
				{"blackUserId": userObjID, "winner": "white"},
			},
		})
	case "draws":
		conditions = append(conditions, bson.M{
			"$or": []bson.M{
				{"winner": ""},
				{"winner": nil},
				{"winner": bson.M{"$exists": false}},
			},
		})
	}

	filter := bson.M{"$and": conditions}

	// Get total count
	total, err := h.db.MatchHistory().CountDocuments(ctx, filter)
	if err != nil {
		http.Error(w, "Failed to count games", http.StatusInternalServerError)
		return
	}

	// Get paginated results
	skip := int64((page - 1) * limit)
	opts := options.Find().
		SetSort(bson.M{"completedAt": -1}).
		SetSkip(skip).
		SetLimit(int64(limit))

	cursor, err := h.db.MatchHistory().Find(ctx, filter, opts)
	if err != nil {
		http.Error(w, "Failed to fetch game history", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var games []models.MatchHistory
	if err := cursor.All(ctx, &games); err != nil {
		http.Error(w, "Failed to decode game history", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"games": games,
		"total": total,
		"page":  page,
		"limit": limit,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// LookupUserByDisplayName returns the user ID, display name, and Elo for a given display name.
func (h *GameHandler) LookupUserByDisplayName(w http.ResponseWriter, r *http.Request) {
	displayName := r.URL.Query().Get("displayName")
	if displayName == "" {
		respondWithError(w, http.StatusBadRequest, "displayName query parameter is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var user models.User
	err := h.db.Users().FindOne(ctx, bson.M{"displayName": displayName}).Decode(&user)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"userId":      user.ID.Hex(),
		"displayName": user.DisplayName,
		"eloRating":   user.EloRating,
	})
}

func parseIntParam(s string) (int, error) {
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}
