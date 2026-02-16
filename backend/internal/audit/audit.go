package audit

import (
	"context"
	"log"
	"net/http"
	"time"

	"chess-game/internal/db"
	"chess-game/internal/middleware"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Event types for audit logging
const (
	EventLoginSuccess  = "login_success"
	EventLoginFailed   = "login_failed"
	EventRegister      = "register"
	EventPasswordChange = "password_change"
	EventPasswordReset = "password_reset"
	EventLogout        = "logout"
	EventAccountLocked = "account_locked"
	EventOAuthLogin    = "oauth_login"
)

// AuditEvent represents a security-relevant event.
type AuditEvent struct {
	ID        primitive.ObjectID  `bson:"_id,omitempty"`
	EventType string              `bson:"eventType"`
	UserID    *primitive.ObjectID `bson:"userId,omitempty"`
	Email     string              `bson:"email,omitempty"`
	IP        string              `bson:"ip"`
	UserAgent string              `bson:"userAgent"`
	Details   string              `bson:"details,omitempty"`
	CreatedAt time.Time           `bson:"createdAt"`
}

// LogEvent writes an audit event to the database (fire-and-forget).
func LogEvent(database *db.MongoDB, eventType string, userID *primitive.ObjectID, email string, r *http.Request, details string) {
	event := AuditEvent{
		EventType: eventType,
		UserID:    userID,
		Email:     email,
		IP:        middleware.GetClientIP(r),
		UserAgent: r.UserAgent(),
		Details:   details,
		CreatedAt: time.Now(),
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := database.AuditLog().InsertOne(ctx, bson.M{
			"eventType": event.EventType,
			"userId":    event.UserID,
			"email":     event.Email,
			"ip":        event.IP,
			"userAgent": event.UserAgent,
			"details":   event.Details,
			"createdAt": event.CreatedAt,
		}); err != nil {
			log.Printf("Audit log write failed: %v", err)
		}
	}()
}
