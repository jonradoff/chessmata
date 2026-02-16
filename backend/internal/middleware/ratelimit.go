package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	mu       sync.RWMutex
	requests map[string]*rateLimitEntry
	cleanup  *time.Ticker
	done     chan bool
}

type rateLimitEntry struct {
	count     int
	windowEnd time.Time
}

// RateLimitConfig defines rate limit parameters
type RateLimitConfig struct {
	MaxRequests int           // Maximum requests allowed in the window
	Window      time.Duration // Time window for rate limiting
}

// Common rate limit configurations
var (
	// Account creation: 5 accounts per hour per IP
	AccountCreationLimit = RateLimitConfig{MaxRequests: 5, Window: time.Hour}

	// Login attempts: 10 attempts per 15 minutes per IP
	LoginAttemptLimit = RateLimitConfig{MaxRequests: 10, Window: 15 * time.Minute}

	// Password reset requests: 5 per hour per IP
	PasswordResetLimit = RateLimitConfig{MaxRequests: 5, Window: time.Hour}

	// Resend verification: 3 per 60 seconds per email
	ResendVerificationLimit = RateLimitConfig{MaxRequests: 3, Window: 60 * time.Second}

	// Display name availability check: 30 per minute per IP
	DisplayNameCheckLimit = RateLimitConfig{MaxRequests: 30, Window: time.Minute}

	// Suggested display name: 20 per minute per IP
	SuggestedNameLimit = RateLimitConfig{MaxRequests: 20, Window: time.Minute}

	// Google OAuth initiation: 10 per minute per IP
	OAuthInitLimit = RateLimitConfig{MaxRequests: 10, Window: time.Minute}

	// Email verification: 10 per hour per IP
	EmailVerificationLimit = RateLimitConfig{MaxRequests: 10, Window: time.Hour}

	// Game creation: 10 per minute per IP
	GameCreationLimit = RateLimitConfig{MaxRequests: 10, Window: time.Minute}

	// Token refresh: 30 per minute per IP
	TokenRefreshLimit = RateLimitConfig{MaxRequests: 30, Window: time.Minute}

	// WebSocket upgrade: 20 per minute per IP
	WebSocketUpgradeLimit = RateLimitConfig{MaxRequests: 20, Window: time.Minute}
)

// NewRateLimiter creates a new rate limiter with automatic cleanup
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*rateLimitEntry),
		cleanup:  time.NewTicker(5 * time.Minute),
		done:     make(chan bool),
	}

	// Start cleanup goroutine
	go func() {
		for {
			select {
			case <-rl.cleanup.C:
				rl.cleanupExpired()
			case <-rl.done:
				return
			}
		}
	}()

	return rl
}

// Stop stops the rate limiter cleanup goroutine
func (rl *RateLimiter) Stop() {
	rl.cleanup.Stop()
	close(rl.done)
}

// cleanupExpired removes expired entries
func (rl *RateLimiter) cleanupExpired() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, entry := range rl.requests {
		if now.After(entry.windowEnd) {
			delete(rl.requests, key)
		}
	}
}

// Allow checks if a request should be allowed based on the rate limit
// Returns (allowed, remaining, resetTime)
func (rl *RateLimiter) Allow(key string, config RateLimitConfig) (bool, int, time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.requests[key]

	if !exists || now.After(entry.windowEnd) {
		// New window
		rl.requests[key] = &rateLimitEntry{
			count:     1,
			windowEnd: now.Add(config.Window),
		}
		return true, config.MaxRequests - 1, now.Add(config.Window)
	}

	// Existing window
	if entry.count >= config.MaxRequests {
		return false, 0, entry.windowEnd
	}

	entry.count++
	return true, config.MaxRequests - entry.count, entry.windowEnd
}

// GetClientIP extracts the real client IP from the request
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (used by proxies like Fly.io)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first IP in the chain
		if ip, _, err := net.SplitHostPort(xff); err == nil {
			return ip
		}
		// Try without port
		if net.ParseIP(xff) != nil {
			return xff
		}
		// May have multiple IPs, take the first
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				firstIP := xff[:i]
				if net.ParseIP(firstIP) != nil {
					return firstIP
				}
				break
			}
		}
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		if net.ParseIP(xri) != nil {
			return xri
		}
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// RateLimitMiddleware creates a middleware that applies rate limiting
func (rl *RateLimiter) RateLimitMiddleware(config RateLimitConfig, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			allowed, remaining, resetTime := rl.Allow(key, config)

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", string(rune(config.MaxRequests)))
			w.Header().Set("X-RateLimit-Remaining", string(rune(remaining)))
			w.Header().Set("X-RateLimit-Reset", resetTime.Format(time.RFC3339))

			if !allowed {
				retryAfter := int(time.Until(resetTime).Seconds())
				if retryAfter < 1 {
					retryAfter = 1
				}
				w.Header().Set("Retry-After", string(rune(retryAfter)))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":      "Rate limit exceeded",
					"retryAfter": retryAfter,
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// IPRateLimitMiddleware creates a middleware that rate limits by IP
func (rl *RateLimiter) IPRateLimitMiddleware(config RateLimitConfig) func(http.Handler) http.Handler {
	return rl.RateLimitMiddleware(config, func(r *http.Request) string {
		return GetClientIP(r)
	})
}

// RateLimitHandler wraps a handler function with rate limiting
func (rl *RateLimiter) RateLimitHandler(config RateLimitConfig, keyFunc func(*http.Request) string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := keyFunc(r)
		allowed, remaining, resetTime := rl.Allow(key, config)

		// Set rate limit headers
		w.Header().Set("X-RateLimit-Limit", intToStr(config.MaxRequests))
		w.Header().Set("X-RateLimit-Remaining", intToStr(remaining))
		w.Header().Set("X-RateLimit-Reset", resetTime.Format(time.RFC3339))

		if !allowed {
			retryAfter := int(time.Until(resetTime).Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			w.Header().Set("Retry-After", intToStr(retryAfter))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":      "Rate limit exceeded",
				"retryAfter": retryAfter,
			})
			return
		}

		handler(w, r)
	}
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + intToStr(-n)
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
