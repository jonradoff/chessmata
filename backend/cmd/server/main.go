package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"chess-game/internal/auth"
	"chess-game/internal/config"
	"chess-game/internal/db"
	"chess-game/internal/handlers"
	"chess-game/internal/matchmaking"
	"chess-game/internal/middleware"

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

	// Initialize matchmaking queue
	matchmakingQueue := matchmaking.NewQueue(mongodb)
	matchmakingQueue.Start()
	defer matchmakingQueue.Stop()

	// Create middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtService, mongodb)

	// Create handlers
	wsHandler := handlers.NewWebSocketHandler(mongodb)
	gameHandler := handlers.NewGameHandler(mongodb, wsHandler)
	authHandler := handlers.NewAuthHandler(mongodb, jwtService, passwordService, googleOAuth)
	matchmakingHandler := handlers.NewMatchmakingHandler(mongodb, matchmakingQueue)

	// Set up router
	router := mux.NewRouter()

	// WebSocket routes
	router.HandleFunc("/ws/games/{sessionId}", wsHandler.HandleWebSocket)

	// API routes
	api := router.PathPrefix("/api").Subrouter()

	// Auth routes (public)
	api.HandleFunc("/auth/register", authHandler.Register).Methods("POST")
	api.HandleFunc("/auth/login", authHandler.Login).Methods("POST")
	api.HandleFunc("/auth/refresh", authHandler.Refresh).Methods("POST")
	api.HandleFunc("/auth/google", authHandler.GoogleOAuth).Methods("GET")
	api.HandleFunc("/auth/google/callback", authHandler.GoogleOAuthCallback).Methods("GET")

	// Auth routes (protected)
	authApi := api.PathPrefix("/auth").Subrouter()
	authApi.Use(authMiddleware.RequireAuth)
	authApi.HandleFunc("/logout", authHandler.Logout).Methods("POST")
	authApi.HandleFunc("/me", authHandler.GetMe).Methods("GET")

	// Game routes (optional auth)
	gameApi := api.PathPrefix("/games").Subrouter()
	gameApi.Use(authMiddleware.OptionalAuth)
	gameApi.HandleFunc("", gameHandler.CreateGame).Methods("POST")
	gameApi.HandleFunc("/{sessionId}", gameHandler.GetGame).Methods("GET")
	gameApi.HandleFunc("/{sessionId}/join", gameHandler.JoinGame).Methods("POST")
	gameApi.HandleFunc("/{sessionId}/move", gameHandler.MakeMove).Methods("POST")
	gameApi.HandleFunc("/{sessionId}/moves", gameHandler.GetMoves).Methods("GET")
	gameApi.HandleFunc("/{sessionId}/resign", gameHandler.ResignGame).Methods("POST")

	// Matchmaking routes (optional auth, but ranked requires auth in handler)
	matchApi := api.PathPrefix("/matchmaking").Subrouter()
	matchApi.Use(authMiddleware.OptionalAuth)
	matchApi.HandleFunc("/join", matchmakingHandler.JoinQueue).Methods("POST")
	matchApi.HandleFunc("/leave", matchmakingHandler.LeaveQueue).Methods("POST")
	matchApi.HandleFunc("/status", matchmakingHandler.GetQueueStatus).Methods("GET")

	// API Documentation
	router.HandleFunc("/docs", handlers.ServeAPIDocs).Methods("GET")

	// Health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

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
		Handler:      corsHandler.Handler(router),
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
