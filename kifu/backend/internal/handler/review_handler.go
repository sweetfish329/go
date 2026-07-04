package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/sweetfish329/go/kifu/backend/internal/model"
	"github.com/sweetfish329/go/kifu/backend/internal/repository"
)

type ReviewHandler struct {
	repo *repository.ReviewRepository
}

func NewReviewHandler(repo *repository.ReviewRepository) *ReviewHandler {
	return &ReviewHandler{repo: repo}
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
	mux.HandleFunc("GET /api/kifus/{id}/reviews", h.ListForKifu)
	mux.HandleFunc("POST /api/kifus/{id}/reviews", h.Create)
	mux.HandleFunc("PUT /api/kifus/{id}/reviews/{review_id}", h.Update)
	mux.HandleFunc("DELETE /api/kifus/{id}/reviews/{review_id}", h.Delete)
}

func (h *ReviewHandler) ListForKifu(w http.ResponseWriter, r *http.Request) {
	kifuID := r.PathValue("id")
	if kifuID == "" {
		respondWithError(w, http.StatusBadRequest, "Missing Kifu ID")
		return
	}

	reviews, err := h.repo.FindByKifuID(kifuID)
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

func (h *ReviewHandler) Update(w http.ResponseWriter, r *http.Request) {
	reviewID := r.PathValue("review_id")
	if reviewID == "" {
		respondWithError(w, http.StatusBadRequest, "Missing Review ID")
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
	reviewID := r.PathValue("review_id")
	if reviewID == "" {
		respondWithError(w, http.StatusBadRequest, "Missing Review ID")
		return
	}

	err := h.repo.Delete(reviewID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"result": "success"})
}

// Helpers for validation
func parseParamInt(r *http.Request, key string, defaultValue int) int {
	valStr := r.PathValue(key)
	if valStr == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultValue
	}
	return val
}
