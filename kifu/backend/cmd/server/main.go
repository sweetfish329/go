package main

import (
	"log"
	"net/http"
	"os"

	"github.com/sweetfish329/go/kifu/backend/internal/db"
	"github.com/sweetfish329/go/kifu/backend/internal/handler"
	"github.com/sweetfish329/go/kifu/backend/internal/repository"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("Starting server...")

	// Initialize database
	database, err := db.InitDB()
	if err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}
	defer database.Close()

	// Initialize repositories
	kifuRepo := repository.NewKifuRepository(database)
	reviewRepo := repository.NewReviewRepository(database)
	userRepo := repository.NewUserRepository(database)
	oauthRepo := repository.NewOAuthRepository(database)
	siteSettingRepo := repository.NewSiteSettingRepository(database)

	// Initialize handlers
	kifuHandler := handler.NewKifuHandler(kifuRepo, siteSettingRepo)
	reviewHandler := handler.NewReviewHandler(reviewRepo, kifuRepo)
	authHandler := handler.NewAuthHandler(userRepo, oauthRepo, siteSettingRepo)
	adminHandler := handler.NewAdminHandler(oauthRepo, siteSettingRepo)

	// Routing setup
	mux := http.NewServeMux()
	kifuHandler.RegisterRoutes(mux)
	reviewHandler.RegisterRoutes(mux)
	authHandler.RegisterRoutes(mux)
	adminHandler.RegisterRoutes(mux)

	// Serve static files and assets, but intercept GET "/" exactly to inject OGP tags
	fs := http.FileServer(http.Dir("./dist"))
	mux.HandleFunc("GET /{$}", kifuHandler.RootHandler)
	mux.HandleFunc("GET /u/{userId}", kifuHandler.RootHandler)
	mux.HandleFunc("GET /u/{userId}/{kifuId}", kifuHandler.RootHandler)
	mux.Handle("/", fs)

	// Wrap Mux with CORS middleware
	handlerWithCORS := enableCORS(mux)

	log.Printf("Server listening on port %s", port)
	if err := http.ListenAndServe(":"+port, handlerWithCORS); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// CORS Middleware
func enableCORS(next http.Handler) http.Handler {
	allowedOrigin := os.Getenv("ALLOWED_ORIGIN")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set headers
		origin := r.Header.Get("Origin")
		if allowedOrigin != "" {
			if origin == allowedOrigin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
