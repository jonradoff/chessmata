package services

import (
	"context"
	"log"
	"time"

	"chess-game/internal/db"
	"chess-game/internal/elo"
	"chess-game/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GameCompletionResult holds the results of processing a completed game
type GameCompletionResult struct {
	WhiteEloChange int `json:"whiteEloChange"`
	BlackEloChange int `json:"blackEloChange"`
	WhiteNewElo    int `json:"whiteNewElo"`
	BlackNewElo    int `json:"blackNewElo"`
}

// GameCompletionService handles post-game processing like Elo updates
type GameCompletionService struct {
	db         *db.MongoDB
	calculator *elo.Calculator
}

// NewGameCompletionService creates a new game completion service
func NewGameCompletionService(database *db.MongoDB) *GameCompletionService {
	return &GameCompletionService{
		db:         database,
		calculator: elo.NewCalculator(),
	}
}

// ProcessGameCompletion handles all post-game tasks including Elo updates and match history
func (s *GameCompletionService) ProcessGameCompletion(ctx context.Context, game *models.Game) (*GameCompletionResult, error) {
	if game.Status != models.GameStatusComplete {
		log.Printf("Game %s is not complete (status: %s), skipping", game.SessionID, game.Status)
		return nil, nil
	}

	// Find white and black players
	var whitePlayer, blackPlayer *models.Player
	for i := range game.Players {
		if game.Players[i].Color == models.White {
			whitePlayer = &game.Players[i]
		} else if game.Players[i].Color == models.Black {
			blackPlayer = &game.Players[i]
		}
	}

	if whitePlayer == nil || blackPlayer == nil {
		log.Printf("Game %s doesn't have both players, skipping", game.SessionID)
		return nil, nil
	}

	// Calculate game duration
	var gameDuration int = 0
	if game.StartedAt != nil && game.CompletedAt != nil {
		gameDuration = int(game.CompletedAt.Sub(*game.StartedAt).Seconds())
	}

	// Count moves
	moveCount, _ := s.db.Moves().CountDocuments(ctx, bson.M{"sessionId": game.SessionID})

	var result *GameCompletionResult

	// Elo calculations only for ranked games
	whiteEloChange := 0
	blackEloChange := 0
	whiteNewElo := whitePlayer.EloRating
	blackNewElo := blackPlayer.EloRating

	if game.IsRanked {
		// Get game result for each player
		whiteResult, blackResult := elo.GetGameResultFromWinner(string(game.Winner))

		// Determine games played for K-factor, from user or agent rating
		whiteIsAgent := whitePlayer.AgentName != ""
		blackIsAgent := blackPlayer.AgentName != ""

		var whiteGamesPlayed, blackGamesPlayed int

		if whiteIsAgent && whitePlayer.UserID != nil {
			if ar := s.getAgentRating(ctx, *whitePlayer.UserID, whitePlayer.AgentName); ar != nil {
				whiteGamesPlayed = ar.RankedGamesPlayed
			}
		} else if whitePlayer.UserID != nil {
			if u := s.getUserByID(ctx, *whitePlayer.UserID); u != nil {
				whiteGamesPlayed = u.RankedGamesPlayed
			}
		}

		if blackIsAgent && blackPlayer.UserID != nil {
			if ar := s.getAgentRating(ctx, *blackPlayer.UserID, blackPlayer.AgentName); ar != nil {
				blackGamesPlayed = ar.RankedGamesPlayed
			}
		} else if blackPlayer.UserID != nil {
			if u := s.getUserByID(ctx, *blackPlayer.UserID); u != nil {
				blackGamesPlayed = u.RankedGamesPlayed
			}
		}

		// Calculate new ratings
		whiteNewElo = s.calculator.CalculateNewRating(
			whitePlayer.EloRating,
			blackPlayer.EloRating,
			whiteResult,
			whiteGamesPlayed,
		)
		blackNewElo = s.calculator.CalculateNewRating(
			blackPlayer.EloRating,
			whitePlayer.EloRating,
			blackResult,
			blackGamesPlayed,
		)

		whiteEloChange = whiteNewElo - whitePlayer.EloRating
		blackEloChange = blackNewElo - blackPlayer.EloRating

		log.Printf("Game %s Elo update: White %d -> %d (%+d), Black %d -> %d (%+d)",
			game.SessionID,
			whitePlayer.EloRating, whiteNewElo, whiteEloChange,
			blackPlayer.EloRating, blackNewElo, blackEloChange)

		// Update stats: agent Elo for agents, user Elo for humans
		if whiteIsAgent && whitePlayer.UserID != nil {
			s.updateAgentStats(ctx, *whitePlayer.UserID, whitePlayer.AgentName, whiteNewElo, whiteResult)
		} else if whitePlayer.UserID != nil {
			s.updateUserStats(ctx, *whitePlayer.UserID, whiteNewElo, whiteResult)
		}

		if blackIsAgent && blackPlayer.UserID != nil {
			s.updateAgentStats(ctx, *blackPlayer.UserID, blackPlayer.AgentName, blackNewElo, blackResult)
		} else if blackPlayer.UserID != nil {
			s.updateUserStats(ctx, *blackPlayer.UserID, blackNewElo, blackResult)
		}

		// Update game with Elo changes (for frontend to display)
		s.updateGameWithEloChanges(ctx, game.SessionID, whiteEloChange, blackEloChange, whiteNewElo, blackNewElo)

		result = &GameCompletionResult{
			WhiteEloChange: whiteEloChange,
			BlackEloChange: blackEloChange,
			WhiteNewElo:    whiteNewElo,
			BlackNewElo:    blackNewElo,
		}
	} else {
		// Increment totalGamesPlayed for unranked games
		if whitePlayer.UserID != nil {
			s.db.Users().UpdateOne(ctx, bson.M{"_id": *whitePlayer.UserID}, bson.M{
				"$inc": bson.M{"totalGamesPlayed": 1},
			})
		}
		if blackPlayer.UserID != nil {
			s.db.Users().UpdateOne(ctx, bson.M{"_id": *blackPlayer.UserID}, bson.M{
				"$inc": bson.M{"totalGamesPlayed": 1},
			})
		}
	}

	// Create match history record for ALL games (ranked and unranked)
	matchHistory := &models.MatchHistory{
		GameID:           game.ID,
		SessionID:        game.SessionID,
		IsRanked:         game.IsRanked,
		WhiteUserID:      whitePlayer.UserID,
		WhiteDisplayName: whitePlayer.DisplayName,
		WhiteAgent:       whitePlayer.AgentName,
		WhiteEloStart:    whitePlayer.EloRating,
		WhiteEloEnd:      whiteNewElo,
		WhiteEloChange:   whiteEloChange,
		BlackUserID:      blackPlayer.UserID,
		BlackDisplayName: blackPlayer.DisplayName,
		BlackAgent:       blackPlayer.AgentName,
		BlackEloStart:    blackPlayer.EloRating,
		BlackEloEnd:      blackNewElo,
		BlackEloChange:   blackEloChange,
		Winner:           game.Winner,
		WinReason:        game.WinReason,
		TotalMoves:       int(moveCount),
		GameDuration:     gameDuration,
		CompletedAt:      time.Now(),
	}

	_, err := s.db.MatchHistory().InsertOne(ctx, matchHistory)
	if err != nil {
		log.Printf("Failed to create match history for game %s: %v", game.SessionID, err)
	}

	return result, nil
}

func (s *GameCompletionService) getUserByID(ctx context.Context, id primitive.ObjectID) *models.User {
	var user models.User
	err := s.db.Users().FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		return nil
	}
	return &user
}

