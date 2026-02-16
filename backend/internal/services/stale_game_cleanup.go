package services

import (
	"context"
	"log"
	"os"
	"time"

	"chess-game/internal/db"
	"chess-game/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GameOverBroadcaster is implemented by WebSocketHandler to broadcast game over events.
type GameOverBroadcaster interface {
	BroadcastGameOver(sessionId string, game *models.Game, winner string, reason string)
}

// StaleGameCleanupService periodically finds and completes games where a player
// timed out but the game was never marked complete (e.g., server restart, missed timer).
type StaleGameCleanupService struct {
	db                    *db.MongoDB
	gameCompletionService *GameCompletionService
	broadcaster           GameOverBroadcaster
	stopCh                chan struct{}
	interval              time.Duration
	staleThreshold        time.Duration
}

// NewStaleGameCleanupService creates a new cleanup service.
func NewStaleGameCleanupService(
	database *db.MongoDB,
	completionService *GameCompletionService,
	broadcaster GameOverBroadcaster,
) *StaleGameCleanupService {
	return &StaleGameCleanupService{
		db:                    database,
		gameCompletionService: completionService,
		broadcaster:           broadcaster,
		stopCh:                make(chan struct{}),
		interval:              1 * time.Minute,
		staleThreshold:        30 * time.Second,
	}
}

// Start begins the periodic cleanup loop in a background goroutine.
func (s *StaleGameCleanupService) Start() {
	go s.runCleanupLoop()
	log.Println("Stale game cleanup service started (interval: 1m, threshold: 30s)")
}

// Stop signals the cleanup loop to exit.
func (s *StaleGameCleanupService) Stop() {
	close(s.stopCh)
	log.Println("Stale game cleanup service stopped")
}

func (s *StaleGameCleanupService) runCleanupLoop() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.runCleanupPass()
		}
	}
}

func (s *StaleGameCleanupService) runCleanupPass() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Try to acquire distributed lock
	if !s.tryAcquireLock(ctx) {
		return // Another server is handling cleanup
	}
	defer s.releaseLock(ctx)

	games, err := s.findStaleGames(ctx)
	if err != nil {
		log.Printf("Stale game cleanup: failed to query stale games: %v", err)
		return
	}

	if len(games) == 0 {
		return
	}

	log.Printf("Stale game cleanup: found %d stale game(s)", len(games))

	for i := range games {
		s.completeStaleGame(ctx, &games[i])
	}
}

func (s *StaleGameCleanupService) tryAcquireLock(ctx context.Context) bool {
	hostname, err := os.Hostname()
	if err != nil {
		log.Printf("Failed to get hostname: %v", err)
		hostname = "unknown"
	}

	now := time.Now()
	lockExpiry := now.Add(5 * time.Minute)

	filter := bson.M{
		"_id": "stale_game_cleanup",
		"$or": []bson.M{
			{"lockedUntil": bson.M{"$exists": false}},
			{"lockedUntil": bson.M{"$lt": now}},
		},
	}

	update := bson.M{
		"$set": bson.M{
			"lockedUntil": lockExpiry,
			"lockedBy":    hostname,
			"lockedAt":    now,
		},
	}

	opts := options.FindOneAndUpdate().SetUpsert(true)
	err = s.db.CleanupLocks().FindOneAndUpdate(ctx, filter, update, opts).Err()
	if err != nil {
		// If err is not nil, another server already holds the lock (duplicate key or no match)
		return false
	}

	return true
}

func (s *StaleGameCleanupService) releaseLock(ctx context.Context) {
	_, err := s.db.CleanupLocks().UpdateOne(ctx,
		bson.M{"_id": "stale_game_cleanup"},
		bson.M{"$set": bson.M{"lockedUntil": time.Now()}},
	)
	if err != nil {
		log.Printf("Stale game cleanup: failed to release lock: %v", err)
	}
}

