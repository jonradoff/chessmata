package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
	"unicode"

	"chess-game/internal/auth"
	"chess-game/internal/db"
	"chess-game/internal/game"
	"chess-game/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// activeGame tracks a running game's cancel func and turn notification channel.
type activeGame struct {
	cancel context.CancelFunc
	turnCh chan struct{}
}

// BuiltinAgent manages the chessmata-2ply AI that plays games via the move API.
type BuiltinAgent struct {
	db          *db.MongoDB
	jwtService  *auth.JWTService
	userID      primitive.ObjectID
	agentName   string
	serverAddr  string // e.g. "http://localhost:8080"
	accessToken string // JWT for authenticating HTTP requests
	mu          sync.Mutex
	activeGames map[string]*activeGame // sessionID -> game info
	stopCh      chan struct{}          // signals periodic check to stop
}

const maxConsecutiveErrors = 5 // agent exits game loop after this many consecutive getGame failures

// NewBuiltinAgent creates a new builtin agent instance.
func NewBuiltinAgent(database *db.MongoDB, jwtService *auth.JWTService, userID primitive.ObjectID, agentName string, serverAddr string) *BuiltinAgent {
	// Generate a JWT so the agent's HTTP requests pass player authorization
	token, err := jwtService.GenerateAccessToken(userID.Hex(), "system@chessmata.com", "Metavert")
	if err != nil {
		log.Printf("Warning: Agent failed to generate access token: %v", err)
	}

	return &BuiltinAgent{
		db:          database,
		jwtService:  jwtService,
		userID:      userID,
		agentName:   agentName,
		serverAddr:  serverAddr,
		accessToken: token,
		activeGames: make(map[string]*activeGame),
		stopCh:      make(chan struct{}),
	}
}

// ResumeActiveGames finds any active games the agent is involved in and resumes playing them.
// Called on server startup to recover from restarts.
func (a *BuiltinAgent) ResumeActiveGames() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Find active games where a player has this agent's name and user ID
	cursor, err := a.db.Games().Find(ctx, bson.M{
		"status": models.GameStatusActive,
		"players": bson.M{
			"$elemMatch": bson.M{
				"agentName": a.agentName,
				"userId":    a.userID,
			},
		},
	})
	if err != nil {
		log.Printf("Agent ResumeActiveGames: error querying games: %v", err)
		return
	}
	defer cursor.Close(ctx)

	var games []models.Game
	if err := cursor.All(ctx, &games); err != nil {
		log.Printf("Agent ResumeActiveGames: error decoding games: %v", err)
		return
	}

	for _, g := range games {
		for _, p := range g.Players {
			if p.AgentName == a.agentName && p.UserID != nil && *p.UserID == a.userID {
				log.Printf("Agent resuming active game %s as %s", g.SessionID, p.Color)
				a.StartGame(g.SessionID, p.Color)
				break
			}
		}
	}

	if len(games) == 0 {
		log.Printf("Agent ResumeActiveGames: no active games to resume")
	}
}

// StartGame begins the agent playing in a game. Called when matchmaking assigns
// the builtin agent to a game.
func (a *BuiltinAgent) StartGame(sessionID string, color models.PlayerColor) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Don't start twice
	if _, exists := a.activeGames[sessionID]; exists {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	turnCh := make(chan struct{}, 1)
	a.activeGames[sessionID] = &activeGame{cancel: cancel, turnCh: turnCh}

	log.Printf("Builtin agent %s starting game %s as %s", a.agentName, sessionID, color)
	go a.playGame(ctx, sessionID, color, turnCh)
}

// StopGame stops the agent from playing a specific game.
func (a *BuiltinAgent) StopGame(sessionID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if ag, exists := a.activeGames[sessionID]; exists {
		ag.cancel()
		delete(a.activeGames, sessionID)
	}
}

// NotifyTurn wakes the agent's game loop when it's the agent's turn.
func (a *BuiltinAgent) NotifyTurn(sessionID string) {
	a.mu.Lock()
	ag, exists := a.activeGames[sessionID]
	a.mu.Unlock()
	if exists {
		select {
		case ag.turnCh <- struct{}{}:
		default:
			// Already has a pending notification
		}
	}
}

