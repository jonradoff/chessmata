package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AgentRating stores Elo and stats for an agent, keyed by (ownerUserId, agentName).
type AgentRating struct {
	ID                primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	OwnerUserID       primitive.ObjectID `json:"ownerUserId" bson:"ownerUserId"`
	AgentName         string             `json:"agentName" bson:"agentName"`
	EloRating         int                `json:"eloRating" bson:"eloRating"`
	RankedGamesPlayed int                `json:"rankedGamesPlayed" bson:"rankedGamesPlayed"`
	Wins              int                `json:"wins" bson:"wins"`
	Losses            int                `json:"losses" bson:"losses"`
	Draws             int                `json:"draws" bson:"draws"`
	CreatedAt         time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt         time.Time          `json:"updatedAt" bson:"updatedAt"`
}
