package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"chess-game/internal/audit"
	"chess-game/internal/auth"
	"chess-game/internal/db"
	"chess-game/internal/email"
	"chess-game/internal/game"
	"chess-game/internal/middleware"
	"chess-game/internal/models"
	"chess-game/internal/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AuthHandler struct {
	db              *db.MongoDB
	jwtService      *auth.JWTService
	passwordService *auth.PasswordService
	googleOAuth     *auth.GoogleOAuthService
	emailService    *email.ResendService
	frontendURL     string
}

func NewAuthHandler(database *db.MongoDB, jwtService *auth.JWTService, passwordService *auth.PasswordService, googleOAuth *auth.GoogleOAuthService, emailService *email.ResendService, frontendURL string) *AuthHandler {
	return &AuthHandler{
		db:              database,
		jwtService:      jwtService,
		passwordService: passwordService,
		googleOAuth:     googleOAuth,
		emailService:    emailService,
		frontendURL:     frontendURL,
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

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type VerifyEmailRequest struct {
	Token string `json:"token"`
}

type ResendVerificationRequest struct {
	Email string `json:"email"`
}

type CheckDisplayNameRequest struct {
	DisplayName string `json:"displayName"`
}

type ChangeDisplayNameRequest struct {
	DisplayName string `json:"displayName"`
}

type UpdatePreferencesRequest struct {
	AutoDeclineDraws      *bool    `json:"autoDeclineDraws,omitempty"`
	PreferredTimeControls []string `json:"preferredTimeControls,omitempty"`
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

	// Check if email already exists (generic message to prevent enumeration)
	var existing models.User
	err := h.db.Users().FindOne(r.Context(), bson.M{"email": req.Email}).Decode(&existing)
	if err == nil {
		respondWithError(w, http.StatusConflict, "Unable to create account with these details")
		return
	}

	// Check if display name already exists (generic message to prevent enumeration)
	err = h.db.Users().FindOne(r.Context(), bson.M{"displayName": req.DisplayName}).Decode(&existing)
	if err == nil {
		respondWithError(w, http.StatusConflict, "Unable to create account with these details")
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
		EmailVerified:     false,
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

	// Create and send verification email
	verificationToken := generateRandomToken()
	verification := models.VerificationToken{
		ID:        primitive.NewObjectID(),
		UserID:    user.ID,
		Token:     verificationToken,
		Type:      models.TokenTypeEmailVerification,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}
	h.db.VerificationTokens().InsertOne(r.Context(), verification)

	// Send verification email (don't block on error)
	go func() {
		if h.emailService != nil {
			log.Printf("Sending verification email to %s", user.Email)
			if err := h.emailService.SendVerificationEmail(user.Email, user.DisplayName, verificationToken); err != nil {
				log.Printf("Failed to send verification email to %s: %v", user.Email, err)
			}
		} else {
			log.Printf("Email service not configured, skipping verification email to %s", user.Email)
		}
	}()

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

	audit.LogEvent(h.db, audit.EventRegister, &user.ID, user.Email, r, "")

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
		audit.LogEvent(h.db, audit.EventLoginFailed, nil, req.Email, r, "user not found")
		respondWithError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Check account lockout
	if user.AccountLockedUntil != nil && time.Now().Before(*user.AccountLockedUntil) {
		audit.LogEvent(h.db, audit.EventLoginFailed, &user.ID, req.Email, r, "account locked")
		respondWithError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Check if user has password auth method (generic message to prevent method enumeration)
	hasPassword := false
	for _, method := range user.AuthMethods {
		if method == models.AuthMethodPassword {
			hasPassword = true
			break
		}
	}

	if !hasPassword {
		audit.LogEvent(h.db, audit.EventLoginFailed, &user.ID, req.Email, r, "no password method")
		respondWithError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Verify password
	if err := h.passwordService.ComparePassword(user.PasswordHash, req.Password); err != nil {
		// Increment failed login attempts
		now := time.Now()
		updateFields := bson.M{
			"failedLoginAttempts": user.FailedLoginAttempts + 1,
			"updatedAt":           now,
		}
		if user.FailedLoginAttempts+1 >= 5 {
			lockUntil := now.Add(15 * time.Minute)
			updateFields["accountLockedUntil"] = lockUntil
			audit.LogEvent(h.db, audit.EventAccountLocked, &user.ID, req.Email, r,
				fmt.Sprintf("locked until %s after %d failed attempts", lockUntil.Format(time.RFC3339), user.FailedLoginAttempts+1))
		}
		h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{"$set": updateFields})

		audit.LogEvent(h.db, audit.EventLoginFailed, &user.ID, req.Email, r, "wrong password")
		respondWithError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Check if user is active
	if !user.IsActive {
		respondWithError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Successful login — reset failed attempts and update last login
	now := time.Now()
	user.LastLoginAt = &now
	h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{
			"lastLoginAt":         now,
			"updatedAt":           now,
			"failedLoginAttempts": 0,
			"accountLockedUntil":  nil,
		},
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

	audit.LogEvent(h.db, audit.EventLoginSuccess, &user.ID, user.Email, r, "")

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

// Logout revokes a refresh token and the current access token
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

	// Revoke the current access token so it can't be reused
	authHeader := r.Header.Get("Authorization")
	if parts := strings.SplitN(authHeader, " ", 2); len(parts) == 2 {
		accessTokenHash := hashToken(parts[1])
		// Store with a TTL so it auto-expires when the token would have expired
		h.db.RevokedTokens().InsertOne(r.Context(), bson.M{
			"tokenHash": accessTokenHash,
			"userId":    user.ID,
			"createdAt": time.Now(),
			"expiresAt": time.Now().Add(24 * time.Hour), // Match access token max lifetime
		})
	}

	audit.LogEvent(h.db, audit.EventLogout, &user.ID, user.Email, r, "")

	respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Logged out successfully",
	})
}

// GoogleOAuth initiates Google OAuth flow
func (h *AuthHandler) GoogleOAuth(w http.ResponseWriter, r *http.Request) {
	// Generate random state and store in MongoDB (works across multiple instances)
	state := generateRandomState()
	h.db.OAuthStates().InsertOne(r.Context(), bson.M{
		"_id":       state,
		"expiresAt": time.Now().Add(10 * time.Minute),
		"createdAt": time.Now(),
	})

	url := h.googleOAuth.GetAuthURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// GoogleOAuthCallback handles Google OAuth callback
func (h *AuthHandler) GoogleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	// Validate state from MongoDB (atomically delete to prevent reuse)
	result := h.db.OAuthStates().FindOneAndDelete(r.Context(), bson.M{
		"_id":       state,
		"expiresAt": bson.M{"$gt": time.Now()},
	})
	if result.Err() != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid or expired OAuth state")
		return
	}

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

	// Require verified email from Google
	if !userInfo.VerifiedEmail {
		respondWithError(w, http.StatusBadRequest, "Google account email is not verified")
		return
	}

	// Check if user exists by Google ID
	var user models.User
	err = h.db.Users().FindOne(r.Context(), bson.M{"googleId": userInfo.ID}).Decode(&user)

	if err != nil {
		// Check if email exists (link accounts)
		err = h.db.Users().FindOne(r.Context(), bson.M{"email": strings.ToLower(userInfo.Email)}).Decode(&user)

		if err != nil {
			// Create new user with random display name
			now := time.Now()
			displayName, err := utils.GenerateUniqueDisplayName(r.Context(), h.db.Users())
			if err != nil {
				log.Printf("Failed to generate unique display name: %v", err)
				respondWithError(w, http.StatusInternalServerError, "Failed to create user")
				return
			}

			user = models.User{
				ID:                    primitive.NewObjectID(),
				Email:                 strings.ToLower(userInfo.Email),
				DisplayName:           displayName,
				GoogleID:              userInfo.ID,
				AuthMethods:           []models.AuthMethod{models.AuthMethodGoogle},
				EmailVerified:         true, // Google has verified the email
				EloRating:             models.DefaultEloRating,
				RankedGamesPlayed:     0,
				RankedWins:            0,
				RankedLosses:          0,
				RankedDraws:           0,
				TotalGamesPlayed:      0,
				IsActive:              true,
				CreatedAt:             now,
				UpdatedAt:             now,
				DisplayNameChanges:    0, // Initial generated name doesn't count
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

	audit.LogEvent(h.db, audit.EventOAuthLogin, &user.ID, user.Email, r, "google")

	// Redirect to frontend with tokens in URL fragment (fragments are not sent to server or logged)
	callbackURL := fmt.Sprintf("%s/auth/callback#access_token=%s&refresh_token=%s",
		h.frontendURL, accessToken, refreshToken)
	http.Redirect(w, r, callbackURL, http.StatusTemporaryRedirect)
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

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func generateRandomState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func generateRandomToken() string {
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

// VerifyEmail verifies a user's email address
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req VerifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Token == "" {
		respondWithError(w, http.StatusBadRequest, "Token is required")
		return
	}

	// Find the verification token
	var verification models.VerificationToken
	err := h.db.VerificationTokens().FindOne(r.Context(), bson.M{
		"token": req.Token,
		"type":  models.TokenTypeEmailVerification,
	}).Decode(&verification)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid or expired verification token")
		return
	}

	// Check if token is expired
	if time.Now().After(verification.ExpiresAt) {
		respondWithError(w, http.StatusBadRequest, "Verification token has expired")
		return
	}

	// Check if token was already used
	if verification.UsedAt != nil {
		respondWithError(w, http.StatusBadRequest, "Verification token has already been used")
		return
	}

	// Update user's email verified status
	now := time.Now()
	_, err = h.db.Users().UpdateOne(r.Context(), bson.M{"_id": verification.UserID}, bson.M{
		"$set": bson.M{
			"emailVerified": true,
			"updatedAt":     now,
		},
	})

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to verify email")
		return
	}

	// Mark token as used
	h.db.VerificationTokens().UpdateOne(r.Context(), bson.M{"_id": verification.ID}, bson.M{
		"$set": bson.M{"usedAt": now},
	})

	respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Email verified successfully",
	})
}

// ResendVerification resends the verification email
func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var req ResendVerificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Email == "" {
		respondWithError(w, http.StatusBadRequest, "Email is required")
		return
	}

	// Find user by email
	var user models.User
	err := h.db.Users().FindOne(r.Context(), bson.M{"email": req.Email}).Decode(&user)
	if err != nil {
		// Don't reveal if email exists
		respondWithJSON(w, http.StatusOK, map[string]string{
			"message": "If an account with that email exists, a verification email has been sent",
		})
		return
	}

	// Check if already verified
	if user.EmailVerified {
		respondWithJSON(w, http.StatusOK, map[string]string{
			"message": "If an account with that email exists, a verification email has been sent",
		})
		return
	}

	// Check per-user rate limit: 1 per 60 seconds
	if user.LastVerificationSent != nil {
		timeSinceLastSent := time.Since(*user.LastVerificationSent)
		if timeSinceLastSent < 60*time.Second {
			remainingSeconds := int((60*time.Second - timeSinceLastSent).Seconds())
			respondWithError(w, http.StatusTooManyRequests, fmt.Sprintf("Please wait %d seconds before requesting another verification email", remainingSeconds))
			return
		}
	}

	// Update last verification sent time
	now := time.Now()
	h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{"lastVerificationSent": now},
	})

	// Create new verification token
	verificationToken := generateRandomToken()
	verification := models.VerificationToken{
		ID:        primitive.NewObjectID(),
		UserID:    user.ID,
		Token:     verificationToken,
		Type:      models.TokenTypeEmailVerification,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}
	h.db.VerificationTokens().InsertOne(r.Context(), verification)

	// Send verification email
	go func() {
		if h.emailService != nil {
			log.Printf("Resending verification email to %s", user.Email)
			if err := h.emailService.SendVerificationEmail(user.Email, user.DisplayName, verificationToken); err != nil {
				log.Printf("Failed to resend verification email to %s: %v", user.Email, err)
			}
		} else {
			log.Printf("Email service not configured, skipping verification email to %s", user.Email)
		}
	}()

	respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Verification email sent",
	})
}

