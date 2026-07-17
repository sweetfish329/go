package main

import (
	"compress/gzip"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/CAFxX/httpcompression"
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

	// Validate admin credentials env vars at startup to avoid runtime DoS
	if os.Getenv("ADMIN_USERNAME") == "" || os.Getenv("ADMIN_PASSWORD") == "" {
		log.Fatalf("ADMIN_USERNAME and ADMIN_PASSWORD environment variables must be set")
	}

	if os.Getenv("ALLOWED_ORIGIN") == "" {
		log.Fatalf("ALLOWED_ORIGIN environment variable must be set (wildcard * is disabled for security)")
	}

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
	mux.HandleFunc("GET /u/{userId}/{kifuId}/sgf", kifuHandler.GetPublicKifuSgf)
	mux.HandleFunc("GET /share/{token}/sgf", kifuHandler.GetSharedSgf)
	mux.Handle("/", fs)

	// Wrap Mux with CORS, CSRF, request decompression and response compression middleware
	handlerWithCORS := enableCORS(mux)
	handlerWithCSRF := handler.CSRFMiddleware(handlerWithCORS)
	handlerWithDecompression := requestDecompression(handlerWithCSRF)

	compressor, err := httpcompression.DefaultAdapter()
	if err != nil {
		log.Fatalf("Failed to initialize httpcompression adapter: %v", err)
	}
	handlerWithCompression := compressor(handlerWithDecompression)

	log.Printf("Server listening on port %s", port)
	if err := http.ListenAndServe(":"+port, handlerWithCompression); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// requestDecompression Middleware handles decompressing request body if Content-Encoding is gzip
func requestDecompression(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Encoding") == "gzip" {
			gzipReader, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "Failed to initialize gzip reader: "+err.Error(), http.StatusBadRequest)
				return
			}
			r.Body = &gzipReadCloser{Reader: gzipReader, OriginalBody: r.Body}
		}
		next.ServeHTTP(w, r)
	})
}

type gzipReadCloser struct {
	*gzip.Reader
	OriginalBody io.ReadCloser
}

func (g *gzipReadCloser) Close() error {
	err1 := g.Reader.Close()
	err2 := g.OriginalBody.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

// CORS Middleware
func enableCORS(next http.Handler) http.Handler {
	allowedOrigin := os.Getenv("ALLOWED_ORIGIN")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowedOrigin != "" {
			if origin == allowedOrigin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
