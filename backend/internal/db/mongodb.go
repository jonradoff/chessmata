package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDB struct {
	Client   *mongo.Client
	Database *mongo.Database
}

func NewMongoDB(uri, database string) (*MongoDB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(500).
		SetMinPoolSize(10).
		SetMaxConnIdleTime(5 * time.Minute)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Ping the database to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	db := &MongoDB{
		Client:   client,
		Database: client.Database(database),
	}

	// Create indexes in the background (non-blocking)
	go db.ensureIndexes()

	return db, nil
}

// ensureIndexes creates all required indexes. Called once on startup.
func (m *MongoDB) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	indexes := []struct {
		collection string
		models     []mongo.IndexModel
	}{
		{
			"games",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "sessionId", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "status", Value: 1}, {Key: "updatedAt", Value: -1}}},
			},
		},
		{
			"moves",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "sessionId", Value: 1}, {Key: "moveNumber", Value: 1}}},
			},
		},
		{
			"users",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true).SetSparse(true)},
				{Keys: bson.D{{Key: "displayName", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "googleId", Value: 1}}, Options: options.Index().SetSparse(true)},
				{Keys: bson.D{{Key: "eloRating", Value: -1}}},
			},
		},
		{
			"matchmaking_queue",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "status", Value: 1}, {Key: "joinedAt", Value: 1}}},
				{Keys: bson.D{{Key: "connectionId", Value: 1}}},
				{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
			},
		},
		{
			"match_history",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "whiteUserId", Value: 1}, {Key: "completedAt", Value: -1}}},
				{Keys: bson.D{{Key: "blackUserId", Value: 1}, {Key: "completedAt", Value: -1}}},
				{Keys: bson.D{{Key: "sessionId", Value: 1}}},
			},
		},
		{
			"agent_ratings",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "ownerUserId", Value: 1}, {Key: "agentName", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "eloRating", Value: -1}}},
			},
		},
		{
			"refresh_tokens",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "userId", Value: 1}}},
				{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
			},
		},
		{
			"verification_tokens",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "type", Value: 1}}},
				{Keys: bson.D{{Key: "token", Value: 1}}},
				{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
			},
		},
		{
			"api_keys",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "keyHash", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "userId", Value: 1}}},
			},
		},
		{
			"ws_events",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "createdAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(60)},
			},
		},
		{
			"oauth_states",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
			},
		},
		{
			"audit_log",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "createdAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(90 * 24 * 3600)}, // 90-day retention
				{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}}},
			},
		},
		{
			"revoked_tokens",
			[]mongo.IndexModel{
				{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
				{Keys: bson.D{{Key: "tokenHash", Value: 1}}, Options: options.Index().SetUnique(true)},
			},
		},
	}

	for _, idx := range indexes {
		coll := m.Database.Collection(idx.collection)
		_, err := coll.Indexes().CreateMany(ctx, idx.models)
		if err != nil {
			log.Printf("Warning: failed to create indexes on %s: %v", idx.collection, err)
		}
	}

	log.Println("Database indexes ensured")
}

func (m *MongoDB) Close(ctx context.Context) error {
	return m.Client.Disconnect(ctx)
}

func (m *MongoDB) Games() *mongo.Collection {
	return m.Database.Collection("games")
}

func (m *MongoDB) Moves() *mongo.Collection {
	return m.Database.Collection("moves")
}

func (m *MongoDB) Users() *mongo.Collection {
	return m.Database.Collection("users")
}

func (m *MongoDB) RefreshTokens() *mongo.Collection {
	return m.Database.Collection("refresh_tokens")
}

func (m *MongoDB) MatchHistory() *mongo.Collection {
	return m.Database.Collection("match_history")
}

func (m *MongoDB) MatchmakingQueue() *mongo.Collection {
	return m.Database.Collection("matchmaking_queue")
}

func (m *MongoDB) VerificationTokens() *mongo.Collection {
	return m.Database.Collection("verification_tokens")
}

func (m *MongoDB) ApiKeys() *mongo.Collection {
	return m.Database.Collection("api_keys")
}

func (m *MongoDB) AgentRatings() *mongo.Collection {
	return m.Database.Collection("agent_ratings")
}

func (m *MongoDB) WSEvents() *mongo.Collection {
	return m.Database.Collection("ws_events")
}

func (m *MongoDB) CleanupLocks() *mongo.Collection {
	return m.Database.Collection("cleanup_locks")
}

func (m *MongoDB) OAuthStates() *mongo.Collection {
	return m.Database.Collection("oauth_states")
}

func (m *MongoDB) AuditLog() *mongo.Collection {
	return m.Database.Collection("audit_log")
}

func (m *MongoDB) RevokedTokens() *mongo.Collection {
	return m.Database.Collection("revoked_tokens")
}
