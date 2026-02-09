package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"chess-game/internal/config"
	"chess-game/internal/db"
)

func main() {
	// Load config
	cfg, err := config.Load("dev")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to MongoDB
	mongodb, err := db.NewMongoDB(cfg.MongoDB.URI, cfg.MongoDB.Database)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mongodb.Close(ctx)
	}()

	ctx := context.Background()

	// Delete all games
	gamesResult, err := mongodb.Games().DeleteMany(ctx, map[string]interface{}{})
	if err != nil {
		log.Fatalf("Failed to delete games: %v", err)
	}
	fmt.Printf("Deleted %d games\n", gamesResult.DeletedCount)

	// Delete all moves
	movesResult, err := mongodb.Moves().DeleteMany(ctx, map[string]interface{}{})
	if err != nil {
		log.Fatalf("Failed to delete moves: %v", err)
	}
	fmt.Printf("Deleted %d moves\n", movesResult.DeletedCount)

	fmt.Println("Database cleared successfully")
}
