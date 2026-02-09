package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"chess-game/internal/auth"
	"chess-game/internal/db"
	"chess-game/internal/middleware"
	"chess-game/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AuthHandler struct {
	db              *db.MongoDB
	jwtService      *auth.JWTService
	passwordService *auth.PasswordService
	googleOAuth     *auth.GoogleOAuthService
	oauthStates     map[string]time.Time // Simple in-memory state storage (use Redis in production)
}

func NewAuthHandler(database *db.MongoDB, jwtService *auth.JWTService, passwordService *auth.PasswordService, googleOAuth *auth.GoogleOAuthService) *AuthHandler {
	return &AuthHandler{
		db:              database,
		jwtService:      jwtService,
		passwordService: passwordService,
		googleOAuth:     googleOAuth,
		oauthStates:     make(map[string]time.Time),
	}
}

// Request/Response types
type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type AuthResponse struct {
	AccessToken  string       `json:"accessToken"`
	RefreshToken string       `json:"refreshToken"`
	User         *models.User `json:"user"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// Register creates a new user account
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate input
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.DisplayName = strings.TrimSpace(req.DisplayName)

	if req.Email == "" || req.Password == "" || req.DisplayName == "" {
		respondWithError(w, http.StatusBadRequest, "Email, password, and display name are required")
		return
	}

	// Validate password strength
	if err := h.passwordService.ValidatePasswordStrength(req.Password); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check if email already exists
	var existing models.User
	err := h.db.Users().FindOne(r.Context(), bson.M{"email": req.Email}).Decode(&existing)
	if err == nil {
		respondWithError(w, http.StatusConflict, "Email already registered")
		return
	}

	// Check if display name already exists
	err = h.db.Users().FindOne(r.Context(), bson.M{"displayName": req.DisplayName}).Decode(&existing)
	if err == nil {
		respondWithError(w, http.StatusConflict, "Display name already taken")
		return
	}

	// Hash password
	passwordHash, err := h.passwordService.HashPassword(req.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to process password")
		return
	}

	// Create user
	now := time.Now()
	user := models.User{
		ID:                primitive.NewObjectID(),
		Email:             req.Email,
		DisplayName:       req.DisplayName,
		PasswordHash:      passwordHash,
		AuthMethods:       []models.AuthMethod{models.AuthMethodPassword},
		EloRating:         models.DefaultEloRating,
		RankedGamesPlayed: 0,
		RankedWins:        0,
		RankedLosses:      0,
		RankedDraws:       0,
		TotalGamesPlayed:  0,
		IsActive:          true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	_, err = h.db.Users().InsertOne(r.Context(), user)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	// Generate tokens
	accessToken, err := h.jwtService.GenerateAccessToken(user.ID.Hex(), user.Email, user.DisplayName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate access token")
		return
	}

	refreshToken, err := h.jwtService.GenerateRefreshToken(user.ID.Hex())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate refresh token")
		return
	}

	// Store refresh token
	if err := h.storeRefreshToken(r.Context(), user.ID, refreshToken, r.UserAgent()); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to store refresh token")
		return
	}

	respondWithJSON(w, http.StatusCreated, AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         &user,
	})
}

// Login authenticates a user
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	// Find user by email
	var user models.User
	err := h.db.Users().FindOne(r.Context(), bson.M{"email": req.Email}).Decode(&user)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Check if user has password auth method
	hasPassword := false
	for _, method := range user.AuthMethods {
		if method == models.AuthMethodPassword {
			hasPassword = true
			break
		}
	}

	if !hasPassword {
		respondWithError(w, http.StatusUnauthorized, "This account uses a different authentication method")
		return
	}

	// Verify password
	if err := h.passwordService.ComparePassword(user.PasswordHash, req.Password); err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Check if user is active
	if !user.IsActive {
		respondWithError(w, http.StatusUnauthorized, "Account is inactive")
		return
	}

	// Update last login
	now := time.Now()
	user.LastLoginAt = &now
	h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{"lastLoginAt": now, "updatedAt": now},
	})

	// Generate tokens
	accessToken, err := h.jwtService.GenerateAccessToken(user.ID.Hex(), user.Email, user.DisplayName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate access token")
		return
	}

	refreshToken, err := h.jwtService.GenerateRefreshToken(user.ID.Hex())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate refresh token")
		return
	}

	// Store refresh token
	if err := h.storeRefreshToken(r.Context(), user.ID, refreshToken, r.UserAgent()); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to store refresh token")
		return
	}

	respondWithJSON(w, http.StatusOK, AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         &user,
	})
}

// Refresh generates a new access token from a refresh token
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate refresh token
	claims, err := h.jwtService.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid or expired refresh token")
		return
	}

	// Check if refresh token is in database and not revoked
	tokenHash := hashToken(req.RefreshToken)
	userID, _ := primitive.ObjectIDFromHex(claims.UserID)

	var storedToken models.RefreshToken
	err = h.db.RefreshTokens().FindOne(r.Context(), bson.M{
		"userId":    userID,
		"tokenHash": tokenHash,
		"isRevoked": false,
	}).Decode(&storedToken)

	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid refresh token")
		return
	}

	// Load user
	var user models.User
	err = h.db.Users().FindOne(r.Context(), bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "User not found")
		return
	}

	if !user.IsActive {
		respondWithError(w, http.StatusUnauthorized, "Account is inactive")
		return
	}

	// Generate new access token
	accessToken, err := h.jwtService.GenerateAccessToken(user.ID.Hex(), user.Email, user.DisplayName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate access token")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"accessToken": accessToken,
	})
}

// Logout revokes a refresh token
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	tokenHash := hashToken(req.RefreshToken)

	// Revoke the refresh token
	_, err := h.db.RefreshTokens().UpdateOne(r.Context(), bson.M{
		"userId":    user.ID,
		"tokenHash": tokenHash,
	}, bson.M{
		"$set": bson.M{"isRevoked": true},
	})

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to revoke token")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Logged out successfully",
	})
}

// GoogleOAuth initiates Google OAuth flow
func (h *AuthHandler) GoogleOAuth(w http.ResponseWriter, r *http.Request) {
	// Generate random state
	state := generateRandomState()
	h.oauthStates[state] = time.Now().Add(10 * time.Minute)

	// Clean up old states (simple cleanup)
	go h.cleanupOAuthStates()

	url := h.googleOAuth.GetAuthURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// GoogleOAuthCallback handles Google OAuth callback
func (h *AuthHandler) GoogleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	// Validate state
	expiry, exists := h.oauthStates[state]
	if !exists || time.Now().After(expiry) {
		respondWithError(w, http.StatusBadRequest, "Invalid or expired OAuth state")
		return
	}
	delete(h.oauthStates, state)

	// Exchange code for token
	token, err := h.googleOAuth.ExchangeCode(r.Context(), code)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to exchange OAuth code")
		return
	}

	// Get user info from Google
	userInfo, err := h.googleOAuth.GetUserInfo(r.Context(), token)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get user info")
		return
	}

	// Check if user exists by Google ID
	var user models.User
	err = h.db.Users().FindOne(r.Context(), bson.M{"googleId": userInfo.ID}).Decode(&user)

	if err != nil {
		// Check if email exists (link accounts)
		err = h.db.Users().FindOne(r.Context(), bson.M{"email": strings.ToLower(userInfo.Email)}).Decode(&user)

		if err != nil {
			// Create new user
			now := time.Now()
			displayName := userInfo.Name
			// Ensure unique display name
			baseName := displayName
			counter := 1
			for {
				var existing models.User
				err := h.db.Users().FindOne(r.Context(), bson.M{"displayName": displayName}).Decode(&existing)
				if err != nil {
					break // Display name available
				}
				displayName = fmt.Sprintf("%s%d", baseName, counter)
				counter++
			}

			user = models.User{
				ID:                primitive.NewObjectID(),
				Email:             strings.ToLower(userInfo.Email),
				DisplayName:       displayName,
				GoogleID:          userInfo.ID,
				AuthMethods:       []models.AuthMethod{models.AuthMethodGoogle},
				EloRating:         models.DefaultEloRating,
				RankedGamesPlayed: 0,
				RankedWins:        0,
				RankedLosses:      0,
				RankedDraws:       0,
				TotalGamesPlayed:  0,
				IsActive:          true,
				CreatedAt:         now,
				UpdatedAt:         now,
			}

			_, err = h.db.Users().InsertOne(r.Context(), user)
			if err != nil {
				respondWithError(w, http.StatusInternalServerError, "Failed to create user")
				return
			}
		} else {
			// Link Google account to existing user
			hasGoogle := false
			for _, method := range user.AuthMethods {
				if method == models.AuthMethodGoogle {
					hasGoogle = true
					break
				}
			}

			if !hasGoogle {
				user.AuthMethods = append(user.AuthMethods, models.AuthMethodGoogle)
				user.GoogleID = userInfo.ID
				user.UpdatedAt = time.Now()

				_, err = h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
					"$set": bson.M{
						"googleId":    user.GoogleID,
						"authMethods": user.AuthMethods,
						"updatedAt":   user.UpdatedAt,
					},
				})

				if err != nil {
					respondWithError(w, http.StatusInternalServerError, "Failed to link Google account")
					return
				}
			}
		}
	}

	// Update last login
	now := time.Now()
	user.LastLoginAt = &now
	h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{"lastLoginAt": now, "updatedAt": now},
	})

	// Generate tokens
	accessToken, err := h.jwtService.GenerateAccessToken(user.ID.Hex(), user.Email, user.DisplayName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate access token")
		return
	}

	refreshToken, err := h.jwtService.GenerateRefreshToken(user.ID.Hex())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate refresh token")
		return
	}

	// Store refresh token
	if err := h.storeRefreshToken(r.Context(), user.ID, refreshToken, r.UserAgent()); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to store refresh token")
		return
	}

	// Redirect to frontend with tokens in URL (the frontend will extract and store them)
	frontendURL := fmt.Sprintf("http://localhost:9030/auth/callback?access_token=%s&refresh_token=%s",
		accessToken, refreshToken)
	http.Redirect(w, r, frontendURL, http.StatusTemporaryRedirect)
}

// GetMe returns the current user's information
func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	respondWithJSON(w, http.StatusOK, user)
}

// Helper functions

func (h *AuthHandler) storeRefreshToken(ctx context.Context, userID primitive.ObjectID, token string, deviceInfo string) error {
	tokenHash := hashToken(token)
	refreshToken := models.RefreshToken{
		ID:         primitive.NewObjectID(),
		UserID:     userID,
		TokenHash:  tokenHash,
		ExpiresAt:  time.Now().Add(h.jwtService.GetRefreshTTL()),
		CreatedAt:  time.Now(),
		IsRevoked:  false,
		DeviceInfo: deviceInfo,
	}

	_, err := h.db.RefreshTokens().InsertOne(ctx, refreshToken)
	return err
}

func (h *AuthHandler) cleanupOAuthStates() {
	now := time.Now()
	for state, expiry := range h.oauthStates {
		if now.After(expiry) {
			delete(h.oauthStates, state)
		}
	}
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func generateRandomState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, ErrorResponse{Error: message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