// RunImmediateCleanup runs a one-shot cleanup pass with zero threshold, catching
// any games that timed out during server downtime. Call on startup BEFORE
// resuming agent game loops so the agent doesn't waste cycles on dead games.
func (s *StaleGameCleanupService) RunImmediateCleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	games, err := s.findStaleGamesWithThreshold(ctx, 0)
	if err != nil {
		log.Printf("Startup stale game cleanup: failed to query: %v", err)
		return
	}

	if len(games) == 0 {
		log.Println("Startup stale game cleanup: no timed-out games found")
		return
	}

	log.Printf("Startup stale game cleanup: found %d timed-out game(s)", len(games))
	for i := range games {
		s.completeStaleGame(ctx, &games[i])
	}
}

func (s *StaleGameCleanupService) findStaleGames(ctx context.Context) ([]models.Game, error) {
	return s.findStaleGamesWithThreshold(ctx, s.staleThreshold)
}

func (s *StaleGameCleanupService) findStaleGamesWithThreshold(ctx context.Context, threshold time.Duration) ([]models.Game, error) {
	// A game is stale if:
	//   currentTurn player's (lastMoveAt + remainingMs) < (now - threshold)
	// This means their clock ran out more than threshold ago.
	cutoff := time.Now().Add(-threshold).UnixMilli()

	filter := bson.M{
		"status":           string(models.GameStatusActive),
		"timeControl.mode": bson.M{"$ne": "unlimited"},
		"playerTimes":      bson.M{"$exists": true},
		"$or": []bson.M{
			{
				"currentTurn": "white",
				"$expr": bson.M{
					"$lt": []interface{}{
						bson.M{"$add": []interface{}{"$playerTimes.white.lastMoveAt", "$playerTimes.white.remainingMs"}},
						cutoff,
					},
				},
			},
			{
				"currentTurn": "black",
				"$expr": bson.M{
					"$lt": []interface{}{
						bson.M{"$add": []interface{}{"$playerTimes.black.lastMoveAt", "$playerTimes.black.remainingMs"}},
						cutoff,
					},
				},
			},
		},
	}

	cursor, err := s.db.Games().Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var games []models.Game
	if err := cursor.All(ctx, &games); err != nil {
		return nil, err
	}

	return games, nil
}

func (s *StaleGameCleanupService) completeStaleGame(ctx context.Context, game *models.Game) {
	// Determine winner: the player whose turn it is timed out, so the other player wins
	var winnerColor models.PlayerColor
	if game.CurrentTurn == models.White {
		winnerColor = models.Black
	} else {
		winnerColor = models.White
	}

	now := time.Now()

	// Atomic update with status:"active" filter to prevent double-processing
	result, err := s.db.Games().UpdateOne(ctx,
		bson.M{
			"sessionId": game.SessionID,
			"status":    string(models.GameStatusActive),
		},
		bson.M{
			"$set": bson.M{
				"status":      string(models.GameStatusComplete),
				"winner":      string(winnerColor),
				"winReason":   "timeout",
				"completedAt": now,
				"updatedAt":   now,
			},
		},
	)
	if err != nil {
		log.Printf("Stale game cleanup: failed to update game %s: %v", game.SessionID, err)
		return
	}

	if result.MatchedCount == 0 {
		// Another server already completed this game
		return
	}

	// Fetch the updated game for completion processing and broadcasting
	var updatedGame models.Game
	err = s.db.Games().FindOne(ctx, bson.M{"sessionId": game.SessionID}).Decode(&updatedGame)
	if err != nil {
		log.Printf("Stale game cleanup: failed to fetch updated game %s: %v", game.SessionID, err)
		return
	}

	// Process game completion (Elo changes, match history)
	if s.gameCompletionService != nil {
		s.gameCompletionService.ProcessGameCompletion(ctx, &updatedGame)
		// Re-fetch to get Elo changes applied to the game document
		s.db.Games().FindOne(ctx, bson.M{"sessionId": game.SessionID}).Decode(&updatedGame)
	}

	// Broadcast game over to any connected players
	if s.broadcaster != nil {
		s.broadcaster.BroadcastGameOver(game.SessionID, &updatedGame, string(winnerColor), "timeout")
	}

	log.Printf("Stale game cleanup: completed game %s (winner: %s, reason: timeout)", game.SessionID, winnerColor)
}