// playGame is the main game loop for the agent.
func (a *BuiltinAgent) playGame(ctx context.Context, sessionID string, color models.PlayerColor, turnCh <-chan struct{}) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("AGENT PANIC in game %s: %v", sessionID, r)
		}
		a.mu.Lock()
		delete(a.activeGames, sessionID)
		a.mu.Unlock()
		log.Printf("Builtin agent %s finished game %s", a.agentName, sessionID)
	}()

	// Small initial delay to let the game fully persist to DB
	select {
	case <-ctx.Done():
		return
	case <-time.After(2 * time.Second):
	}

	// Find the agent's player ID in the game
	playerID := a.getPlayerID(ctx, sessionID)
	if playerID == "" {
		log.Printf("Agent could not find its player ID in game %s", sessionID)
		return
	}
	log.Printf("Agent %s found playerID=%s in game %s, color=%s", a.agentName, playerID, sessionID, color)

	consecutiveErrors := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Fetch game state
		gameState, err := a.getGame(ctx, sessionID)
		if err != nil {
			consecutiveErrors++
			log.Printf("Agent error fetching game %s (attempt %d/%d): %v",
				sessionID, consecutiveErrors, maxConsecutiveErrors, err)
			if consecutiveErrors >= maxConsecutiveErrors {
				log.Printf("Agent giving up on game %s after %d consecutive errors", sessionID, consecutiveErrors)
				return
			}
			// Exponential backoff: 1s, 2s, 4s, 8s, 16s
			backoff := time.Duration(1<<uint(consecutiveErrors-1)) * time.Second
			if backoff > 16*time.Second {
				backoff = 16 * time.Second
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
				continue
			}
		}
		consecutiveErrors = 0 // reset on success

		// Check if game is over
		if gameState.Status == models.GameStatusComplete {
			log.Printf("Agent: game %s is complete, exiting", sessionID)
			return
		}

		// Check for and auto-reject any pending draw offers directed at us
		if gameState.DrawOffers != nil && gameState.DrawOffers.PendingFrom != "" && gameState.DrawOffers.PendingFrom != string(color) {
			log.Printf("Agent: declining draw offer in game %s (offered by %s)", sessionID, gameState.DrawOffers.PendingFrom)
			if err := a.respondToDraw(sessionID, playerID, false); err != nil {
				log.Printf("Agent: error declining draw in game %s: %v", sessionID, err)
			}
			// Small delay after declining before continuing
			select {
			case <-ctx.Done():
				return
			case <-time.After(200 * time.Millisecond):
				continue
			}
		}

		// Check if it's our turn
		if gameState.CurrentTurn != color {
			// Wait for turn notification or timeout fallback (safety net)
			select {
			case <-ctx.Done():
				return
			case <-turnCh:
				continue
			case <-time.After(5 * time.Second):
				continue // Safety fallback in case notification is missed
			}
		}

		// It's our turn — compute best move
		log.Printf("Agent: computing move for game %s (FEN: %s)", sessionID, gameState.BoardState)
		board, err := game.ParseFEN(gameState.BoardState)
		if err != nil {
			log.Printf("Agent error parsing FEN: %v", err)
			return
		}

		bestMove := BestMove(board)
		if bestMove == nil {
			log.Printf("Agent found no legal moves in game %s", sessionID)
			return
		}
		log.Printf("Agent: best move %s->%s in game %s", bestMove.From.String(), bestMove.To.String(), sessionID)

		// Small delay to feel more natural (500ms-1500ms)
		thinkTime := 500 + time.Duration(len(gameState.BoardState)%1000)*time.Millisecond
		if thinkTime > 1500*time.Millisecond {
			thinkTime = 1500 * time.Millisecond
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(thinkTime):
		}

		// Submit the move
		promotion := ""
		if bestMove.Promotion != 0 {
			promotion = string(unicode.ToLower(bestMove.Promotion))
		}

		log.Printf("Agent: submitting move %s->%s (promotion=%q) to %s/api/games/%s/move",
			bestMove.From.String(), bestMove.To.String(), promotion, a.serverAddr, sessionID)
		err = a.makeMove(sessionID, playerID, bestMove.From.String(), bestMove.To.String(), promotion)
		if err != nil {
			log.Printf("Agent error making move in game %s: %v", sessionID, err)
			// Retry after a delay
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
				continue
			}
		} else {
			log.Printf("Agent: move submitted successfully in game %s", sessionID)
		}
	}
}