// ForgotPassword sends a password reset email
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Email == "" {
		respondWithError(w, http.StatusBadRequest, "Email is required")
		return
	}

	// Always return success to prevent email enumeration
	successResponse := map[string]string{
		"message": "If an account with that email exists, a password reset email has been sent",
	}

	// Find user by email
	var user models.User
	err := h.db.Users().FindOne(r.Context(), bson.M{"email": req.Email}).Decode(&user)
	if err != nil {
		respondWithJSON(w, http.StatusOK, successResponse)
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
		respondWithJSON(w, http.StatusOK, successResponse)
		return
	}

	// Create password reset token (expires in 1 hour)
	resetToken := generateRandomToken()
	verification := models.VerificationToken{
		ID:        primitive.NewObjectID(),
		UserID:    user.ID,
		Token:     resetToken,
		Type:      models.TokenTypePasswordReset,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		CreatedAt: time.Now(),
	}
	h.db.VerificationTokens().InsertOne(r.Context(), verification)

	// Send password reset email
	go func() {
		if h.emailService != nil {
			log.Printf("Sending password reset email to %s", user.Email)
			if err := h.emailService.SendPasswordResetEmail(user.Email, user.DisplayName, resetToken); err != nil {
				log.Printf("Failed to send password reset email to %s: %v", user.Email, err)
			}
		} else {
			log.Printf("Email service not configured, skipping password reset email to %s", user.Email)
		}
	}()

	respondWithJSON(w, http.StatusOK, successResponse)
}

