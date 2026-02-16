package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"chess-game/internal/game"
)

type PlayerColor string

const (
	White PlayerColor = "white"
	Black PlayerColor = "black"
)

type GameStatus string

const (
	GameStatusWaiting  GameStatus = "waiting"  // Waiting for second player
	GameStatusActive   GameStatus = "active"   // Game in progress
	GameStatusComplete GameStatus = "complete" // Game finished
)

type GameType string

const (
	GameTypeCasual      GameType = "casual"
	GameTypeMatchmaking GameType = "matchmaking"
)

type Player struct {
	ID             string              `json:"id" bson:"id"`
	UserID         *primitive.ObjectID `json:"userId,omitempty" bson:"userId,omitempty"` // nullable for anonymous
	DisplayName    string              `json:"displayName" bson:"displayName"`
	AgentName      string              `json:"agentName,omitempty" bson:"agentName,omitempty"`
	ClientSoftware string              `json:"clientSoftware,omitempty" bson:"clientSoftware,omitempty"`
	Color          PlayerColor         `json:"color" bson:"color"`
	EloRating      int                 `json:"eloRating" bson:"eloRating"`
	JoinedAt       time.Time           `json:"joinedAt" bson:"joinedAt"`
}

// EloChanges stores the rating changes after a ranked game
type EloChanges struct {
	WhiteChange int `json:"whiteChange" bson:"whiteChange"`
	BlackChange int `json:"blackChange" bson:"blackChange"`
	WhiteNewElo int `json:"whiteNewElo" bson:"whiteNewElo"`
	BlackNewElo int `json:"blackNewElo" bson:"blackNewElo"`
}

type Game struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	SessionID   string             `json:"sessionId" bson:"sessionId"`
	Players     []Player           `json:"players" bson:"players"`
	Status      GameStatus         `json:"status" bson:"status"`
	CurrentTurn PlayerColor        `json:"currentTurn" bson:"currentTurn"`
	BoardState  string             `json:"boardState" bson:"boardState"` // FEN notation
	Winner      PlayerColor        `json:"winner,omitempty" bson:"winner,omitempty"`
	WinReason   string             `json:"winReason,omitempty" bson:"winReason,omitempty"` // "checkmate", "resignation", "timeout", draw reasons
	IsRanked    bool               `json:"isRanked" bson:"isRanked"`
	GameType    GameType           `json:"gameType" bson:"gameType"`
	StartedAt   *time.Time         `json:"startedAt,omitempty" bson:"startedAt,omitempty"`
	CompletedAt *time.Time         `json:"completedAt,omitempty" bson:"completedAt,omitempty"`
	EloChanges  *EloChanges        `json:"eloChanges,omitempty" bson:"eloChanges,omitempty"`
	MoveCount   int                `json:"moveCount" bson:"moveCount"`
	CreatedAt   time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt   time.Time          `json:"updatedAt" bson:"updatedAt"`

	// Time control fields
	TimeControl     *game.TimeControl  `json:"timeControl,omitempty" bson:"timeControl,omitempty"`
	PlayerTimes     *game.PlayerTimes  `json:"playerTimes,omitempty" bson:"playerTimes,omitempty"`
	DrawOffers      *game.DrawOffers   `json:"drawOffers,omitempty" bson:"drawOffers,omitempty"`
	PositionHistory []string           `json:"positionHistory,omitempty" bson:"positionHistory,omitempty"` // FEN positions for repetition detection

	// Computed draw-claim availability (not persisted)
	CanClaimThreefold  bool `json:"canClaimThreefold,omitempty" bson:"-"`
	CanClaimFiftyMoves bool `json:"canClaimFiftyMoves,omitempty" bson:"-"`
}

type Move struct {
	ID         primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	GameID     primitive.ObjectID `json:"gameId" bson:"gameId"`
	SessionID  string             `json:"sessionId" bson:"sessionId"`
	PlayerID   string             `json:"playerId" bson:"playerId"`
	MoveNumber int                `json:"moveNumber" bson:"moveNumber"`
	From       string             `json:"from" bson:"from"`       // e.g., "e2"
	To         string             `json:"to" bson:"to"`           // e.g., "e4"
	Piece      string             `json:"piece" bson:"piece"`     // e.g., "P" for pawn
	Notation   string             `json:"notation" bson:"notation"` // Standard algebraic notation, e.g., "e4"
	Capture    bool               `json:"capture" bson:"capture"`
	Check      bool               `json:"check" bson:"check"`
	Checkmate  bool               `json:"checkmate" bson:"checkmate"`
	Promotion  string             `json:"promotion,omitempty" bson:"promotion,omitempty"`
	CreatedAt  time.Time          `json:"createdAt" bson:"createdAt"`
}

// Starting position in FEN notation
const InitialBoardFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