// getPlayerID finds the agent's player ID in the game.
func (a *BuiltinAgent) getPlayerID(ctx context.Context, sessionID string) string {
	var g models.Game
	err := a.db.Games().FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&g)
	if err != nil {
		log.Printf("Agent getPlayerID: error finding game %s: %v", sessionID, err)
		return ""
	}

	log.Printf("Agent getPlayerID: game %s has %d players, looking for agentName=%s userID=%s",
		sessionID, len(g.Players), a.agentName, a.userID.Hex())
	for _, p := range g.Players {
		userIDStr := "<nil>"
		if p.UserID != nil {
			userIDStr = p.UserID.Hex()
		}
		log.Printf("Agent getPlayerID: player id=%s, name=%s, agent=%q, userID=%s, color=%s",
			p.ID, p.DisplayName, p.AgentName, userIDStr, p.Color)
		if p.AgentName == a.agentName && p.UserID != nil && *p.UserID == a.userID {
			return p.ID
		}
	}
	return ""
}

// getGame fetches the current game state from the database.
func (a *BuiltinAgent) getGame(ctx context.Context, sessionID string) (*models.Game, error) {
	var g models.Game
	err := a.db.Games().FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&g)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// respondToDraw accepts or declines a draw offer via the game API.
func (a *BuiltinAgent) respondToDraw(sessionID, playerID string, accept bool) error {
	payload := map[string]interface{}{
		"playerId": playerID,
		"accept":   accept,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal draw response: %w", err)
	}
	url := fmt.Sprintf("%s/api/games/%s/respond-draw", a.serverAddr, sessionID)

	resp, err := a.authedPost(url, body)
	if err != nil {
		return fmt.Errorf("HTTP error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("respond-draw rejected (status %d): %v", resp.StatusCode, errResp)
	}

	return nil
}

// StartPeriodicCheck launches a background goroutine that every `interval`
// scans for active games where it is the agent's turn but no game loop is
// running, and restarts them. This catches any agent goroutines that silently
// died due to transient errors or panics.
func (a *BuiltinAgent) StartPeriodicCheck(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-a.stopCh:
				return
			case <-ticker.C:
				a.checkForStuckGames()
			}
		}
	}()
	log.Printf("Agent periodic stuck-game check started (interval: %v)", interval)
}

// Stop shuts down the periodic check goroutine and cancels all active game loops.
func (a *BuiltinAgent) Stop() {
	close(a.stopCh)

	a.mu.Lock()
	defer a.mu.Unlock()
	for sessionID, ag := range a.activeGames {
		ag.cancel()
		delete(a.activeGames, sessionID)
	}
	log.Println("Builtin agent stopped")
}

// checkForStuckGames queries the DB for active games where the agent should
// be playing and it's the agent's turn, then ensures a game loop is running.
func (a *BuiltinAgent) checkForStuckGames() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cursor, err := a.db.Games().Find(ctx, bson.M{
		"status": models.GameStatusActive,
		"players": bson.M{
			"$elemMatch": bson.M{
				"agentName": a.agentName,
				"userId":    a.userID,
			},
		},
	})
	if err != nil {
		log.Printf("Agent periodic check: error querying games: %v", err)
		return
	}
	defer cursor.Close(ctx)

	var games []models.Game
	if err := cursor.All(ctx, &games); err != nil {
		log.Printf("Agent periodic check: error decoding games: %v", err)
		return
	}

	resumed := 0
	for _, g := range games {
		// Find the agent's color
		for _, p := range g.Players {
			if p.AgentName == a.agentName && p.UserID != nil && *p.UserID == a.userID {
				// Check if game loop is already running
				a.mu.Lock()
				_, running := a.activeGames[g.SessionID]
				a.mu.Unlock()

				if !running {
					log.Printf("Agent periodic check: restarting stalled game %s (agent color: %s, turn: %s)",
						g.SessionID, p.Color, g.CurrentTurn)
					a.StartGame(g.SessionID, p.Color)
					resumed++
				}
				break
			}
		}
	}

	if resumed > 0 {
		log.Printf("Agent periodic check: resumed %d stalled game(s)", resumed)
	}
}

// authedPost sends a POST request with the agent's JWT Authorization header.
func (a *BuiltinAgent) authedPost(url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if a.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.accessToken)
	}
	return http.DefaultClient.Do(req)
}

// makeMove submits a move via the game API.
func (a *BuiltinAgent) makeMove(sessionID, playerID, from, to, promotion string) error {
	payload := map[string]string{
		"playerId": playerID,
		"from":     from,
		"to":       to,
	}
	if promotion != "" {
		payload["promotion"] = promotion
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal move payload: %w", err)
	}
	url := fmt.Sprintf("%s/api/games/%s/move", a.serverAddr, sessionID)

	resp, err := a.authedPost(url, body)
	if err != nil {
		return fmt.Errorf("HTTP error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("move rejected (status %d): %v", resp.StatusCode, errResp)
	}

	return nil
}