// ResetPassword resets a user's password using a reset token
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Token == "" || req.NewPassword == "" {
		respondWithError(w, http.StatusBadRequest, "Token and new password are required")
		return
	}

	// Validate password strength
	if err := h.passwordService.ValidatePasswordStrength(req.NewPassword); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Atomically find and mark the token as used (prevents race conditions)
	now := time.Now()
	var verification models.VerificationToken
	err := h.db.VerificationTokens().FindOneAndUpdate(
		r.Context(),
		bson.M{
			"token":     req.Token,
			"type":      models.TokenTypePasswordReset,
			"usedAt":    nil,
			"expiresAt": bson.M{"$gt": now},
		},
		bson.M{"$set": bson.M{"usedAt": now}},
		options.FindOneAndUpdate().SetReturnDocument(options.Before),
	).Decode(&verification)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid or expired reset token")
		return
	}

	// Hash the new password
	passwordHash, err := h.passwordService.HashPassword(req.NewPassword)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to process password")
		return
	}

	// Update user's password and reset failed login state
	_, err = h.db.Users().UpdateOne(r.Context(), bson.M{"_id": verification.UserID}, bson.M{
		"$set": bson.M{
			"passwordHash":        passwordHash,
			"updatedAt":           now,
			"failedLoginAttempts": 0,
			"accountLockedUntil":  nil,
		},
	})

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update password")
		return
	}

	// Revoke all refresh tokens for this user (log them out everywhere)
	h.db.RefreshTokens().UpdateMany(r.Context(), bson.M{"userId": verification.UserID}, bson.M{
		"$set": bson.M{"isRevoked": true},
	})

	audit.LogEvent(h.db, audit.EventPasswordReset, &verification.UserID, "", r, "")

	respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Password has been reset successfully",
	})
}

