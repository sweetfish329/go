package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/sweetfish329/go/kifu/backend/internal/model"
	"github.com/sweetfish329/go/kifu/backend/internal/repository"
	"github.com/sweetfish329/go/kifu/backend/internal/sgf"
)

type KifuHandler struct {
	repo *repository.KifuRepository
}

func NewKifuHandler(repo *repository.KifuRepository) *KifuHandler {
	return &KifuHandler{repo: repo}
}

type UploadKifuRequest struct {
	Title   string `json:"title"`
	SgfData string `json:"sgf_data"`
}

type ShareKifuRequest struct {
	ExpiresInDays *int `json:"expires_in_days"` // nil or <= 0 means never expires
	Disable       bool `json:"disable"`         // true to disable sharing
}

func (h *KifuHandler) RegisterRoutes(mux *http.ServeMux) {
	// Protected routes
	mux.Handle("GET /api/kifus", AuthMiddleware(http.HandlerFunc(h.List)))
	mux.Handle("GET /api/kifus/{id}", AuthMiddleware(http.HandlerFunc(h.Get)))
	mux.Handle("POST /api/kifus", AuthMiddleware(http.HandlerFunc(h.Upload)))
	mux.Handle("POST /api/kifus/{id}/share", AuthMiddleware(http.HandlerFunc(h.Share)))
	mux.Handle("DELETE /api/kifus/{id}", AuthMiddleware(http.HandlerFunc(h.Delete)))

	// Public routes
	mux.HandleFunc("GET /api/share/{token}", h.GetShared)
}

func (h *KifuHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	kifus, err := h.repo.FindAllByUser(userID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, kifus)
}

func (h *KifuHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Missing ID")
		return
	}

	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	kifu, err := h.repo.FindByID(id)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if kifu == nil {
		respondWithError(w, http.StatusNotFound, "Kifu not found")
		return
	}

	// Verify ownership
	if kifu.UploadedBy == nil || *kifu.UploadedBy != userID {
		respondWithError(w, http.StatusForbidden, "Forbidden")
		return
	}

	respondWithJSON(w, http.StatusOK, kifu)
}

func (h *KifuHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req UploadKifuRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.SgfData == "" {
		respondWithError(w, http.StatusBadRequest, "sgf_data is required")
		return
	}

	// Parse SGF to validate and extract metadata
	rootNode, err := sgf.Parse(req.SgfData)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid SGF format: "+err.Error())
		return
	}

	meta := rootNode.ExtractMetadata()

	// If title is not provided, use a default title
	title := req.Title
	if title == "" {
		title = meta.BlackPlayer + " vs " + meta.WhitePlayer
		if title == " vs " {
			title = "Untitled Game"
		}
	}

	kifu := &model.Kifu{
		Title:       title,
		BlackPlayer: meta.BlackPlayer,
		BlackRank:   meta.BlackRank,
		WhitePlayer: meta.WhitePlayer,
		WhiteRank:   meta.WhiteRank,
		GameDate:    meta.Date,
		Result:      meta.Result,
		Komi:        meta.Komi,
		Handicap:    meta.Handicap,
		SgfData:     req.SgfData,
		UploadedBy:  &userID,
	}

	if err := h.repo.Save(kifu); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, kifu)
}

func (h *KifuHandler) Share(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Missing ID")
		return
	}

	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	kifu, err := h.repo.FindByID(id)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if kifu == nil {
		respondWithError(w, http.StatusNotFound, "Kifu not found")
		return
	}

	// Verify ownership
	if kifu.UploadedBy == nil || *kifu.UploadedBy != userID {
		respondWithError(w, http.StatusForbidden, "Forbidden")
		return
	}

	var req ShareKifuRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	var token *string
	var expiresAt *interface{} // Using interface{} for sql query params (nil or time.Time)

	if !req.Disable {
		t, err := generateRandomToken()
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to generate share token")
			return
		}
		token = &t

		if req.ExpiresInDays != nil && *req.ExpiresInDays > 0 {
			exp := time.Now().Add(time.Duration(*req.ExpiresInDays) * 24 * time.Hour)
			var expVal interface{} = exp
			expiresAt = &expVal
		} else {
			expiresAt = nil // Infinite
		}
	} else {
		token = nil
		expiresAt = nil
	}

	if err := h.repo.UpdateShare(id, token, expiresAt); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Retrieve updated kifu to return to client
	updatedKifu, err := h.repo.FindByID(id)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch updated kifu info")
		return
	}

	respondWithJSON(w, http.StatusOK, updatedKifu)
}

func (h *KifuHandler) GetShared(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		respondWithError(w, http.StatusBadRequest, "Missing token")
		return
	}

	kifu, err := h.repo.FindByShareToken(token)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if kifu == nil {
		respondWithError(w, http.StatusNotFound, "Shared kifu not found")
		return
	}

	// Check expiration
	if kifu.ShareExpiresAt != nil && kifu.ShareExpiresAt.Before(time.Now()) {
		respondWithError(w, http.StatusGone, "Shared link has expired")
		return
	}

	respondWithJSON(w, http.StatusOK, kifu)
}

func (h *KifuHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Missing ID")
		return
	}

	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	kifu, err := h.repo.FindByID(id)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if kifu == nil {
		respondWithError(w, http.StatusNotFound, "Kifu not found")
		return
	}

	// Verify ownership
	if kifu.UploadedBy == nil || *kifu.UploadedBy != userID {
		respondWithError(w, http.StatusForbidden, "Forbidden")
		return
	}

	err = h.repo.Delete(id)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"result": "success"})
}

func generateRandomToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Helpers for JSON response
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(response)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}
