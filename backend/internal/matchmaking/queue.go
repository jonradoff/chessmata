package matchmaking

import (
	"context"
	"log"
	"math"
	"sync"
	"time"

	"chess-game/internal/db"
	"chess-game/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	initialEloRange      = 50
	eloRangeIncrement    = 25
	maxEloRange          = 200
	timeIncrementSeconds = 10
	processingInterval   = 2 * time.Second
)

type Queue struct {
	db     *db.MongoDB
	mu     sync.RWMutex
	ticker *time.Ticker
	stopCh chan bool
}

func NewQueue(database *db.MongoDB) *Queue {
	return &Queue{
		db:     database,
		stopCh: make(chan bool),
	}
}

// Start begins the background matching loop
func (q *Queue) Start() {
	q.ticker = time.NewTicker(processingInterval)
	go q.processLoop()
	log.Println("Matchmaking queue started")
}

// Stop halts the background matching loop
func (q *Queue) Stop() {
	if q.ticker != nil {
		q.ticker.Stop()
	}
	close(q.stopCh)
	log.Println("Matchmaking queue stopped")
}

// AddToQueue adds a player to the matchmaking queue
func (q *Queue) AddToQueue(ctx context.Context, entry *models.MatchmakingQueue) error {
	entry.ID = primitive.NewObjectID()
	entry.JoinedAt = time.Now()
	entry.ExpiresAt = time.Now().Add(models.DefaultQueueTimeout)
	entry.Status = models.QueueStatusWaiting

	_, err := q.db.MatchmakingQueue().InsertOne(ctx, entry)
	return err
}

// RemoveFromQueue removes a player from the queue
func (q *Queue) RemoveFromQueue(ctx context.Context, connectionID string) error {
	_, err := q.db.MatchmakingQueue().DeleteOne(ctx, bson.M{
		"connectionId": connectionID,
		"status":       models.QueueStatusWaiting,
	})
	return err
}

// GetQueueStatus returns the current queue status for a player
func (q *Queue) GetQueueStatus(ctx context.Context, connectionID string) (*models.MatchmakingQueue, error) {
	var entry models.MatchmakingQueue
	err := q.db.MatchmakingQueue().FindOne(ctx, bson.M{
		"connectionId": connectionID,
	}).Decode(&entry)

	if err != nil {
		return nil, err
	}

	return &entry, nil
}

// processLoop runs continuously to match players
func (q *Queue) processLoop() {
	for {
		select {
		case <-q.ticker.C:
			q.processMatches()
		case <-q.stopCh:
			return
		}
	}
}

// processMatches finds and creates matches for waiting players
func (q *Queue) processMatches() {
	ctx := context.Background()

	// Find all waiting players
	cursor, err := q.db.MatchmakingQueue().Find(ctx, bson.M{
		"status": models.QueueStatusWaiting,
	}, options.Find().SetSort(bson.M{"joinedAt": 1}))

	if err != nil {
		log.Printf("Error finding waiting players: %v", err)
		return
	}
	defer cursor.Close(ctx)

	var waitingPlayers []models.MatchmakingQueue
	if err := cursor.All(ctx, &waitingPlayers); err != nil {
		log.Printf("Error decoding waiting players: %v", err)
		return
	}

	if len(waitingPlayers) < 2 {
		return // Need at least 2 players
	}

	// Try to match players
	matched := make(map[primitive.ObjectID]bool)

	for i := 0; i < len(waitingPlayers); i++ {
		if matched[waitingPlayers[i].ID] {
			continue
		}

		player1 := waitingPlayers[i]

		// Find a match for this player
		for j := i + 1; j < len(waitingPlayers); j++ {
			if matched[waitingPlayers[j].ID] {
				continue
			}

			player2 := waitingPlayers[j]

			if q.canMatch(&player1, &player2) {
				// Create match
				if err := q.createMatch(ctx, &player1, &player2); err != nil {
					log.Printf("Error creating match: %v", err)
				} else {
					matched[player1.ID] = true
					matched[player2.ID] = true
					log.Printf("Matched players: %s and %s", player1.DisplayName, player2.DisplayName)
				}
				break
			}
		}
	}

	// Clean up expired entries
	q.cleanupExpired(ctx)
}

// canMatch determines if two players can be matched
func (q *Queue) canMatch(p1, p2 *models.MatchmakingQueue) bool {
	// Check opponent type preferences
	if !q.checkOpponentType(p1, p2) {
		return false
	}

	// Check ranked match type
	if p1.IsRanked != p2.IsRanked {
		return false
	}

	// If unranked, match immediately
	if !p1.IsRanked {
		return true
	}

	// For ranked games, check Elo compatibility
	return q.checkEloCompatibility(p1, p2)
}