// ChangePassword allows an authenticated user to change their password
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.NewPassword == "" {
		respondWithError(w, http.StatusBadRequest, "New password is required")
		return
	}

	// Check if user already has password auth method
	hasPassword := false
	authMethods := user.AuthMethods
	for _, method := range authMethods {
		if method == models.AuthMethodPassword {
			hasPassword = true
			break
		}
	}

	// If user already has a password, require current password for verification
	if hasPassword {
		if req.CurrentPassword == "" {
			respondWithError(w, http.StatusBadRequest, "Current password is required")
			return
		}
		if err := h.passwordService.ComparePassword(user.PasswordHash, req.CurrentPassword); err != nil {
			respondWithError(w, http.StatusUnauthorized, "Current password is incorrect")
			return
		}
	}

	// Validate password strength
	if err := h.passwordService.ValidatePasswordStrength(req.NewPassword); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Hash the new password
	passwordHash, err := h.passwordService.HashPassword(req.NewPassword)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to process password")
		return
	}

	// If user doesn't have password method, add it
	if !hasPassword {
		authMethods = append(authMethods, models.AuthMethodPassword)
	}

	// Update user's password
	now := time.Now()
	_, err = h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{
			"passwordHash": passwordHash,
			"authMethods":  authMethods,
			"updatedAt":    now,
		},
	})

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update password")
		return
	}

	audit.LogEvent(h.db, audit.EventPasswordChange, &user.ID, user.Email, r, "")

	respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Password has been changed successfully",
	})
}

// SuggestDisplayName generates a unique random display name
func (h *AuthHandler) SuggestDisplayName(w http.ResponseWriter, r *http.Request) {
	displayName, err := utils.GenerateUniqueDisplayName(r.Context(), h.db.Users())
	if err != nil {
		log.Printf("Failed to generate display name: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to generate display name")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"displayName": displayName,
	})
}

// CheckDisplayName checks if a display name is available
func (h *AuthHandler) CheckDisplayName(w http.ResponseWriter, r *http.Request) {
	var req CheckDisplayNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.DisplayName = strings.TrimSpace(req.DisplayName)

	if req.DisplayName == "" {
		respondWithError(w, http.StatusBadRequest, "Display name is required")
		return
	}

	// Validate display name format (3-20 chars, alphanumeric and underscores)
	if len(req.DisplayName) < 3 || len(req.DisplayName) > 20 {
		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"available": false,
			"reason":    "Display name must be between 3 and 20 characters",
		})
		return
	}

	// Check for valid characters
	for _, c := range req.DisplayName {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			respondWithJSON(w, http.StatusOK, map[string]interface{}{
				"available": false,
				"reason":    "Display name can only contain letters, numbers, and underscores",
			})
			return
		}
	}

	// Check if it exists
	var existing models.User
	err := h.db.Users().FindOne(r.Context(), bson.M{"displayName": req.DisplayName}).Decode(&existing)
	if err == nil {
		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"available": false,
			"reason":    "Display name is already taken",
		})
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"available": true,
	})
}

