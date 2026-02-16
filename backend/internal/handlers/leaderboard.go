package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"chess-game/internal/db"
	"chess-game/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type LeaderboardHandler struct {
	db *db.MongoDB
}

func NewLeaderboardHandler(database *db.MongoDB) *LeaderboardHandler {
	return &LeaderboardHandler{db: database}
}

type LeaderboardEntry struct {
	Rank        int    `json:"rank"`
	DisplayName string `json:"displayName"`
	EloRating   int    `json:"eloRating"`
	Wins        int    `json:"wins"`
	Losses      int    `json:"losses"`
	Draws       int    `json:"draws"`
	GamesPlayed int    `json:"gamesPlayed"`
}

// GetLeaderboard returns the top players or agents by Elo.
// GET /api/leaderboard?type=players|agents
func (h *LeaderboardHandler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	leaderboardType := r.URL.Query().Get("type")
	if leaderboardType == "" {
		leaderboardType = "players"
	}

	var entries []LeaderboardEntry

	switch leaderboardType {
	case "agents":
		entries = h.getAgentLeaderboard(ctx)
	default:
		entries = h.getPlayerLeaderboard(ctx)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (h *LeaderboardHandler) getPlayerLeaderboard(ctx context.Context) []LeaderboardEntry {
	opts := options.Find().
		SetSort(bson.M{"eloRating": -1}).
		SetLimit(50).
		SetProjection(bson.M{
			"displayName":      1,
			"eloRating":        1,
			"rankedWins":       1,
			"rankedLosses":     1,
			"rankedDraws":      1,
			"rankedGamesPlayed": 1,
		})

	cursor, err := h.db.Users().Find(ctx, bson.M{
		"rankedGamesPlayed": bson.M{"$gt": 0},
		"isActive":          true,
	}, opts)
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)

	var users []models.User
	if err := cursor.All(ctx, &users); err != nil {
		return nil
	}

	entries := make([]LeaderboardEntry, len(users))
	for i, u := range users {
		entries[i] = LeaderboardEntry{
			Rank:        i + 1,
			DisplayName: u.DisplayName,
			EloRating:   u.EloRating,
			Wins:        u.RankedWins,
			Losses:      u.RankedLosses,
			Draws:       u.RankedDraws,
			GamesPlayed: u.RankedGamesPlayed,
		}
	}
	return entries
}

func (h *LeaderboardHandler) getAgentLeaderboard(ctx context.Context) []LeaderboardEntry {
	opts := options.Find().
		SetSort(bson.M{"eloRating": -1}).
		SetLimit(50)

	cursor, err := h.db.AgentRatings().Find(ctx, bson.M{
		"rankedGamesPlayed": bson.M{"$gt": 0},
	}, opts)
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)

	var agents []models.AgentRating
	if err := cursor.All(ctx, &agents); err != nil {
		return nil
	}

	// Collect owner user IDs for display name lookup
	ownerIDs := make([]primitive.ObjectID, 0, len(agents))
	for _, a := range agents {
		ownerIDs = append(ownerIDs, a.OwnerUserID)
	}

	// Fetch owner display names
	ownerNames := make(map[primitive.ObjectID]string)
	if len(ownerIDs) > 0 {
		userCursor, err := h.db.Users().Find(ctx, bson.M{
			"_id": bson.M{"$in": ownerIDs},
		}, options.Find().SetProjection(bson.M{"displayName": 1}))
		if err == nil {
			defer userCursor.Close(ctx)
			var users []models.User
			if userCursor.All(ctx, &users) == nil {
				for _, u := range users {
					ownerNames[u.ID] = u.DisplayName
				}
			}
		}
	}

	entries := make([]LeaderboardEntry, len(agents))
	for i, a := range agents {
		ownerName := ownerNames[a.OwnerUserID]
		if ownerName == "" {
			ownerName = "Unknown"
		}
		entries[i] = LeaderboardEntry{
			Rank:        i + 1,
			DisplayName: ownerName + ":" + a.AgentName,
			EloRating:   a.EloRating,
			Wins:        a.Wins,
			Losses:      a.Losses,
			Draws:       a.Draws,
			GamesPlayed: a.RankedGamesPlayed,
		}
	}
	return entries
}
