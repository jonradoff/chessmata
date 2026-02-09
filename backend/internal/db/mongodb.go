package db

import (
	"context"
	"fmt"
	"time"

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

	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Ping the database to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	return &MongoDB{
		Client:   client,
		Database: client.Database(database),
	}, nil
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
