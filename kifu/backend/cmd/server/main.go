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

	// Initialize handlers
	kifuHandler := handler.NewKifuHandler(kifuRepo)
	reviewHandler := handler.NewReviewHandler(reviewRepo)

	// Routing setup
	mux := http.NewServeMux()
	kifuHandler.RegisterRoutes(mux)
	reviewHandler.RegisterRoutes(mux)

	// Simple root route
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Kifu Store API is running"))
	})

	// Wrap Mux with CORS middleware
	handlerWithCORS := enableCORS(mux)

	log.Printf("Server listening on port %s", port)
	if err := http.ListenAndServe(":"+port, handlerWithCORS); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// CORS Middleware
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
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
