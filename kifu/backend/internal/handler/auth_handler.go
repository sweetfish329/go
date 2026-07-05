package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sweetfish329/go/kifu/backend/internal/model"
	"github.com/sweetfish329/go/kifu/backend/internal/repository"
)

type AuthHandler struct {
	repo      *repository.UserRepository
	oauthRepo *repository.OAuthRepository
}

func NewAuthHandler(repo *repository.UserRepository, oauthRepo *repository.OAuthRepository) *AuthHandler {
	return &AuthHandler{repo: repo, oauthRepo: oauthRepo}
}

type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type OAuthLoginRequest struct {
	Provider        string `json:"provider"`
	ProviderUserID  string `json:"provider_user_id"`
	DefaultUsername string `json:"default_username"`
}

type UpdateUsernameRequest struct {
	Username string `json:"username"`
}

type AuthResponse struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/register", h.Register)
	mux.HandleFunc("POST /api/auth/login", h.Login)
	mux.HandleFunc("POST /api/auth/oauth", h.OAuthLogin)
	mux.HandleFunc("GET /api/auth/providers", h.GetEnabledProviders)

	// Protected
	mux.Handle("GET /api/auth/me", AuthMiddleware(http.HandlerFunc(h.Me)))
	mux.Handle("PUT /api/auth/username", AuthMiddleware(http.HandlerFunc(h.UpdateUsername)))
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	respondWithError(w, http.StatusForbidden, "Password registration is disabled. Please use OAuth.")
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	respondWithError(w, http.StatusForbidden, "Password login is disabled. Please use OAuth.")
}

func (h *AuthHandler) OAuthLogin(w http.ResponseWriter, r *http.Request) {
	var req OAuthLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Provider == "" || req.ProviderUserID == "" {
		respondWithError(w, http.StatusBadRequest, "provider and provider_user_id are required")
		return
	}

	// 1. Check if user already registered this oauth
	user, err := h.repo.FindByOAuth(req.Provider, req.ProviderUserID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if user == nil {
		// 2. Register new user for this oauth identity
		baseUsername := req.DefaultUsername
		if baseUsername == "" {
			baseUsername = req.Provider + "_user"
		}

		uniqueName := h.getUniqueUsername(baseUsername)

		user = &model.User{
			Username: uniqueName,
		}
		oauth := &model.UserOAuth{
			Provider:       req.Provider,
			ProviderUserID: req.ProviderUserID,
		}

		if err := h.repo.CreateWithOAuth(user, oauth); err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// 3. Generate token
	token, err := GenerateToken(user.ID, user.Username)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	respondWithJSON(w, http.StatusOK, AuthResponse{
		Token: token,
		User:  user,
	})
}

func (h *AuthHandler) UpdateUsername(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req UpdateUsernameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Username == "" {
		respondWithError(w, http.StatusBadRequest, "Username is required")
		return
	}

	// Check name duplication
	existing, err := h.repo.FindByUsername(req.Username)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing != nil && existing.ID != userID {
		respondWithError(w, http.StatusConflict, "Username is already taken")
		return
	}

	if err := h.repo.UpdateUsername(userID, req.Username); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updatedUser, err := h.repo.FindByID(userID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch updated user info")
		return
	}

	respondWithJSON(w, http.StatusOK, updatedUser)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, err := h.repo.FindByID(userID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if user == nil {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	respondWithJSON(w, http.StatusOK, user)
}

func (h *AuthHandler) getUniqueUsername(base string) string {
	username := base
	for {
		existing, err := h.repo.FindByUsername(username)
		if err == nil && existing == nil {
			return username
		}
		// Append random hex suffix
		bytes := make([]byte, 2)
		_, _ = rand.Read(bytes)
		username = fmt.Sprintf("%s_%s", base, hex.EncodeToString(bytes))
	}
}

func (h *AuthHandler) GetEnabledProviders(w http.ResponseWriter, r *http.Request) {
	settings, err := h.oauthRepo.FindAll()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	statuses := map[string]bool{
		"google": false,
		"line":   false,
		"meta":   false,
	}

	for _, s := range settings {
		statuses[s.Provider] = s.Enabled
	}

	respondWithJSON(w, http.StatusOK, statuses)
}
