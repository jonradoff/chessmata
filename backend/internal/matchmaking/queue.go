package matchmaking

import (
	"context"
	cryptorand "crypto/rand"
	"log"
	"math"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"chess-game/internal/db"
	"chess-game/internal/game"
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

	// Agent timing delays
	agentOnlyDelay   = 10 * time.Second // agent-only: wait 10s before any agent can match
	mixedAgentDelay  = 10 * time.Second // mixed: wait 10s before 3rd-party agents
	mixedBuiltinDelay = 20 * time.Second // mixed: wait 20s before builtin agent
)

// MatchNotifier is called when two players are matched.
// connectionId identifies the matchmaking client; sessionId is the new game.
type MatchNotifier func(connectionId string, sessionId string)

// BuiltinAgentProvider is called to inject the builtin agent into a game.
// It receives the game session ID and the agent's assigned color.
type BuiltinAgentProvider func(sessionID string, color models.PlayerColor)

// LobbyChangeNotifier is called whenever the lobby state changes (join, leave, match, expire).
type LobbyChangeNotifier func()

type Queue struct {
	db                   *db.MongoDB
	mu                   sync.RWMutex
	ticker               *time.Ticker
	stopCh               chan bool
	matchNotifier        MatchNotifier
	lobbyChangeNotifier  LobbyChangeNotifier
	builtinAgentProvider BuiltinAgentProvider
	builtinAgentUserID   *primitive.ObjectID
	builtinAgentName     string
	builtinAgentElo      int
}

func NewQueue(database *db.MongoDB) *Queue {
	return &Queue{
		db:     database,
		stopCh: make(chan bool),
	}
}

// SetMatchNotifier registers a callback invoked when a match is created.
func (q *Queue) SetMatchNotifier(fn MatchNotifier) {
	q.matchNotifier = fn
}

// SetLobbyChangeNotifier registers a callback invoked when the lobby state changes.
func (q *Queue) SetLobbyChangeNotifier(fn LobbyChangeNotifier) {
	q.lobbyChangeNotifier = fn
}

// SetBuiltinAgent registers the builtin AI agent for fallback matching.
func (q *Queue) SetBuiltinAgent(userID primitive.ObjectID, agentName string, elo int, provider BuiltinAgentProvider) {
	q.builtinAgentUserID = &userID
	q.builtinAgentName = agentName
	q.builtinAgentElo = elo
	q.builtinAgentProvider = provider
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
	if err == nil && q.lobbyChangeNotifier != nil {
		go q.lobbyChangeNotifier()
	}
	return err
}

