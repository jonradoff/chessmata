package utils

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Word lists for generating random display names
var adjectives = []string{
	"Swift", "Brave", "Clever", "Noble", "Mighty", "Silent", "Golden", "Silver",
	"Crystal", "Shadow", "Crimson", "Azure", "Cosmic", "Ancient", "Mystic", "Royal",
	"Fierce", "Gentle", "Wild", "Calm", "Bold", "Wise", "Quick", "Keen",
	"Dark", "Light", "Storm", "Frost", "Fire", "Iron", "Steel", "Stone",
	"Thunder", "Winter", "Summer", "Spring", "Autumn", "Night", "Dawn", "Dusk",
	"Lunar", "Solar", "Stellar", "Void", "Phantom", "Ghost", "Spirit", "Soul",
	"Eternal", "Infinite", "Primal", "Elder", "Young", "Grand", "Prime", "Alpha",
	"Omega", "Delta", "Sigma", "Zeta", "Beta", "Gamma", "Apex", "Echo",
}

var nouns = []string{
	"Knight", "Bishop", "Rook", "Queen", "King", "Pawn", "Dragon", "Phoenix",
	"Wolf", "Bear", "Eagle", "Hawk", "Lion", "Tiger", "Falcon", "Serpent",
	"Wizard", "Mage", "Sage", "Oracle", "Scholar", "Hunter", "Warrior", "Champion",
	"Castle", "Tower", "Crown", "Throne", "Sword", "Shield", "Arrow", "Bow",
	"Storm", "Thunder", "Lightning", "Blaze", "Frost", "Shadow", "Light", "Star",
	"Moon", "Sun", "Comet", "Nova", "Nebula", "Galaxy", "Cosmos", "Void",
	"Guardian", "Sentinel", "Watcher", "Keeper", "Seeker", "Rider", "Walker", "Runner",
	"Master", "Lord", "Baron", "Duke", "Prince", "Count", "Marshal", "Captain",
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

// GenerateRandomDisplayName generates a random display name in format "AdjectiveNoun123"
func GenerateRandomDisplayName() string {
	adjective := adjectives[rand.Intn(len(adjectives))]
	noun := nouns[rand.Intn(len(nouns))]
	number := rand.Intn(1000) // 0-999
	return fmt.Sprintf("%s%s%d", adjective, noun, number)
}

// GenerateUniqueDisplayName generates a display name that doesn't exist in the database
func GenerateUniqueDisplayName(ctx context.Context, usersCollection *mongo.Collection) (string, error) {
	maxAttempts := 100
	for i := 0; i < maxAttempts; i++ {
		name := GenerateRandomDisplayName()

		// Check if it exists
		var existing bson.M
		err := usersCollection.FindOne(ctx, bson.M{"displayName": name}).Decode(&existing)
		if err == mongo.ErrNoDocuments {
			// Name is available
			return name, nil
		}
		if err != nil && err != mongo.ErrNoDocuments {
			return "", fmt.Errorf("database error checking display name: %w", err)
		}
		// Name taken, try again
	}

	// Fallback: add more random digits
	for i := 0; i < 100; i++ {
		adjective := adjectives[rand.Intn(len(adjectives))]
		noun := nouns[rand.Intn(len(nouns))]
		number := rand.Intn(100000) // Larger range
		name := fmt.Sprintf("%s%s%d", adjective, noun, number)

		var existing bson.M
		err := usersCollection.FindOne(ctx, bson.M{"displayName": name}).Decode(&existing)
		if err == mongo.ErrNoDocuments {
			return name, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique display name after many attempts")
}
