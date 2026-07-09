package handler

import (
	"context"
	"errors"
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
		secret = "kifu-secret-key-1234"
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

func GenerateAdminToken(username string) (string, error) {
	expirationTime := time.Now().Add(12 * time.Hour) // 12 hours
	claims := &Claims{
		UserID:   "admin",
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

// AuthMiddleware protects routes that require authentication
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tokenStr string
		cookie, err := r.Cookie("session_token")
		if err == nil && cookie.Value != "" {
			tokenStr = cookie.Value
		} else {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.Split(authHeader, " ")
				if len(parts) == 2 && parts[0] == "Bearer" {
					tokenStr = parts[1]
				}
			}
		}

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
		var tokenStr string
		cookie, err := r.Cookie("session_token")
		if err == nil && cookie.Value != "" {
			tokenStr = cookie.Value
		} else {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.Split(authHeader, " ")
				if len(parts) == 2 && parts[0] == "Bearer" {
					tokenStr = parts[1]
				}
			}
		}

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
		var tokenStr string
		cookie, err := r.Cookie("session_token")
		if err == nil && cookie.Value != "" {
			tokenStr = cookie.Value
		} else {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.Split(authHeader, " ")
				if len(parts) == 2 && parts[0] == "Bearer" {
					tokenStr = parts[1]
				}
			}
		}

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
