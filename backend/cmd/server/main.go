package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"chess-game/internal/agent"
	"chess-game/internal/auth"
	"chess-game/internal/config"
	"chess-game/internal/db"
	"chess-game/internal/email"
	"chess-game/internal/eventbus"
	"chess-game/internal/handlers"
	"chess-game/internal/matchmaking"
	"chess-game/internal/middleware"
	"chess-game/internal/models"
	"chess-game/internal/services"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func main() {
	// Load configuration
	env := config.GetEnv()
	cfg, err := config.Load(env)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Starting chess server in %s mode", cfg.Environment)

	// Connect to MongoDB
	mongodb, err := db.NewMongoDB(cfg.MongoDB.URI, cfg.MongoDB.Database)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mongodb.Close(ctx)
	}()

	log.Printf("Connected to MongoDB database: %s", cfg.MongoDB.Database)

	// Initialize auth services
	jwtService := auth.NewJWTService(cfg.JWT.AccessSecret, cfg.JWT.RefreshSecret)
	passwordService := auth.NewPasswordService()
	googleOAuth := auth.NewGoogleOAuthService(
		cfg.OAuth.GoogleClientID,
		cfg.OAuth.GoogleClientSecret,
		cfg.OAuth.GoogleRedirectURL,
	)

	// Initialize email service
	var emailService *email.ResendService
	if cfg.Email.ResendAPIKey != "" {
		emailService = email.NewResendService(cfg.Email.ResendAPIKey, cfg.Frontend.URL)
		log.Println("Email service initialized")
	} else {
		log.Println("Warning: Email service not configured (RESEND_API_KEY not set)")
	}

	// Initialize matchmaking queue
	matchmakingQueue := matchmaking.NewQueue(mongodb)
	defer matchmakingQueue.Stop()

	// Create rate limiter
	rateLimiter := middleware.NewRateLimiter()
	defer rateLimiter.Stop()

	// Create middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtService, mongodb)

	// Create services
	gameCompletionService := services.NewGameCompletionService(mongodb)

	// Create handlers
	wsHandler := handlers.NewWebSocketHandler(mongodb, jwtService)
	gameHandler := handlers.NewGameHandler(mongodb, wsHandler, gameCompletionService, cfg.Game.MaxPositionHistory)

	// Start stale game cleanup worker (finds timed-out games missed by in-memory timers)
	cleanupService := services.NewStaleGameCleanupService(mongodb, gameCompletionService, wsHandler)
	cleanupService.Start()
	defer cleanupService.Stop()

	// Create cross-machine event bus using MongoDB Change Streams
	eb := eventbus.New(
		mongodb.WSEvents(),
		wsHandler.GetHub().BroadcastToSession,
		wsHandler.NotifyMatchLocalFunc(),
	)
	if err := eb.EnsureIndexes(context.Background()); err != nil {
		log.Printf("Warning: Failed to create ws_events indexes: %v", err)
	}
	eb.Start()
	defer eb.Stop()
	wsHandler.SetEventBus(eb)
	log.Printf("Cross-machine event bus initialized (machineId=%s)", eb.MachineID())

	// Wire matchmaking → WebSocket push notifications
	matchmakingQueue.SetMatchNotifier(wsHandler.NotifyMatchFound)
	matchmakingQueue.SetLobbyChangeNotifier(wsHandler.BroadcastLobbyUpdate)

	// Run immediate stale-game cleanup to catch games that timed out during downtime
	cleanupService.RunImmediateCleanup()

	// Initialize builtin AI agent
	serverAddr := fmt.Sprintf("http://localhost:%d", cfg.Server.Port)
	builtinAgent := initBuiltinAgent(mongodb, jwtService, matchmakingQueue, serverAddr, gameHandler)
	if builtinAgent != nil {
		defer builtinAgent.Stop()
	}

	matchmakingQueue.Start()
	authHandler := handlers.NewAuthHandler(mongodb, jwtService, passwordService, googleOAuth, emailService, cfg.Frontend.URL)
	matchmakingHandler := handlers.NewMatchmakingHandler(mongodb, matchmakingQueue)
	apiKeyHandler := handlers.NewApiKeyHandler(mongodb)
	leaderboardHandler := handlers.NewLeaderboardHandler(mongodb)

	// Set up router
	router := mux.NewRouter()

	// WebSocket routes (rate limited)
	router.HandleFunc("/ws/games/{sessionId}", rateLimiter.RateLimitHandler(
		middleware.WebSocketUpgradeLimit,
		func(r *http.Request) string { return "ws:" + middleware.GetClientIP(r) },
		wsHandler.HandleWebSocket,
	))
	router.HandleFunc("/ws/matchmaking/{connectionId}", rateLimiter.RateLimitHandler(
		middleware.WebSocketUpgradeLimit,
		func(r *http.Request) string { return "ws:" + middleware.GetClientIP(r) },
		wsHandler.HandleMatchmakingWebSocket,
	))
	router.HandleFunc("/ws/lobby", rateLimiter.RateLimitHandler(
		middleware.WebSocketUpgradeLimit,
		func(r *http.Request) string { return "ws:" + middleware.GetClientIP(r) },
		wsHandler.HandleLobbyWebSocket,
	))

	// API routes
	api := router.PathPrefix("/api").Subrouter()

	// Auth routes (public) with rate limiting
	// Account creation: 5 per hour per IP
	api.HandleFunc("/auth/register", rateLimiter.RateLimitHandler(
		middleware.AccountCreationLimit,
		func(r *http.Request) string { return "register:" + middleware.GetClientIP(r) },
		authHandler.Register,
	)).Methods("POST")

	// Login: 10 attempts per 15 minutes per IP
	api.HandleFunc("/auth/login", rateLimiter.RateLimitHandler(
		middleware.LoginAttemptLimit,
		func(r *http.Request) string { return "login:" + middleware.GetClientIP(r) },
		authHandler.Login,
	)).Methods("POST")

	api.HandleFunc("/auth/refresh", rateLimiter.RateLimitHandler(
		middleware.TokenRefreshLimit,
		func(r *http.Request) string { return "refresh:" + middleware.GetClientIP(r) },
		authHandler.Refresh,
	)).Methods("POST")

	// OAuth: 10 per minute per IP
	api.HandleFunc("/auth/google", rateLimiter.RateLimitHandler(
		middleware.OAuthInitLimit,
		func(r *http.Request) string { return "oauth:" + middleware.GetClientIP(r) },
		authHandler.GoogleOAuth,
	)).Methods("GET")
	api.HandleFunc("/auth/google/callback", authHandler.GoogleOAuthCallback).Methods("GET")

	// Email verification: 10 per hour per IP
	api.HandleFunc("/auth/verify-email", rateLimiter.RateLimitHandler(
		middleware.EmailVerificationLimit,
		func(r *http.Request) string { return "verify:" + middleware.GetClientIP(r) },
		authHandler.VerifyEmail,
	)).Methods("POST")

	// Resend verification: 1 per 60 seconds per email (handled in handler too for per-user tracking)
	api.HandleFunc("/auth/resend-verification", rateLimiter.RateLimitHandler(
		middleware.ResendVerificationLimit,
		func(r *http.Request) string { return "resend:" + middleware.GetClientIP(r) },
		authHandler.ResendVerification,
	)).Methods("POST")

	// Password reset: 5 per hour per IP
	api.HandleFunc("/auth/forgot-password", rateLimiter.RateLimitHandler(
		middleware.PasswordResetLimit,
		func(r *http.Request) string { return "forgot:" + middleware.GetClientIP(r) },
		authHandler.ForgotPassword,
	)).Methods("POST")
	api.HandleFunc("/auth/reset-password", authHandler.ResetPassword).Methods("POST")

	// Display name endpoints (public)
	api.HandleFunc("/auth/suggest-display-name", rateLimiter.RateLimitHandler(
		middleware.SuggestedNameLimit,
		func(r *http.Request) string { return "suggest:" + middleware.GetClientIP(r) },
		authHandler.SuggestDisplayName,
	)).Methods("GET")
	api.HandleFunc("/auth/check-display-name", rateLimiter.RateLimitHandler(
		middleware.DisplayNameCheckLimit,
		func(r *http.Request) string { return "check:" + middleware.GetClientIP(r) },
		authHandler.CheckDisplayName,
	)).Methods("POST")

	// Auth routes (protected)
	authApi := api.PathPrefix("/auth").Subrouter()
	authApi.Use(authMiddleware.RequireAuth)
	authApi.HandleFunc("/logout", authHandler.Logout).Methods("POST")
	authApi.HandleFunc("/me", authHandler.GetMe).Methods("GET")
	authApi.HandleFunc("/change-password", authHandler.ChangePassword).Methods("POST")
	authApi.HandleFunc("/change-display-name", authHandler.ChangeDisplayName).Methods("POST")
	authApi.HandleFunc("/preferences", authHandler.UpdatePreferences).Methods("PATCH")
	authApi.HandleFunc("/api-keys", apiKeyHandler.CreateApiKey).Methods("POST")
	authApi.HandleFunc("/api-keys", apiKeyHandler.ListApiKeys).Methods("GET")
	authApi.HandleFunc("/api-keys/{keyId}", apiKeyHandler.DeleteApiKey).Methods("DELETE")

	// Game routes (optional auth)
	gameApi := api.PathPrefix("/games").Subrouter()
	gameApi.Use(authMiddleware.OptionalAuth)
	gameApi.HandleFunc("", rateLimiter.RateLimitHandler(
		middleware.GameCreationLimit,
		func(r *http.Request) string { return "gamecreate:" + middleware.GetClientIP(r) },
		gameHandler.CreateGame,
	)).Methods("POST")
	// Static routes MUST come before /{sessionId} catch-all
	gameApi.HandleFunc("/active", gameHandler.ListActiveGames).Methods("GET")
	gameApi.HandleFunc("/completed", gameHandler.ListCompletedGames).Methods("GET")
	gameApi.HandleFunc("/{sessionId}", gameHandler.GetGame).Methods("GET")
	gameApi.HandleFunc("/{sessionId}/join", gameHandler.JoinGame).Methods("POST")
	gameApi.HandleFunc("/{sessionId}/move", gameHandler.MakeMove).Methods("POST")
	gameApi.HandleFunc("/{sessionId}/moves", gameHandler.GetMoves).Methods("GET")
	gameApi.HandleFunc("/{sessionId}/resign", gameHandler.ResignGame).Methods("POST")
	gameApi.HandleFunc("/{sessionId}/offer-draw", gameHandler.OfferDraw).Methods("POST")
	gameApi.HandleFunc("/{sessionId}/respond-draw", gameHandler.RespondToDraw).Methods("POST")
	gameApi.HandleFunc("/{sessionId}/claim-draw", gameHandler.ClaimDraw).Methods("POST")

	// User routes (optional auth — ranked games visible to all, unranked only to owner)
	userApi := api.PathPrefix("/users").Subrouter()
	userApi.Use(authMiddleware.OptionalAuth)
	userApi.HandleFunc("/lookup", gameHandler.LookupUserByDisplayName).Methods("GET")
	userApi.HandleFunc("/{userId}/games", gameHandler.GetUserGameHistory).Methods("GET")

	// Leaderboard route (public)
	api.HandleFunc("/leaderboard", leaderboardHandler.GetLeaderboard).Methods("GET")

	// Matchmaking routes (optional auth, but ranked requires auth in handler)
	matchApi := api.PathPrefix("/matchmaking").Subrouter()
	matchApi.Use(authMiddleware.OptionalAuth)
	matchApi.HandleFunc("/join", matchmakingHandler.JoinQueue).Methods("POST")
	matchApi.HandleFunc("/leave", matchmakingHandler.LeaveQueue).Methods("POST")
	matchApi.HandleFunc("/status", matchmakingHandler.GetQueueStatus).Methods("GET")
	matchApi.HandleFunc("/lobby", matchmakingHandler.GetLobby).Methods("GET")

	// API Documentation
	router.HandleFunc("/docs", handlers.ServeAPIDocs(cfg.Analytics.GoogleAnalyticsID)).Methods("GET")

	// Health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Serve static files in production
	if cfg.Environment == "prod" {
		// Prepare index.html with optional GA injection
		indexHTML, err := os.ReadFile("./public/index.html")
		if err != nil {
			log.Fatalf("Failed to read index.html: %v", err)
		}
		indexContent := string(indexHTML)
		if cfg.Analytics.GoogleAnalyticsID != "" {
			gaSnippet := fmt.Sprintf("<!-- Google tag (gtag.js) -->\n"+
				`    <script async src="https://www.googletagmanager.com/gtag/js?id=%s"></script>`+"\n"+
				"    <script>%s</script>\n"+
				"  </head>",
				cfg.Analytics.GoogleAnalyticsID,
				middleware.GAInlineScript(cfg.Analytics.GoogleAnalyticsID))
			indexContent = strings.Replace(indexContent, "</head>", gaSnippet, 1)
		}

		// Serve frontend static files
		spa := http.FileServer(http.Dir("./public"))
		router.PathPrefix("/").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Don't serve static files for API or WebSocket routes
			if len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/api/" || len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/ws/" {
				http.NotFound(w, r)
				return
			}
			// Serve index.html for all non-existent files (SPA routing)
			path := "./public" + r.URL.Path
			if _, err := os.Stat(path); os.IsNotExist(err) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write([]byte(indexContent))
				return
			}
			spa.ServeHTTP(w, r)
		}))
	}

	// CORS middleware
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{cfg.Frontend.URL},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Player-ID"},
		AllowCredentials: true,
	})

	// Create server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      middleware.SecurityHeaders(cfg.Analytics.GoogleAnalyticsID)(corsHandler.Handler(router)),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Server listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}

