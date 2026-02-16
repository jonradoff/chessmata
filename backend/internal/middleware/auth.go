package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"log"
	"net/http"
	"strings"
	"time"

	"chess-game/internal/auth"
	"chess-game/internal/db"
	"chess-game/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type contextKey string

const (
	UserContextKey contextKey = "user"
)

type AuthMiddleware struct {
	jwtService *auth.JWTService
	db         *db.MongoDB
}

func NewAuthMiddleware(jwtService *auth.JWTService, database *db.MongoDB) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService: jwtService,
		db:         database,
	}
}

// RequireAuth validates JWT and loads user into context
// Returns 401 if token is missing or invalid
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Check Bearer prefix
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]

		// Check if this is an API key (cmk_ prefix)
		if strings.HasPrefix(tokenString, "cmk_") {
			user := m.authenticateApiKey(r.Context(), tokenString)
			if user == nil {
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Validate JWT token
		claims, err := m.jwtService.ValidateAccessToken(tokenString)
		if err != nil {
			if err == auth.ErrExpiredToken {
				http.Error(w, "Token has expired", http.StatusUnauthorized)
				return
			}
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Check if this access token has been revoked (e.g. user logged out)
		if m.isTokenRevoked(r.Context(), tokenString) {
			http.Error(w, "Token has been revoked", http.StatusUnauthorized)
			return
		}

		// Load user from database
		userID, err := primitive.ObjectIDFromHex(claims.UserID)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusUnauthorized)
			return
		}

		var user models.User
		err = m.db.Users().FindOne(r.Context(), bson.M{"_id": userID}).Decode(&user)
		if err != nil {
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		// Check if user is active
		if !user.IsActive {
			http.Error(w, "User account is inactive", http.StatusUnauthorized)
			return
		}

		// Add user to context
		ctx := context.WithValue(r.Context(), UserContextKey, &user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuth validates JWT if present, but allows request to continue without auth
// Useful for endpoints that work for both authenticated and anonymous users
func (m *AuthMiddleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// No auth header, continue without user
			next.ServeHTTP(w, r)
			return
		}

		// Check Bearer prefix
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			// Invalid format, continue without user
			next.ServeHTTP(w, r)
			return
		}

		tokenString := parts[1]

		// Check if this is an API key (cmk_ prefix)
		if strings.HasPrefix(tokenString, "cmk_") {
			user := m.authenticateApiKey(r.Context(), tokenString)
			if user != nil {
				ctx := context.WithValue(r.Context(), UserContextKey, user)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			// Invalid API key, continue without user
			next.ServeHTTP(w, r)
			return
		}

		// Validate JWT token
		claims, err := m.jwtService.ValidateAccessToken(tokenString)
		if err != nil {
			// Invalid token, continue without user
			next.ServeHTTP(w, r)
			return
		}

		// Check if this access token has been revoked (e.g. user logged out)
		if m.isTokenRevoked(r.Context(), tokenString) {
			// Revoked token, continue without user
			next.ServeHTTP(w, r)
			return
		}

		// Load user from database
		userID, err := primitive.ObjectIDFromHex(claims.UserID)
		if err != nil {
			// Invalid user ID, continue without user
			next.ServeHTTP(w, r)
			return
		}

		var user models.User
		err = m.db.Users().FindOne(r.Context(), bson.M{"_id": userID}).Decode(&user)
		if err != nil {
			// User not found, continue without user
			next.ServeHTTP(w, r)
			return
		}

		// Check if user is active
		if !user.IsActive {
			// Inactive user, continue without user
			next.ServeHTTP(w, r)
			return
		}

		// Add user to context
		ctx := context.WithValue(r.Context(), UserContextKey, &user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authenticateApiKey hashes the token and looks up the API key + user
func (m *AuthMiddleware) authenticateApiKey(ctx context.Context, token string) *models.User {
	hash := sha256.Sum256([]byte(token))
	keyHash := base64.StdEncoding.EncodeToString(hash[:])

	var apiKey models.ApiKey
	err := m.db.ApiKeys().FindOne(ctx, bson.M{"keyHash": keyHash}).Decode(&apiKey)
	if err != nil {
		return nil
	}

	var user models.User
	err = m.db.Users().FindOne(ctx, bson.M{"_id": apiKey.UserID}).Decode(&user)
	if err != nil || !user.IsActive {
		return nil
	}

	// Update lastUsedAt in background
	go func() {
		now := time.Now()
		m.db.ApiKeys().UpdateOne(context.Background(), bson.M{"_id": apiKey.ID}, bson.M{
			"$set": bson.M{"lastUsedAt": now},
		})
	}()

	return &user
}

// isTokenRevoked checks if the given raw token has been revoked (e.g. on logout).
func (m *AuthMiddleware) isTokenRevoked(ctx context.Context, rawToken string) bool {
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := base64.StdEncoding.EncodeToString(hash[:])

	count, err := m.db.RevokedTokens().CountDocuments(ctx, bson.M{"tokenHash": tokenHash})
	if err != nil {
		log.Printf("Warning: revoked-token lookup failed: %v", err)
		return false // fail-open to avoid locking everyone out on DB hiccup
	}
	return count > 0
}

// GetUserFromContext retrieves the authenticated user from the request context
func GetUserFromContext(ctx context.Context) (*models.User, bool) {
	user, ok := ctx.Value(UserContextKey).(*models.User)
	return user, ok
}
