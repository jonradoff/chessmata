package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"chess-game/internal/game"
)

type AuthMethod string

const (
	AuthMethodPassword AuthMethod = "password"
	AuthMethodGoogle   AuthMethod = "google"
)

type User struct {
	ID                    primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Email                 string             `json:"email" bson:"email"`
	DisplayName           string             `json:"displayName" bson:"displayName"`
	PasswordHash          string             `json:"-" bson:"passwordHash,omitempty"`  // Never send to client
	GoogleID              string             `json:"-" bson:"googleId,omitempty"`       // Never send to client
	AuthMethods           []AuthMethod       `json:"authMethods" bson:"authMethods"`
	EmailVerified         bool               `json:"emailVerified" bson:"emailVerified"`
	EloRating             int                `json:"eloRating" bson:"eloRating"`
	RankedGamesPlayed     int                `json:"rankedGamesPlayed" bson:"rankedGamesPlayed"`
	RankedWins            int                `json:"rankedWins" bson:"rankedWins"`
	RankedLosses          int                `json:"rankedLosses" bson:"rankedLosses"`
	RankedDraws           int                `json:"rankedDraws" bson:"rankedDraws"`
	TotalGamesPlayed      int                `json:"totalGamesPlayed" bson:"totalGamesPlayed"`
	IsActive              bool               `json:"isActive" bson:"isActive"`
	CreatedAt             time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt             time.Time          `json:"updatedAt" bson:"updatedAt"`
	LastLoginAt           *time.Time         `json:"lastLoginAt,omitempty" bson:"lastLoginAt,omitempty"`
	LastDisplayNameChange *time.Time         `json:"lastDisplayNameChange,omitempty" bson:"lastDisplayNameChange,omitempty"`
	DisplayNameChanges    int                `json:"displayNameChanges" bson:"displayNameChanges"` // Number of times display name was changed (0 = initial/generated)
	LastVerificationSent  *time.Time         `json:"-" bson:"lastVerificationSent,omitempty"`      // Never send to client
	FailedLoginAttempts   int                `json:"-" bson:"failedLoginAttempts"`
	AccountLockedUntil    *time.Time         `json:"-" bson:"accountLockedUntil,omitempty"`

	// User preferences
	Preferences *UserPreferences `json:"preferences,omitempty" bson:"preferences,omitempty"`
}

// UserPreferences stores user game preferences
type UserPreferences struct {
	AutoDeclineDraws      bool                   `json:"autoDeclineDraws" bson:"autoDeclineDraws"`
	PreferredTimeControls []game.TimeControlMode `json:"preferredTimeControls,omitempty" bson:"preferredTimeControls,omitempty"`
}

type TokenType string

const (
	TokenTypeEmailVerification TokenType = "email_verification"
	TokenTypePasswordReset     TokenType = "password_reset"
)