func (s *GameCompletionService) updateUserStats(ctx context.Context, userID primitive.ObjectID, newElo int, result elo.GameResult) {
	update := bson.M{
		"$set": bson.M{
			"eloRating": newElo,
			"updatedAt": time.Now(),
		},
		"$inc": bson.M{
			"rankedGamesPlayed": 1,
			"totalGamesPlayed":  1,
		},
	}

	// Increment win/loss/draw counter
	switch result {
	case elo.Win:
		update["$inc"].(bson.M)["rankedWins"] = 1
	case elo.Loss:
		update["$inc"].(bson.M)["rankedLosses"] = 1
	case elo.Draw:
		update["$inc"].(bson.M)["rankedDraws"] = 1
	}

	_, err := s.db.Users().UpdateOne(ctx, bson.M{"_id": userID}, update)
	if err != nil {
		log.Printf("Failed to update user %s stats: %v", userID.Hex(), err)
	}
}

func (s *GameCompletionService) getAgentRating(ctx context.Context, ownerUserID primitive.ObjectID, agentName string) *models.AgentRating {
	var ar models.AgentRating
	err := s.db.AgentRatings().FindOne(ctx, bson.M{
		"ownerUserId": ownerUserID,
		"agentName":   agentName,
	}).Decode(&ar)
	if err != nil {
		return nil
	}
	return &ar
}

func (s *GameCompletionService) updateAgentStats(ctx context.Context, ownerUserID primitive.ObjectID, agentName string, newElo int, result elo.GameResult) {
	inc := bson.M{"rankedGamesPlayed": 1}
	switch result {
	case elo.Win:
		inc["wins"] = 1
	case elo.Loss:
		inc["losses"] = 1
	case elo.Draw:
		inc["draws"] = 1
	}

	now := time.Now()
	opts := options.Update().SetUpsert(true)
	_, err := s.db.AgentRatings().UpdateOne(ctx, bson.M{
		"ownerUserId": ownerUserID,
		"agentName":   agentName,
	}, bson.M{
		"$set": bson.M{
			"eloRating": newElo,
			"updatedAt": now,
		},
		"$inc": inc,
		"$setOnInsert": bson.M{
			"createdAt": now,
		},
	}, opts)
	if err != nil {
		log.Printf("Failed to update agent rating %s/%s: %v", ownerUserID.Hex(), agentName, err)
	}
}

func (s *GameCompletionService) updateGameWithEloChanges(ctx context.Context, sessionID string, whiteChange, blackChange, whiteNew, blackNew int) {
	// Store Elo changes on the game for frontend to access
	update := bson.M{
		"$set": bson.M{
			"eloChanges": bson.M{
				"whiteChange": whiteChange,
				"blackChange": blackChange,
				"whiteNewElo": whiteNew,
				"blackNewElo": blackNew,
			},
		},
	}

	_, err := s.db.Games().UpdateOne(ctx, bson.M{"sessionId": sessionID}, update)
	if err != nil {
		log.Printf("Failed to update game %s with Elo changes: %v", sessionID, err)
	}
}
