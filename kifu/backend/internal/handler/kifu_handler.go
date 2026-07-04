package handler

import (
	"encoding/json"
	"net/http"

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

func (h *KifuHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/kifus", h.List)
	mux.HandleFunc("GET /api/kifus/{id}", h.Get)
	mux.HandleFunc("POST /api/kifus", h.Upload)
	mux.HandleFunc("DELETE /api/kifus/{id}", h.Delete)
}

func (h *KifuHandler) List(w http.ResponseWriter, r *http.Request) {
	kifus, err := h.repo.FindAll()
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

	kifu, err := h.repo.FindByID(id)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if kifu == nil {
		respondWithError(w, http.StatusNotFound, "Kifu not found")
		return
	}

	respondWithJSON(w, http.StatusOK, kifu)
}

func (h *KifuHandler) Upload(w http.ResponseWriter, r *http.Request) {
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
	}

	if err := h.repo.Save(kifu); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, kifu)
}

func (h *KifuHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Missing ID")
		return
	}

	err := h.repo.Delete(id)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"result": "success"})
}

// Helpers for JSON response
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}
