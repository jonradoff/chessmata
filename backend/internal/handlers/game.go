package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"chess-game/internal/db"
	"chess-game/internal/game"
	"chess-game/internal/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type GameHandler struct {
	db *db.MongoDB
	ws *WebSocketHandler
}

func NewGameHandler(database *db.MongoDB, wsHandler *WebSocketHandler) *GameHandler {
	return &GameHandler{db: database, ws: wsHandler}
}

type CreateGameResponse struct {
	SessionID string `json:"sessionId"`
	PlayerID  string `json:"playerId"`
	ShareLink string `json:"shareLink"`
}

type JoinGameResponse struct {
	SessionID string              `json:"sessionId"`
	PlayerID  string              `json:"playerId"`
	Color     models.PlayerColor  `json:"color"`
	Game      *models.Game        `json:"game"`
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

	sessionID := generateID()
	playerID := generateID()

	game := &models.Game{
		SessionID:   sessionID,
		Players:     []models.Player{{ID: playerID, Color: models.White, JoinedAt: time.Now()}},
		Status:      models.GameStatusWaiting,
		CurrentTurn: models.White,
		BoardState:  models.InitialBoardFEN,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	_, err := h.db.Games().InsertOne(ctx, game)
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
			response := JoinGameResponse{
				SessionID: sessionID,
				PlayerID:  p.ID,
				Color:     p.Color,
				Game:      &existingGame,
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

	update := bson.M{
		"$push": bson.M{"players": newPlayer},
		"$set": bson.M{
			"status":    models.GameStatusActive,
			"updatedAt": time.Now(),
		},
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
		SessionID: sessionID,
		PlayerID:  playerID,
		Color:     models.Black,
		Game:      &existingGame,
	}

	// Broadcast player joined to existing players
	if h.ws != nil {
		h.ws.BroadcastPlayerJoined(sessionID, &existingGame)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(existingGame)
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

	// Verify player is in the game
	var player *models.Player
	for _, p := range existingGame.Players {
		if p.ID == req.PlayerID {
			player = &p
			break
		}
	}
	if player == nil {
		respondWithError(w, http.StatusBadRequest, "Player not in game")
		return
	}

	// Verify it's player's turn
	if player.Color != existingGame.CurrentTurn {
		respondWithError(w, http.StatusBadRequest, "Not your turn")
		return
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

	// Generate notation before making the move
	var promotionRune rune
	if len(req.Promotion) > 0 {
		promotionRune = rune(req.Promotion[0])
	}
	notation := board.GenerateNotation(fromPos, toPos, promotionRune)

	// Make the move
	piece := board.GetPiece(fromPos)
	isCapture := board.GetPiece(toPos) != 0 || toPos.String() == board.EnPassantSquare
	newBoard := board.MakeMove(fromPos, toPos, promotionRune)

	// Count moves
	moveCount, err := h.db.Moves().CountDocuments(ctx, bson.M{"sessionId": sessionID})
	if err != nil {
		moveCount = 0
	}

	// Create move record
	move := &models.Move{
		GameID:     existingGame.ID,
		SessionID:  sessionID,
		PlayerID:   req.PlayerID,
		MoveNumber: int(moveCount) + 1,
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
	updateFields := bson.M{
		"boardState":  newBoard.ToFEN(),
		"currentTurn": nextTurn,
		"status":      status,
		"updatedAt":   time.Now(),
	}

	// Check for game end conditions
	if newBoard.IsCheckmate() {
		status = models.GameStatusComplete
		updateFields["status"] = status
		updateFields["winner"] = existingGame.CurrentTurn // Current player (who just moved) wins
		updateFields["winReason"] = "checkmate"
	} else if newBoard.IsStalemate() {
		status = models.GameStatusComplete
		updateFields["status"] = status
		updateFields["winReason"] = "stalemate"
		// No winner in stalemate
	}

	update := bson.M{"$set": updateFields}

	_, err = h.db.Games().UpdateOne(ctx, bson.M{"sessionId": sessionID}, update)
	if err != nil {
		http.Error(w, "Failed to update game", http.StatusInternalServerError)
		return
	}

	// Fetch updated game for broadcast
	var updatedGame models.Game
	h.db.Games().FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&updatedGame)

	// Broadcast move to other player
	if h.ws != nil {
		h.ws.BroadcastMove(sessionID, &updatedGame, move, req.PlayerID)
	}

	response := MakeMoveResponse{
		Success:    true,
		Move:       move,
		BoardState: newBoard.ToFEN(),
		Check:      move.Check,
		Checkmate:  move.Checkmate,
		Stalemate:  newBoard.IsStalemate(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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

	var moves []models.Move
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

	// Verify player is in the game
	var resigningPlayer *models.Player
	var winnerColor models.PlayerColor
	for _, p := range existingGame.Players {
		if p.ID == req.PlayerID {
			resigningPlayer = &p
			// Winner is the opposite color
			if p.Color == models.White {
				winnerColor = models.Black
			} else {
				winnerColor = models.White
			}
			break
		}
	}
	if resigningPlayer == nil {
		response := ResignResponse{Success: false, Error: "Player not in game"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
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
	moveCount, _ := h.db.Moves().CountDocuments(ctx, bson.M{"sessionId": sessionID})
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
