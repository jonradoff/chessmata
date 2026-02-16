package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"chess-game/internal/db"
	"chess-game/internal/middleware"
	"chess-game/internal/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	maxApiKeysPerUser = 10
	maxApiKeyNameLen  = 50
	apiKeyPrefix      = "cmk_"
)

type ApiKeyHandler struct {
	db *db.MongoDB
}

func NewApiKeyHandler(database *db.MongoDB) *ApiKeyHandler {
	return &ApiKeyHandler{db: database}
}

type CreateApiKeyRequest struct {
	Name string `json:"name"`
}

type CreateApiKeyResponse struct {
	ApiKey models.ApiKey `json:"apiKey"`
	Key    string        `json:"key"` // Full key, shown only once
}

// CreateApiKey creates a new API key for the authenticated user
func (h *ApiKeyHandler) CreateApiKey(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req CreateApiKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		respondWithError(w, http.StatusBadRequest, "Name is required")
		return
	}
	if len(req.Name) > maxApiKeyNameLen {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Name must be %d characters or less", maxApiKeyNameLen))
		return
	}

	// Check key count limit
	count, err := h.db.ApiKeys().CountDocuments(r.Context(), bson.M{"userId": user.ID})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to check key count")
		return
	}
	if count >= maxApiKeysPerUser {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Maximum of %d API keys allowed", maxApiKeysPerUser))
		return
	}

	// Generate key
	rawToken := generateRandomToken()
	fullKey := apiKeyPrefix + rawToken
	keyHash := hashToken(fullKey)
	keyPrefix := fullKey[:12]

	now := time.Now()
	apiKey := models.ApiKey{
		ID:        primitive.NewObjectID(),
		UserID:    user.ID,
		Name:      req.Name,
		KeyPrefix: keyPrefix,
		KeyHash:   keyHash,
		CreatedAt: now,
	}

	_, err = h.db.ApiKeys().InsertOne(r.Context(), apiKey)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create API key")
		return
	}

	respondWithJSON(w, http.StatusCreated, CreateApiKeyResponse{
		ApiKey: apiKey,
		Key:    fullKey,
	})
}

// ListApiKeys returns all API keys for the authenticated user
func (h *ApiKeyHandler) ListApiKeys(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	cursor, err := h.db.ApiKeys().Find(r.Context(), bson.M{"userId": user.ID})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to list API keys")
		return
	}
	defer cursor.Close(r.Context())

	var keys []models.ApiKey
	if err := cursor.All(r.Context(), &keys); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to read API keys")
		return
	}

	if keys == nil {
		keys = []models.ApiKey{}
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"apiKeys": keys,
	})
}

// DeleteApiKey deletes an API key owned by the authenticated user
func (h *ApiKeyHandler) DeleteApiKey(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	vars := mux.Vars(r)
	keyID, err := primitive.ObjectIDFromHex(vars["keyId"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid key ID")
		return
	}

	result, err := h.db.ApiKeys().DeleteOne(r.Context(), bson.M{
		"_id":    keyID,
		"userId": user.ID,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to delete API key")
		return
	}

	if result.DeletedCount == 0 {
		respondWithError(w, http.StatusNotFound, "API key not found")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "API key deleted",
	})
}

// authenticateApiKey looks up a cmk_ prefixed key and returns the user, or nil
func authenticateApiKey(ctx context.Context, database *db.MongoDB, token string) *models.User {
	if !strings.HasPrefix(token, apiKeyPrefix) {
		return nil
	}

	keyHash := hashToken(token)

	var apiKey models.ApiKey
	err := database.ApiKeys().FindOne(ctx, bson.M{"keyHash": keyHash}).Decode(&apiKey)
	if err != nil {
		return nil
	}

	var user models.User
	err = database.Users().FindOne(ctx, bson.M{"_id": apiKey.UserID}).Decode(&user)
	if err != nil || !user.IsActive {
		return nil
	}

	// Update lastUsedAt in background
	go func() {
		now := time.Now()
		database.ApiKeys().UpdateOne(context.Background(), bson.M{"_id": apiKey.ID}, bson.M{
			"$set": bson.M{"lastUsedAt": now},
		})
	}()

	return &user
}