// checkOpponentType checks if opponent type preferences are compatible
func (q *Queue) checkOpponentType(p1, p2 *models.MatchmakingQueue) bool {
	p1IsAI := p1.AgentName != ""
	p2IsAI := p2.AgentName != ""

	// Check p1's preference
	switch p1.OpponentType {
	case models.OpponentTypeHuman:
		if p2IsAI {
			return false
		}
	case models.OpponentTypeAI:
		if !p2IsAI {
			return false
		}
	case models.OpponentTypeEither:
		// No restriction
	}

	// Check p2's preference
	switch p2.OpponentType {
	case models.OpponentTypeHuman:
		if p1IsAI {
			return false
		}
	case models.OpponentTypeAI:
		if !p1IsAI {
			return false
		}
	case models.OpponentTypeEither:
		// No restriction
	}

	return true
}

// checkEloCompatibility checks if Elo ratings are compatible based on wait time
func (q *Queue) checkEloCompatibility(p1, p2 *models.MatchmakingQueue) bool {
	// Calculate wait time for each player
	p1WaitSeconds := int(time.Since(p1.JoinedAt).Seconds())
	p2WaitSeconds := int(time.Since(p2.JoinedAt).Seconds())

	// Calculate expanding Elo range for each player
	p1Range := q.calculateEloRange(p1WaitSeconds)
	p2Range := q.calculateEloRange(p2WaitSeconds)

	// Check if players' Elo ratings are within each other's acceptable range
	eloDiff := int(math.Abs(float64(p1.CurrentElo - p2.CurrentElo)))

	return eloDiff <= p1Range || eloDiff <= p2Range
}

// calculateEloRange calculates the acceptable Elo range based on wait time
func (q *Queue) calculateEloRange(waitSeconds int) int {
	increments := waitSeconds / timeIncrementSeconds
	eloRange := initialEloRange + (increments * eloRangeIncrement)

	if eloRange > maxEloRange {
		eloRange = maxEloRange
	}

	return eloRange
}

// createMatch creates a game for matched players
func (q *Queue) createMatch(ctx context.Context, p1, p2 *models.MatchmakingQueue) error {
	// Assign colors (randomly or based on preferences)
	whitePlayer, blackPlayer := q.assignColors(p1, p2)

	// Create game
	now := time.Now()
	game := models.Game{
		ID:          primitive.NewObjectID(),
		SessionID:   primitive.NewObjectID().Hex(),
		Players: []models.Player{
			{
				ID:          whitePlayer.ConnectionID,
				UserID:      whitePlayer.UserID,
				DisplayName: whitePlayer.DisplayName,
				AgentName:   whitePlayer.AgentName,
				Color:       models.White,
				EloRating:   whitePlayer.CurrentElo,
				JoinedAt:    now,
			},
			{
				ID:          blackPlayer.ConnectionID,
				UserID:      blackPlayer.UserID,
				DisplayName: blackPlayer.DisplayName,
				AgentName:   blackPlayer.AgentName,
				Color:       models.Black,
				EloRating:   blackPlayer.CurrentElo,
				JoinedAt:    now,
			},
		},
		Status:      models.GameStatusActive,
		CurrentTurn: models.White,
		BoardState:  models.InitialBoardFEN,
		IsRanked:    p1.IsRanked,
		GameType:    models.GameTypeMatchmaking,
		StartedAt:   &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	_, err := q.db.Games().InsertOne(ctx, game)
	if err != nil {
		return err
	}

	// Update queue entries to matched status
	_, err = q.db.MatchmakingQueue().UpdateMany(ctx, bson.M{
		"_id": bson.M{"$in": []primitive.ObjectID{p1.ID, p2.ID}},
	}, bson.M{
		"$set": bson.M{
			"status": models.QueueStatusMatched,
		},
	})

	if err != nil {
		return err
	}

	// TODO: Send WebSocket notifications to both players with game session ID
	// This will be handled by the WebSocket handler integration

	return nil
}

// assignColors assigns white and black colors to matched players
func (q *Queue) assignColors(p1, p2 *models.MatchmakingQueue) (*models.MatchmakingQueue, *models.MatchmakingQueue) {
	// If both have no preference, assign randomly
	if p1.PreferredColor == nil && p2.PreferredColor == nil {
		if time.Now().UnixNano()%2 == 0 {
			return p1, p2 // p1=white, p2=black
		}
		return p2, p1 // p2=white, p1=black
	}

	// If p1 prefers white
	if p1.PreferredColor != nil && *p1.PreferredColor == models.White {
		return p1, p2
	}

	// If p1 prefers black
	if p1.PreferredColor != nil && *p1.PreferredColor == models.Black {
		return p2, p1
	}

	// If p2 prefers white
	if p2.PreferredColor != nil && *p2.PreferredColor == models.White {
		return p2, p1
	}

	// If p2 prefers black
	if p2.PreferredColor != nil && *p2.PreferredColor == models.Black {
		return p1, p2
	}

	// Default: p1=white, p2=black
	return p1, p2
}

// cleanupExpired removes expired queue entries
func (q *Queue) cleanupExpired(ctx context.Context) {
	_, err := q.db.MatchmakingQueue().UpdateMany(ctx, bson.M{
		"expiresAt": bson.M{"$lt": time.Now()},
		"status":    models.QueueStatusWaiting,
	}, bson.M{
		"$set": bson.M{"status": models.QueueStatusExpired},
	})

	if err != nil {
		log.Printf("Error cleaning up expired queue entries: %v", err)
	}
}