// initBuiltinAgent looks up (or creates) the Metavert system user, upserts
// the agent rating, and wires the builtin AI agent into matchmaking.
func initBuiltinAgent(mongodb *db.MongoDB, jwtService *auth.JWTService, queue *matchmaking.Queue, serverAddr string, gameHandler *handlers.GameHandler) *agent.BuiltinAgent {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const agentOwner = "Metavert"
	const agentName = "chessmata-2ply"

	// Find or create the Metavert system user
	var metavert models.User
	err := mongodb.Users().FindOne(ctx, bson.M{"displayName": agentOwner}).Decode(&metavert)
	if err != nil {
		// Auto-create the system user for the builtin agent
		log.Printf("Creating system user '%s' for builtin agent", agentOwner)
		now := time.Now()
		metavert = models.User{
			Email:         "system@chessmata.com",
			DisplayName:   agentOwner,
			AuthMethods:   []models.AuthMethod{},
			EmailVerified: true,
			EloRating:     models.DefaultEloRating,
			IsActive:      true,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		res, insertErr := mongodb.Users().InsertOne(ctx, metavert)
		if insertErr != nil {
			log.Printf("Warning: Failed to create system user '%s': %v", agentOwner, insertErr)
			return nil
		}
		metavert.ID = res.InsertedID.(primitive.ObjectID)
		log.Printf("Created system user '%s' (ID: %s)", agentOwner, metavert.ID.Hex())
	}

	// Upsert agent rating (create if not exists, don't overwrite existing rating)
	_, err = mongodb.AgentRatings().UpdateOne(ctx, bson.M{
		"ownerUserId": metavert.ID,
		"agentName":   agentName,
	}, bson.M{
		"$setOnInsert": bson.M{
			"ownerUserId":      metavert.ID,
			"agentName":        agentName,
			"eloRating":        models.DefaultEloRating,
			"rankedGamesPlayed": 0,
			"wins":             0,
			"losses":           0,
			"draws":            0,
			"createdAt":        time.Now(),
			"updatedAt":        time.Now(),
		},
	}, options.Update().SetUpsert(true))
	if err != nil {
		log.Printf("Warning: Failed to upsert agent rating: %v", err)
		return nil
	}

	// Fetch current agent Elo
	var agentRating models.AgentRating
	err = mongodb.AgentRatings().FindOne(ctx, bson.M{
		"ownerUserId": metavert.ID,
		"agentName":   agentName,
	}).Decode(&agentRating)
	if err != nil {
		log.Printf("Warning: Failed to read agent rating: %v", err)
		return nil
	}

	// Create the builtin agent
	builtinAgent := agent.NewBuiltinAgent(mongodb, jwtService, metavert.ID, agentName, serverAddr)

	// Wire into matchmaking: when the queue decides to inject the builtin agent,
	// it calls this provider which starts the agent's game loop.
	queue.SetBuiltinAgent(metavert.ID, agentName, agentRating.EloRating,
		func(sessionID string, color models.PlayerColor) {
			builtinAgent.StartGame(sessionID, color)
		},
	)

	log.Printf("Builtin agent '%s' initialized (owner: %s, Elo: %d)", agentName, agentOwner, agentRating.EloRating)

	// Wire agent turn notifications so MakeMove can wake the agent immediately
	gameHandler.SetAgentTurnNotifier(builtinAgent.NotifyTurn)

	// Resume any active games from before the server restarted
	builtinAgent.ResumeActiveGames()

	// Start periodic check for stuck agent games every 5 minutes
	builtinAgent.StartPeriodicCheck(5 * time.Minute)

	return builtinAgent
}
