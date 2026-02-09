package elo

import (
	"math"
)

type GameResult int

const (
	Loss GameResult = 0
	Draw GameResult = 1
	Win  GameResult = 2
)

const (
	// K-factors based on number of games played
	KFactorNewbie = 32 // < 30 games
	KFactorActive = 24 // 30-100 games
	KFactorExpert = 16 // > 100 games

	// Rating bounds
	MinRating = 100
	MaxRating = 3000
)

type Calculator struct{}

func NewCalculator() *Calculator {
	return &Calculator{}
}

// CalculateNewRating calculates the new Elo rating for a player
// playerRating: current rating of the player
// opponentRating: current rating of the opponent
// result: GameResult (Win=2, Draw=1, Loss=0)
// gamesPlayed: number of ranked games the player has played (used for K-factor)
func (c *Calculator) CalculateNewRating(playerRating, opponentRating int, result GameResult, gamesPlayed int) int {
	// Determine K-factor based on games played
	kFactor := c.getKFactor(gamesPlayed)

	// Calculate expected score
	expectedScore := c.calculateExpectedScore(playerRating, opponentRating)

	// Calculate actual score (1 for win, 0.5 for draw, 0 for loss)
	var actualScore float64
	switch result {
	case Win:
		actualScore = 1.0
	case Draw:
		actualScore = 0.5
	case Loss:
		actualScore = 0.0
	}

	// Calculate rating change: ΔR = K × (S - E)
	ratingChange := float64(kFactor) * (actualScore - expectedScore)

	// Apply rating change
	newRating := playerRating + int(math.Round(ratingChange))

	// Enforce rating bounds
	if newRating < MinRating {
		newRating = MinRating
	}
	if newRating > MaxRating {
		newRating = MaxRating
	}

	return newRating
}

// CalculateRatingChange returns just the change in rating (positive or negative)
func (c *Calculator) CalculateRatingChange(playerRating, opponentRating int, result GameResult, gamesPlayed int) int {
	newRating := c.CalculateNewRating(playerRating, opponentRating, result, gamesPlayed)
	return newRating - playerRating
}

// calculateExpectedScore calculates the expected score using the Elo formula
// E = 1 / (1 + 10^((OpponentRating - PlayerRating) / 400))
func (c *Calculator) calculateExpectedScore(playerRating, opponentRating int) float64 {
	exponent := float64(opponentRating-playerRating) / 400.0
	return 1.0 / (1.0 + math.Pow(10, exponent))
}

// getKFactor returns the appropriate K-factor based on games played
func (c *Calculator) getKFactor(gamesPlayed int) int {
	switch {
	case gamesPlayed < 30:
		return KFactorNewbie
	case gamesPlayed < 100:
		return KFactorActive
	default:
		return KFactorExpert
	}
}

// GetGameResultFromWinner converts a winner color to game results for both players
// Returns (whiteResult, blackResult)
func GetGameResultFromWinner(winner string) (GameResult, GameResult) {
	switch winner {
	case "white":
		return Win, Loss
	case "black":
		return Loss, Win
	default: // draw or empty
		return Draw, Draw
	}
}
