package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserIDKey contextKey = "userID"
const UsernameKey contextKey = "username"

var jwtSecret []byte

func init() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatalf("JWT_SECRET environment variable is not set")
	}
	jwtSecret = []byte(secret)
}

type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin,omitempty"`
	jwt.RegisteredClaims
}

func GenerateToken(userID string, username string) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour) // 1 day
	claims := &Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func GenerateAdminToken(userID string, username string) (string, error) {
	expirationTime := time.Now().Add(12 * time.Hour) // 12 hours
	claims := &Claims{
		UserID:   userID,
		Username: username,
		IsAdmin:  true,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func ValidateToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

func extractToken(r *http.Request) string {
	cookie, err := r.Cookie("session_token")
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}
	return ""
}

// AuthMiddleware protects routes that require authentication
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := extractToken(r)
		if tokenStr == "" {
			respondWithError(w, http.StatusUnauthorized, "Authorization required")
			return
		}

		claims, err := ValidateToken(tokenStr)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UsernameKey, claims.Username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuthMiddleware parses the token if present, but does not reject requests if missing
func OptionalAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := extractToken(r)
		if tokenStr == "" {
			next.ServeHTTP(w, r)
			return
		}

		claims, err := ValidateToken(tokenStr)
		if err == nil {
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, UsernameKey, claims.Username)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// AdminMiddleware protects routes that require admin privileges
func AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := extractToken(r)
		if tokenStr == "" {
			respondWithError(w, http.StatusUnauthorized, "Authorization required")
			return
		}

		claims, err := ValidateToken(tokenStr)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		if !claims.IsAdmin {
			respondWithError(w, http.StatusForbidden, "Admin privileges required")
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UsernameKey, claims.Username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CSRFMiddleware handles CSRF protection using Double-Submit Cookie pattern for stateful API endpoints
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" || os.Getenv("COOKIE_SECURE") == "true"

		// Skip CSRF check for safe methods
		if r.Method == "GET" || r.Method == "OPTIONS" || r.Method == "HEAD" {
			// Generate or reuse CSRF token and set in cookie if not present or empty
			cookie, err := r.Cookie("csrf_token")
			var csrfToken string
			if err != nil || cookie.Value == "" {
				b := make([]byte, 16)
				if _, err := rand.Read(b); err == nil {
					csrfToken = hex.EncodeToString(b)
					http.SetCookie(w, &http.Cookie{
						Name:     "csrf_token",
						Value:    csrfToken,
						Path:     "/",
						HttpOnly: false, // Must be readable by frontend JavaScript
						Secure:   secure,
						SameSite: http.SameSiteLaxMode,
					})
				}
			}
			next.ServeHTTP(w, r)
			return
		}

		// Verify CSRF token for mutative requests
		cookie, err := r.Cookie("csrf_token")
		if err != nil || cookie.Value == "" {
			respondWithError(w, http.StatusForbidden, "CSRF token missing")
			return
		}

		headerToken := r.Header.Get("X-CSRF-Token")
		if headerToken == "" || headerToken != cookie.Value {
			respondWithError(w, http.StatusForbidden, "Invalid CSRF token")
			return
		}

		next.ServeHTTP(w, r)
	})
}