type VerificationToken struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	UserID    primitive.ObjectID `json:"userId" bson:"userId"`
	Token     string             `json:"-" bson:"token"`
	Type      TokenType          `json:"type" bson:"type"`
	ExpiresAt time.Time          `json:"expiresAt" bson:"expiresAt"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
	UsedAt    *time.Time         `json:"usedAt,omitempty" bson:"usedAt,omitempty"`
}

type RefreshToken struct {
	ID         primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	UserID     primitive.ObjectID `json:"userId" bson:"userId"`
	TokenHash  string             `json:"-" bson:"tokenHash"` // Never send to client
	ExpiresAt  time.Time          `json:"expiresAt" bson:"expiresAt"`
	CreatedAt  time.Time          `json:"createdAt" bson:"createdAt"`
	IsRevoked  bool               `json:"isRevoked" bson:"isRevoked"`
	DeviceInfo string             `json:"deviceInfo,omitempty" bson:"deviceInfo,omitempty"`
}

type ApiKey struct {
	ID         primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	UserID     primitive.ObjectID `json:"userId" bson:"userId"`
	Name       string             `json:"name" bson:"name"`
	KeyPrefix  string             `json:"keyPrefix" bson:"keyPrefix"`   // First 12 chars (cmk_ + 8)
	KeyHash    string             `json:"-" bson:"keyHash"`             // SHA-256 hash, never serialized
	CreatedAt  time.Time          `json:"createdAt" bson:"createdAt"`
	LastUsedAt *time.Time         `json:"lastUsedAt,omitempty" bson:"lastUsedAt,omitempty"`
}

type MatchHistory struct {
	ID              primitive.ObjectID  `json:"id" bson:"_id,omitempty"`
	GameID          primitive.ObjectID  `json:"gameId" bson:"gameId"`
	SessionID       string              `json:"sessionId" bson:"sessionId"`
	IsRanked        bool                `json:"isRanked" bson:"isRanked"`
	WhiteUserID     *primitive.ObjectID `json:"whiteUserId,omitempty" bson:"whiteUserId,omitempty"`
	WhiteDisplayName string             `json:"whiteDisplayName" bson:"whiteDisplayName"`
	WhiteAgent      string              `json:"whiteAgent,omitempty" bson:"whiteAgent,omitempty"`
	WhiteEloStart   int                 `json:"whiteEloStart" bson:"whiteEloStart"`
	WhiteEloEnd     int                 `json:"whiteEloEnd" bson:"whiteEloEnd"`
	WhiteEloChange  int                 `json:"whiteEloChange" bson:"whiteEloChange"`
	BlackUserID     *primitive.ObjectID `json:"blackUserId,omitempty" bson:"blackUserId,omitempty"`
	BlackDisplayName string             `json:"blackDisplayName" bson:"blackDisplayName"`
	BlackAgent      string              `json:"blackAgent,omitempty" bson:"blackAgent,omitempty"`
	BlackEloStart   int                 `json:"blackEloStart" bson:"blackEloStart"`
	BlackEloEnd     int                 `json:"blackEloEnd" bson:"blackEloEnd"`
	BlackEloChange  int                 `json:"blackEloChange" bson:"blackEloChange"`
	Winner          PlayerColor         `json:"winner,omitempty" bson:"winner,omitempty"` // "white", "black", or empty for draw
	WinReason       string              `json:"winReason" bson:"winReason"` // "checkmate", "resignation", "stalemate", "timeout"
	TotalMoves      int                 `json:"totalMoves" bson:"totalMoves"`
	GameDuration    int                 `json:"gameDuration" bson:"gameDuration"` // in seconds
	CompletedAt     time.Time           `json:"completedAt" bson:"completedAt"`
}

type OpponentType string

const (
	OpponentTypeHuman  OpponentType = "human"
	OpponentTypeAI     OpponentType = "ai"
	OpponentTypeEither OpponentType = "either"
)

type QueueStatus string

const (
	QueueStatusWaiting QueueStatus = "waiting"
	QueueStatusMatched QueueStatus = "matched"
	QueueStatusExpired QueueStatus = "expired"
)

type MatchmakingQueue struct {
	ID               primitive.ObjectID     `json:"id" bson:"_id,omitempty"`
	UserID           *primitive.ObjectID    `json:"userId,omitempty" bson:"userId,omitempty"` // nullable for anonymous
	ConnectionID     string                 `json:"connectionId" bson:"connectionId"`
	DisplayName      string                 `json:"displayName" bson:"displayName"`
	AgentName        string                 `json:"agentName,omitempty" bson:"agentName,omitempty"`
	EngineName       string                 `json:"engineName,omitempty" bson:"engineName,omitempty"`
	ClientSoftware   string                 `json:"clientSoftware,omitempty" bson:"clientSoftware,omitempty"`
	IsBuiltinAgent   bool                   `json:"isBuiltinAgent,omitempty" bson:"isBuiltinAgent,omitempty"`
	IsRanked         bool                   `json:"isRanked" bson:"isRanked"`
	CurrentElo       int                    `json:"currentElo" bson:"currentElo"`
	PreferredColor   *PlayerColor           `json:"preferredColor,omitempty" bson:"preferredColor,omitempty"` // nullable
	OpponentType     OpponentType           `json:"opponentType" bson:"opponentType"`
	TimeControls     []game.TimeControlMode `json:"timeControls" bson:"timeControls"` // Multi-select time control preferences
	JoinedAt         time.Time              `json:"joinedAt" bson:"joinedAt"`
	ExpiresAt        time.Time              `json:"expiresAt" bson:"expiresAt"`
	Status           QueueStatus            `json:"status" bson:"status"`
	MatchedWith      *primitive.ObjectID    `json:"matchedWith,omitempty" bson:"matchedWith,omitempty"`
	MatchedSessionID string                 `json:"matchedSessionId,omitempty" bson:"matchedSessionId,omitempty"` // Game session ID when matched
}

// Default values
const (
	DefaultEloRating    = 1200
	DefaultQueueTimeout = 10 * time.Minute
)