// ChangeDisplayName allows an authenticated user to change their display name
func (h *AuthHandler) ChangeDisplayName(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req ChangeDisplayNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.DisplayName = strings.TrimSpace(req.DisplayName)

	if req.DisplayName == "" {
		respondWithError(w, http.StatusBadRequest, "Display name is required")
		return
	}

	// Validate display name format (3-20 chars, alphanumeric and underscores)
	if len(req.DisplayName) < 3 || len(req.DisplayName) > 20 {
		respondWithError(w, http.StatusBadRequest, "Display name must be between 3 and 20 characters")
		return
	}

	// Check for valid characters
	for _, c := range req.DisplayName {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			respondWithError(w, http.StatusBadRequest, "Display name can only contain letters, numbers, and underscores")
			return
		}
	}

	// Check if it's the same as current
	if req.DisplayName == user.DisplayName {
		respondWithJSON(w, http.StatusOK, map[string]string{
			"message": "Display name unchanged",
		})
		return
	}

	// Check if display name is taken
	var existing models.User
	err := h.db.Users().FindOne(r.Context(), bson.M{"displayName": req.DisplayName}).Decode(&existing)
	if err == nil {
		respondWithError(w, http.StatusConflict, "Display name is already taken")
		return
	}

	// Check rate limit: can only change display name once per 24 hours (after the first change)
	// DisplayNameChanges == 0 means this is the initial/generated name, so first change is allowed
	if user.DisplayNameChanges > 0 && user.LastDisplayNameChange != nil {
		timeSinceLastChange := time.Since(*user.LastDisplayNameChange)
		if timeSinceLastChange < 24*time.Hour {
			remainingTime := 24*time.Hour - timeSinceLastChange
			hours := int(remainingTime.Hours())
			minutes := int(remainingTime.Minutes()) % 60
			respondWithError(w, http.StatusTooManyRequests, fmt.Sprintf("You can change your display name again in %dh %dm", hours, minutes))
			return
		}
	}

	// Update display name
	now := time.Now()
	_, err = h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{
			"displayName":           req.DisplayName,
			"lastDisplayNameChange": now,
			"updatedAt":             now,
		},
		"$inc": bson.M{
			"displayNameChanges": 1,
		},
	})

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update display name")
		return
	}

	// Return updated user info
	user.DisplayName = req.DisplayName
	user.LastDisplayNameChange = &now
	user.DisplayNameChanges++
	user.UpdatedAt = now

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Display name updated successfully",
		"user":    user,
	})
}

// UpdatePreferences updates user preferences
func (h *AuthHandler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req UpdatePreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Build update document
	updateFields := bson.M{
		"updatedAt": time.Now(),
	}

	// Initialize preferences if nil
	if user.Preferences == nil {
		user.Preferences = &models.UserPreferences{}
	}

	// Update autoDeclineDraws if provided
	if req.AutoDeclineDraws != nil {
		updateFields["preferences.autoDeclineDraws"] = *req.AutoDeclineDraws
		user.Preferences.AutoDeclineDraws = *req.AutoDeclineDraws
	}

	// Update preferredTimeControls if provided
	if req.PreferredTimeControls != nil {
		// Validate time controls
		validModes := map[string]bool{
			"unlimited":  true,
			"casual":     true,
			"standard":   true,
			"quick":      true,
			"blitz":      true,
			"tournament": true,
		}

		var validTimeControls []string
		for _, tc := range req.PreferredTimeControls {
			if validModes[tc] {
				validTimeControls = append(validTimeControls, tc)
			}
		}

		if len(validTimeControls) == 0 {
			respondWithError(w, http.StatusBadRequest, "At least one valid time control is required")
			return
		}

		updateFields["preferences.preferredTimeControls"] = validTimeControls
		// Convert to game.TimeControlMode for the response
		user.Preferences.PreferredTimeControls = nil
		for _, tc := range validTimeControls {
			user.Preferences.PreferredTimeControls = append(user.Preferences.PreferredTimeControls, game.TimeControlMode(tc))
		}
	}

	// Update user in database
	_, err := h.db.Users().UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": updateFields,
	})

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update preferences")
		return
	}

	user.UpdatedAt = time.Now()

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Preferences updated successfully",
		"user":    user,
	})
}
