package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/sweetfish329/go/kifu/backend/internal/model"
	"github.com/sweetfish329/go/kifu/backend/internal/repository"
)

type ReviewHandler struct {
	repo     *repository.ReviewRepository
	kifuRepo *repository.KifuRepository
}

func NewReviewHandler(repo *repository.ReviewRepository, kifuRepo *repository.KifuRepository) *ReviewHandler {
	return &ReviewHandler{
		repo:     repo,
		kifuRepo: kifuRepo,
	}
}

type CreateReviewRequest struct {
	MoveNumber   int    `json:"move_number"`
	NodePath     string `json:"node_path"`
	ReviewerName string `json:"reviewer_name"`
	Comment      string `json:"comment"`
	Variations   string `json:"variations"`
}

type UpdateReviewRequest struct {
	ReviewerName string `json:"reviewer_name"`
	Comment      string `json:"comment"`
	Variations   string `json:"variations"`
}

func (h *ReviewHandler) RegisterRoutes(mux *http.ServeMux) {
	// Private routes
	mux.Handle("GET /api/kifus/{id}/reviews", AuthMiddleware(http.HandlerFunc(h.ListForKifu)))
	mux.Handle("POST /api/kifus/{id}/reviews", AuthMiddleware(http.HandlerFunc(h.Create)))
	mux.Handle("PUT /api/kifus/{id}/reviews/{review_id}", AuthMiddleware(http.HandlerFunc(h.Update)))
	mux.Handle("DELETE /api/kifus/{id}/reviews/{review_id}", AuthMiddleware(http.HandlerFunc(h.Delete)))

	// Public share routes
	mux.HandleFunc("GET /api/share/{token}/reviews", h.ListForShare)
	mux.Handle("POST /api/share/{token}/reviews", OptionalAuthMiddleware(http.HandlerFunc(h.CreateForShare)))
}

func (h *ReviewHandler) ListForKifu(w http.ResponseWriter, r *http.Request) {
	kifuID := r.PathValue("id")
	if kifuID == "" {
		respondWithError(w, http.StatusBadRequest, "Missing Kifu ID")
		return
	}

	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Verify kifu ownership
	kifu, err := h.kifuRepo.FindByID(kifuID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if kifu == nil {
		respondWithError(w, http.StatusNotFound, "Kifu not found")
		return
	}
	if kifu.UploadedBy == nil || *kifu.UploadedBy != userID {
		respondWithError(w, http.StatusForbidden, "Forbidden")
		return
	}

	reviews, err := h.repo.FindByKifuID(kifuID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, reviews)
}

func (h *ReviewHandler) ListForShare(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		respondWithError(w, http.StatusBadRequest, "Missing token")
		return
	}

	// Validate share token
	kifu, err := h.kifuRepo.FindByShareToken(token)
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

	reviews, err := h.repo.FindByKifuID(kifu.ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, reviews)
}

func (h *ReviewHandler) Create(w http.ResponseWriter, r *http.Request) {
	kifuID := r.PathValue("id")
	if kifuID == "" {
		respondWithError(w, http.StatusBadRequest, "Missing Kifu ID")
		return
	}

	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Verify kifu ownership
	kifu, err := h.kifuRepo.FindByID(kifuID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if kifu == nil {
		respondWithError(w, http.StatusNotFound, "Kifu not found")
		return
	}
	if kifu.UploadedBy == nil || *kifu.UploadedBy != userID {
		respondWithError(w, http.StatusForbidden, "Forbidden")
		return
	}

	var req CreateReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.ReviewerName == "" {
		respondWithError(w, http.StatusBadRequest, "reviewer_name is required")
		return
	}
	if req.Comment == "" {
		respondWithError(w, http.StatusBadRequest, "comment is required")
		return
	}

	review := &model.Review{
		KifuID:       kifuID,
		MoveNumber:   req.MoveNumber,
		NodePath:     req.NodePath,
		ReviewerName: req.ReviewerName,
		Comment:      req.Comment,
		Variations:   req.Variations,
	}

	if err := h.repo.Save(review); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, review)
}

func (h *ReviewHandler) CreateForShare(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		respondWithError(w, http.StatusBadRequest, "Missing token")
		return
	}

	// Validate share token
	kifu, err := h.kifuRepo.FindByShareToken(token)
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

	var req CreateReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Auto-fill reviewer name if logged in
	reviewerName := req.ReviewerName
	if loggedInUsername, exists := r.Context().Value(UsernameKey).(string); exists {
		reviewerName = loggedInUsername
	}

	if reviewerName == "" {
		respondWithError(w, http.StatusBadRequest, "reviewer_name is required")
		return
	}
	if req.Comment == "" {
		respondWithError(w, http.StatusBadRequest, "comment is required")
		return
	}

	review := &model.Review{
		KifuID:       kifu.ID,
		MoveNumber:   req.MoveNumber,
		NodePath:     req.NodePath,
		ReviewerName: reviewerName,
		Comment:      req.Comment,
		Variations:   req.Variations,
	}

	if err := h.repo.Save(review); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, review)
}

func (h *ReviewHandler) Update(w http.ResponseWriter, r *http.Request) {
	kifuID := r.PathValue("id")
	reviewID := r.PathValue("review_id")
	if kifuID == "" || reviewID == "" {
		respondWithError(w, http.StatusBadRequest, "Missing IDs")
		return
	}

	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Verify kifu ownership
	kifu, err := h.kifuRepo.FindByID(kifuID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if kifu == nil {
		respondWithError(w, http.StatusNotFound, "Kifu not found")
		return
	}
	if kifu.UploadedBy == nil || *kifu.UploadedBy != userID {
		respondWithError(w, http.StatusForbidden, "Forbidden")
		return
	}

	var req UpdateReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.ReviewerName == "" {
		respondWithError(w, http.StatusBadRequest, "reviewer_name is required")
		return
	}
	if req.Comment == "" {
		respondWithError(w, http.StatusBadRequest, "comment is required")
		return
	}

	review := &model.Review{
		ID:           reviewID,
		ReviewerName: req.ReviewerName,
		Comment:      req.Comment,
		Variations:   req.Variations,
	}

	if err := h.repo.Update(review); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, review)
}

func (h *ReviewHandler) Delete(w http.ResponseWriter, r *http.Request) {
	kifuID := r.PathValue("id")
	reviewID := r.PathValue("review_id")
	if kifuID == "" || reviewID == "" {
		respondWithError(w, http.StatusBadRequest, "Missing IDs")
		return
	}

	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Verify kifu ownership
	kifu, err := h.kifuRepo.FindByID(kifuID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if kifu == nil {
		respondWithError(w, http.StatusNotFound, "Kifu not found")
		return
	}
	if kifu.UploadedBy == nil || *kifu.UploadedBy != userID {
		respondWithError(w, http.StatusForbidden, "Forbidden")
		return
	}

	err = h.repo.Delete(reviewID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"result": "success"})
}
