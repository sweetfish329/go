package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/png"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sweetfish329/go/kifu/backend/internal/model"
	"github.com/sweetfish329/go/kifu/backend/internal/repository"
	"github.com/sweetfish329/go/kifu/backend/internal/sgf"
)

type KifuHandler struct {
	repo            *repository.KifuRepository
	siteSettingRepo *repository.SiteSettingRepository
}

func NewKifuHandler(repo *repository.KifuRepository, siteSettingRepo *repository.SiteSettingRepository) *KifuHandler {
	return &KifuHandler{
		repo:            repo,
		siteSettingRepo: siteSettingRepo,
	}
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
	mux.HandleFunc("GET /api/share/{token}/og-image", h.GetSharedOgImage)
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

// GetSharedOgImage generates and returns a PNG image representing the final board state of a shared kifu
func (h *KifuHandler) GetSharedOgImage(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		respondWithError(w, http.StatusBadRequest, "Missing token")
		return
	}

	kifu, err := h.repo.FindByShareToken(token)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Shared kifu not found")
		return
	}

	if kifu.ShareExpiresAt != nil && kifu.ShareExpiresAt.Before(time.Now()) {
		respondWithError(w, http.StatusGone, "Shared link has expired")
		return
	}

	// Parse SGF
	rootNode, err := sgf.Parse(kifu.SgfData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to parse SGF: "+err.Error())
		return
	}

	// Simulating SGF moves to reproduce board grid state
	meta := rootNode.ExtractMetadata()
	board := sgf.NewBoard(meta.Size)
	board.ReplicateGame(rootNode)

	// Rendering board grid state to image
	img := sgf.GenerateBoardImage(board.Grid, board.Size)

	w.Header().Set("Content-Type", "image/png")
	// Cache control for OGP images
	w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 1 day
	if err := png.Encode(w, img); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to encode PNG: "+err.Error())
	}
}

// RootHandler handles serving the index.html and dynamically injecting site settings and OGP tags
func (h *KifuHandler) RootHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve site settings
	settings, err := h.siteSettingRepo.FindAll()
	if err != nil {
		settings = map[string]string{
			"title":       "kifu_store",
			"tab_name":    "kifu_store",
			"favicon":     "",
			"theme_color": "#4e342e",
		}
	}

	htmlBytes, err := os.ReadFile("./dist/index.html")
	if err != nil {
		http.ServeFile(w, r, "./dist/index.html")
		return
	}

	html := string(htmlBytes)

	// Replace title tag
	tabName := settings["tab_name"]
	if tabName == "" {
		tabName = "kifu_store"
	}
	html = strings.Replace(html, "<title>kifu_store</title>", "<title>"+tabName+"</title>", 1)

	// Replace favicon if configured
	favicon := settings["favicon"]
	if favicon != "" {
		html = strings.Replace(html, `<link rel="icon" type="image/svg+xml" href="/vite.svg" />`, `<link rel="icon" href="`+favicon+`" />`, 1)
	}

	// Inject CSS variable for theme color
	themeColor := settings["theme_color"]
	if themeColor == "" {
		themeColor = "#4e342e"
	}
	themeStyle := fmt.Sprintf("\n\t<style>:root { --theme-color: %s; }</style>", themeColor)
	html = strings.Replace(html, "</head>", themeStyle+"\n</head>", 1)

	shareToken := r.URL.Query().Get("share")
	if shareToken != "" {
		kifu, err := h.repo.FindByShareToken(shareToken)
		if err == nil && (kifu.ShareExpiresAt == nil || kifu.ShareExpiresAt.Before(time.Now())) {
			// Dynamic OGP properties
			title := kifu.Title
			description := fmt.Sprintf("対局者: 黒 %s vs 白 %s", kifu.BlackPlayer, kifu.WhitePlayer)
			if kifu.Result != "" {
				description += fmt.Sprintf(" (結果: %s)", kifu.Result)
			}

			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
				scheme = proto
			}
			host := r.Host
			ogImageUrl := fmt.Sprintf("%s://%s/api/share/%s/og-image", scheme, host, shareToken)

			ogpMeta := fmt.Sprintf(`
			<meta property="og:title" content="%s | %s" />
			<meta property="og:description" content="%s" />
			<meta property="og:image" content="%s" />
			<meta property="og:type" content="website" />
			<meta name="twitter:card" content="summary_large_image" />
			<meta name="twitter:title" content="%s | %s" />
			<meta name="twitter:description" content="%s" />
			<meta name="twitter:image" content="%s" />`,
				title, tabName, description, ogImageUrl,
				title, tabName, description, ogImageUrl,
			)

			// Replace </head> with OGP tags inserted right before it
			html = strings.Replace(html, "</head>", ogpMeta+"\n</head>", 1)
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}