// RemoveFromQueue removes a player from the queue
func (q *Queue) RemoveFromQueue(ctx context.Context, connectionID string) error {
	result, err := q.db.MatchmakingQueue().DeleteOne(ctx, bson.M{
		"connectionId": connectionID,
		"status":       models.QueueStatusWaiting,
	})
	if err == nil && result.DeletedCount > 0 && q.lobbyChangeNotifier != nil {
		go q.lobbyChangeNotifier()
	}
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

// GetQueuePosition returns the 1-based position of a player in the waiting queue
func (q *Queue) GetQueuePosition(ctx context.Context, entry *models.MatchmakingQueue) int {
	if entry.Status != models.QueueStatusWaiting {
		return 0
	}

	// Count how many waiting entries joined before this one
	count, err := q.db.MatchmakingQueue().CountDocuments(ctx, bson.M{
		"status":   models.QueueStatusWaiting,
		"joinedAt": bson.M{"$lte": entry.JoinedAt},
	})
	if err != nil {
		return 1 // fallback
	}

	if count < 1 {
		return 1
	}
	return int(count)
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

// processMatches finds and creates matches for waiting players.
// Uses Elo-sorted bounded search for O(n) matching and a distributed lock
// to prevent concurrent processing across multiple instances.
func (q *Queue) processMatches() {
	ctx := context.Background()

	// Acquire distributed lock to prevent concurrent processing
	lockID := "matchmaking_process"
	now := time.Now()
	lockExpiry := now.Add(5 * time.Second)

	_, err := q.db.CleanupLocks().UpdateOne(ctx,
		bson.M{
			"_id": lockID,
			"$or": []bson.M{
				{"expiresAt": bson.M{"$lte": now}},
			},
		},
		bson.M{
			"$set": bson.M{
				"expiresAt": lockExpiry,
				"lockedAt":  now,
			},
		},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		// Another instance holds the lock, skip this cycle
		return
	}
	defer q.db.CleanupLocks().DeleteOne(ctx, bson.M{"_id": lockID})

	// Find all waiting players, sorted by Elo for bounded search
	cursor, err := q.db.MatchmakingQueue().Find(ctx, bson.M{
		"status": models.QueueStatusWaiting,
	}, options.Find().SetSort(bson.M{"currentElo": 1}))

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

	if len(waitingPlayers) < 1 {
		return
	}

	// Match players using bounded bidirectional search on Elo-sorted list.
	// For each player, search forward/backward and stop when Elo gap > maxEloRange.
	matched := make(map[primitive.ObjectID]bool)

	for i := 0; i < len(waitingPlayers); i++ {
		if matched[waitingPlayers[i].ID] {
			continue
		}

		player := waitingPlayers[i]
		var bestMatch *models.MatchmakingQueue
		bestEloDiff := math.MaxInt32

		// Search forward (higher Elo)
		for j := i + 1; j < len(waitingPlayers); j++ {
			if matched[waitingPlayers[j].ID] {
				continue
			}
			candidate := waitingPlayers[j]
			eloDiff := candidate.CurrentElo - player.CurrentElo
			if eloDiff > maxEloRange {
				break // All further candidates have even higher Elo
			}
			if !q.canMatch(&player, &candidate) || !q.isAgentMatchAllowed(&player, &candidate) {
				continue
			}
			if eloDiff < bestEloDiff {
				bestEloDiff = eloDiff
				c := candidate
				bestMatch = &c
			}
		}

		// Search backward (lower Elo)
		for j := i - 1; j >= 0; j-- {
			if matched[waitingPlayers[j].ID] {
				continue
			}
			candidate := waitingPlayers[j]
			eloDiff := player.CurrentElo - candidate.CurrentElo
			if eloDiff > maxEloRange {
				break // All further candidates have even lower Elo
			}
			if !q.canMatch(&player, &candidate) || !q.isAgentMatchAllowed(&player, &candidate) {
				continue
			}
			if eloDiff < bestEloDiff {
				bestEloDiff = eloDiff
				c := candidate
				bestMatch = &c
			}
		}

		if bestMatch != nil {
			if err := q.createMatch(ctx, &player, bestMatch); err != nil {
				log.Printf("Error creating match: %v", err)
			} else {
				matched[player.ID] = true
				matched[bestMatch.ID] = true
				log.Printf("Matched players: %s (Elo %d) and %s (Elo %d), diff=%d",
					player.DisplayName, player.CurrentElo,
					bestMatch.DisplayName, bestMatch.CurrentElo, bestEloDiff)
			}
		}
	}

	// After matching, check if any unmatched human players need the builtin agent
	if q.builtinAgentProvider != nil && q.builtinAgentUserID != nil {
		for i := range waitingPlayers {
			if matched[waitingPlayers[i].ID] {
				continue
			}
			p := waitingPlayers[i]
			// Skip if player is an agent themselves
			if p.AgentName != "" {
				continue
			}
			// Skip if player wants humans only
			if p.OpponentType == models.OpponentTypeHuman {
				continue
			}

			waitDuration := time.Since(p.JoinedAt)
			needsBuiltin := false

			if p.OpponentType == models.OpponentTypeAI && waitDuration >= agentOnlyDelay {
				needsBuiltin = true
			} else if p.OpponentType == models.OpponentTypeEither && waitDuration >= mixedBuiltinDelay {
				needsBuiltin = true
			}

			if needsBuiltin {
				q.injectBuiltinAgent(ctx, &p)
				matched[p.ID] = true
			}
		}
	}

	// Clean up expired entries
	q.cleanupExpired(ctx)
}

// isAgentMatchAllowed checks timing restrictions for agent-vs-human matches.
// Returns true if the match is allowed right now.
func (q *Queue) isAgentMatchAllowed(human, agent *models.MatchmakingQueue) bool {
	// If neither is an agent, no restrictions
	humanIsAgent := human.AgentName != ""
	agentIsAgent := agent.AgentName != ""

	if !humanIsAgent && !agentIsAgent {
		return true // both human, no delay
	}

	// Identify which is the human and which is the agent
	h, a := human, agent
	if humanIsAgent && !agentIsAgent {
		h, a = agent, human
	} else if humanIsAgent && agentIsAgent {
		return true // agent vs agent, no delay
	}

	waitDuration := time.Since(h.JoinedAt)
	isBuiltin := a.IsBuiltinAgent

	switch h.OpponentType {
	case models.OpponentTypeAI:
		// Agent-only: wait 10s before any agent
		return waitDuration >= agentOnlyDelay
	case models.OpponentTypeEither:
		// Mixed: 10s for 3rd-party agents, 20s for builtin
		if isBuiltin {
			return waitDuration >= mixedBuiltinDelay
		}
		return waitDuration >= mixedAgentDelay
	case models.OpponentTypeHuman:
		// Should not reach here (checkOpponentType filters this)
		return false
	}

	return true
}

// injectBuiltinAgent creates a game with the builtin agent for a waiting player.
func (q *Queue) injectBuiltinAgent(ctx context.Context, human *models.MatchmakingQueue) {
	// Create a synthetic queue entry for the builtin agent
	agentEntry := &models.MatchmakingQueue{
		ID:             primitive.NewObjectID(),
		UserID:         q.builtinAgentUserID,
		ConnectionID:   "builtin-" + primitive.NewObjectID().Hex(),
		DisplayName:    q.builtinAgentName,
		AgentName:      q.builtinAgentName,
		IsBuiltinAgent: true,
		IsRanked:       human.IsRanked,
		CurrentElo:     q.builtinAgentElo,
		OpponentType:   models.OpponentTypeEither,
		TimeControls:   human.TimeControls, // accept whatever the human wants
		JoinedAt:       time.Now(),
		ExpiresAt:      time.Now().Add(models.DefaultQueueTimeout),
		Status:         models.QueueStatusWaiting,
	}

	if err := q.createMatch(ctx, human, agentEntry); err != nil {
		log.Printf("Error creating builtin agent match: %v", err)
		return
	}

	log.Printf("Injected builtin agent %s for player %s after %v wait",
		q.builtinAgentName, human.DisplayName, time.Since(human.JoinedAt))
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

	// Check time control compatibility
	if !q.checkTimeControlCompatibility(p1, p2) {
		return false
	}

	// Prevent agents with the same engine name from matching
	if p1.EngineName != "" && p2.EngineName != "" && p1.EngineName == p2.EngineName {
		return false
	}

	// If unranked, match immediately
	if !p1.IsRanked {
		return true
	}

	// For ranked games, check Elo compatibility
	return q.checkEloCompatibility(p1, p2)
}

// checkTimeControlCompatibility checks if players have at least one common time control preference
func (q *Queue) checkTimeControlCompatibility(p1, p2 *models.MatchmakingQueue) bool {
	// If either player has no time control preferences, assume they accept any
	if len(p1.TimeControls) == 0 || len(p2.TimeControls) == 0 {
		return true
	}

	// Check for overlap
	for _, tc1 := range p1.TimeControls {
		for _, tc2 := range p2.TimeControls {
			if tc1 == tc2 {
				return true
			}
		}
	}

	return false
}

// findOverlappingTimeControl finds a random time control that both players accept
func (q *Queue) findOverlappingTimeControl(p1, p2 *models.MatchmakingQueue) game.TimeControlMode {
	// If either player has no preferences, default to standard
	if len(p1.TimeControls) == 0 || len(p2.TimeControls) == 0 {
		// If one has preferences, use that
		if len(p1.TimeControls) > 0 {
			return p1.TimeControls[rand.Intn(len(p1.TimeControls))]
		}
		if len(p2.TimeControls) > 0 {
			return p2.TimeControls[rand.Intn(len(p2.TimeControls))]
		}
		return game.TimeStandard
	}

	// Find overlapping time controls
	var overlap []game.TimeControlMode
	for _, tc1 := range p1.TimeControls {
		for _, tc2 := range p2.TimeControls {
			if tc1 == tc2 {
				overlap = append(overlap, tc1)
			}
		}
	}

	if len(overlap) == 0 {
		return game.TimeStandard // Should not happen if checkTimeControlCompatibility passed
	}

	// Pick a random one from the overlap
	return overlap[rand.Intn(len(overlap))]
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

	// Determine time control for this match
	selectedTimeControl := q.findOverlappingTimeControl(p1, p2)
	timeControl := game.GetTimeControl(selectedTimeControl)

	// Create game
	now := time.Now()
	nowMs := now.UnixMilli()

	newGame := models.Game{
		ID:        primitive.NewObjectID(),
		SessionID: primitive.NewObjectID().Hex(),
		Players: []models.Player{
			{
				ID:             whitePlayer.ConnectionID,
				UserID:         whitePlayer.UserID,
				DisplayName:    whitePlayer.DisplayName,
				AgentName:      whitePlayer.AgentName,
				ClientSoftware: whitePlayer.ClientSoftware,
				Color:          models.White,
				EloRating:      whitePlayer.CurrentElo,
				JoinedAt:       now,
			},
			{
				ID:             blackPlayer.ConnectionID,
				UserID:         blackPlayer.UserID,
				DisplayName:    blackPlayer.DisplayName,
				AgentName:      blackPlayer.AgentName,
				ClientSoftware: blackPlayer.ClientSoftware,
				Color:          models.Black,
				EloRating:      blackPlayer.CurrentElo,
				JoinedAt:       now,
			},
		},
		Status:          models.GameStatusActive,
		CurrentTurn:     models.White,
		BoardState:      models.InitialBoardFEN,
		IsRanked:        p1.IsRanked,
		GameType:        models.GameTypeMatchmaking,
		StartedAt:       &now,
		CreatedAt:       now,
		UpdatedAt:       now,
		TimeControl:     &timeControl,
		DrawOffers:      &game.DrawOffers{},
		PositionHistory: []string{models.InitialBoardFEN},
	}

	// Initialize player times if this is a timed game
	if !timeControl.IsUnlimited() {
		newGame.PlayerTimes = &game.PlayerTimes{
			White: game.PlayerTime{RemainingMs: timeControl.BaseTimeMs, LastMoveAt: nowMs},
			Black: game.PlayerTime{RemainingMs: timeControl.BaseTimeMs, LastMoveAt: nowMs},
		}
	}

	_, err := q.db.Games().InsertOne(ctx, newGame)
	if err != nil {
		return err
	}

	// Update queue entries to matched status with the game session ID
	// (builtin agent entries aren't in the DB, so we only update real entries)
	entryIDs := []primitive.ObjectID{p1.ID}
	if !p2.IsBuiltinAgent {
		entryIDs = append(entryIDs, p2.ID)
	}
	if !p1.IsBuiltinAgent && len(entryIDs) == 1 {
		entryIDs = append(entryIDs, p2.ID)
	}

	_, err = q.db.MatchmakingQueue().UpdateMany(ctx, bson.M{
		"_id": bson.M{"$in": entryIDs},
	}, bson.M{
		"$set": bson.M{
			"status":           models.QueueStatusMatched,
			"matchedSessionId": newGame.SessionID,
		},
	})

	if err != nil {
		return err
	}

	// Notify lobby clients that the lobby changed (players matched)
	if q.lobbyChangeNotifier != nil {
		go q.lobbyChangeNotifier()
	}

	// Push match notification via WebSocket (instant).
	if q.matchNotifier != nil {
		if !p1.IsBuiltinAgent {
			q.matchNotifier(p1.ConnectionID, newGame.SessionID)
		}
		if !p2.IsBuiltinAgent {
			q.matchNotifier(p2.ConnectionID, newGame.SessionID)
		}
	}

	// If one of the players is the builtin agent, notify the agent provider
	if q.builtinAgentProvider != nil {
		if p1.IsBuiltinAgent {
			if whitePlayer.ConnectionID == p1.ConnectionID {
				q.builtinAgentProvider(newGame.SessionID, models.White)
			} else {
				q.builtinAgentProvider(newGame.SessionID, models.Black)
			}
		}
		if p2.IsBuiltinAgent {
			if whitePlayer.ConnectionID == p2.ConnectionID {
				q.builtinAgentProvider(newGame.SessionID, models.White)
			} else {
				q.builtinAgentProvider(newGame.SessionID, models.Black)
			}
		}
	}

	return nil
}

// assignColors assigns white and black colors to matched players
func (q *Queue) assignColors(p1, p2 *models.MatchmakingQueue) (*models.MatchmakingQueue, *models.MatchmakingQueue) {
	// If both have no preference, assign randomly using crypto/rand
	if p1.PreferredColor == nil && p2.PreferredColor == nil {
		n, err := cryptorand.Int(cryptorand.Reader, big.NewInt(2))
		if err == nil && n.Int64() == 0 {
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
	result, err := q.db.MatchmakingQueue().UpdateMany(ctx, bson.M{
		"expiresAt": bson.M{"$lt": time.Now()},
		"status":    models.QueueStatusWaiting,
	}, bson.M{
		"$set": bson.M{"status": models.QueueStatusExpired},
	})

	if err != nil {
		log.Printf("Error cleaning up expired queue entries: %v", err)
	} else if result.ModifiedCount > 0 && q.lobbyChangeNotifier != nil {
		go q.lobbyChangeNotifier()
	}
}
