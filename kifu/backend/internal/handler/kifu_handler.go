package handler

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"image/jpeg"
	"image/png"
	"io"
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
	mux.Handle("PUT /api/kifus/{id}/privacy", AuthMiddleware(http.HandlerFunc(h.UpdatePrivacy)))
	mux.Handle("PUT /api/kifus/{id}/ogp", AuthMiddleware(http.HandlerFunc(h.UpdateOgpImage)))
	mux.Handle("DELETE /api/kifus/{id}", AuthMiddleware(http.HandlerFunc(h.Delete)))

	// Public routes
	mux.HandleFunc("GET /api/share/{token}", h.GetShared)
	mux.HandleFunc("GET /api/share/{token}/og-image", h.GetSharedOgImage)
	mux.HandleFunc("GET /api/u/{userId}/kifus", h.ListPublic)
	mux.HandleFunc("GET /api/u/{userId}/kifus/{kifuId}", h.GetPublicKifu)
	mux.HandleFunc("GET /api/u/{userId}/kifus/{kifuId}/og-image", h.GetPublicKifuOgImage)
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

func PrepareKifuFromSgf(sgfData string, reqTitle string, userID string, isAdmin bool) (*model.Kifu, error) {
	rootNode, err := sgf.Parse(sgfData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SGF: %w", err)
	}

	meta := sgf.ExtractMetadata(rootNode)

	title := reqTitle
	if title == "" {
		title = meta.BlackPlayer + " vs " + meta.WhitePlayer
		if title == " vs " {
			title = "Untitled Game"
		}
	}

	var uploadedBy *string
	if !isAdmin {
		uploadedBy = &userID
	}

	return &model.Kifu{
		Title:       title,
		BlackPlayer: meta.BlackPlayer,
		BlackRank:   meta.BlackRank,
		WhitePlayer: meta.WhitePlayer,
		WhiteRank:   meta.WhiteRank,
		GameDate:    meta.Date,
		Result:      meta.Result,
		Komi:        meta.Komi,
		Handicap:    meta.Handicap,
		SgfData:     sgfData,
		UploadedBy:  uploadedBy,
		IsPrivate:   true,
	}, nil
}

func (h *KifuHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Enforce maximum request body size (2MB) for SGF uploads to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, 2*1024*1024)

	var req UploadKifuRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload or body too large")
		return
	}

	if req.SgfData == "" {
		respondWithError(w, http.StatusBadRequest, "sgf_data is required")
		return
	}

	isAdmin, _ := r.Context().Value(IsAdminKey).(bool)

	kifu, err := PrepareKifuFromSgf(req.SgfData, req.Title, userID, isAdmin)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid SGF format: "+err.Error())
		return
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

type UpdatePrivacyRequest struct {
	IsPrivate bool `json:"is_private"`
}

func (h *KifuHandler) UpdatePrivacy(w http.ResponseWriter, r *http.Request) {
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

	if kifu.UploadedBy == nil || *kifu.UploadedBy != userID {
		respondWithError(w, http.StatusForbidden, "Forbidden")
		return
	}

	var req UpdatePrivacyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := h.repo.UpdatePrivacy(id, req.IsPrivate); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{"result": "success", "is_private": req.IsPrivate})
}

func (h *KifuHandler) ListPublic(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	if userID == "" {
		respondWithError(w, http.StatusBadRequest, "Missing UserID")
		return
	}

	kifus, err := h.repo.FindAllPublicByUser(userID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, kifus)
}

func (h *KifuHandler) GetPublicKifu(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	kifuID := r.PathValue("kifuId")
	if userID == "" || kifuID == "" {
		respondWithError(w, http.StatusBadRequest, "Missing UserID or KifuID")
		return
	}

	kifu, err := h.repo.FindByIDAndUser(kifuID, userID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if kifu == nil {
		respondWithError(w, http.StatusNotFound, "Kifu not found")
		return
	}

	if kifu.IsPrivate {
		respondWithError(w, http.StatusForbidden, "Forbidden")
		return
	}

	respondWithJSON(w, http.StatusOK, kifu)
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

	// Try getting from DB first
	imgData, err := h.repo.GetOgpImageByShareToken(token)
	if err == nil && len(imgData) > 0 {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 1 day
		_, _ = w.Write(imgData)
		return
	}

	// Fallback to auto generation
	h.serveGeneratedOgImage(w, kifu)
}

// UpdateOgpImage receives OGP image from client and saves it in database
func (h *KifuHandler) UpdateOgpImage(w http.ResponseWriter, r *http.Request) {
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

	// Read image binary data (limit to 5MB)
	limitReader := io.LimitReader(r.Body, 5*1024*1024)
	imgData, err := io.ReadAll(limitReader)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to read request body: "+err.Error())
		return
	}

	if len(imgData) == 0 {
		respondWithError(w, http.StatusBadRequest, "Empty image data")
		return
	}

	contentType := http.DetectContentType(imgData)
	if contentType == "image/png" {
		// Verify that it can be decoded as a valid PNG
		if _, err := png.Decode(bytes.NewReader(imgData)); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid PNG image data: "+err.Error())
			return
		}
	} else if contentType == "image/jpeg" {
		// Verify that it can be decoded as a valid JPEG
		if _, err := jpeg.Decode(bytes.NewReader(imgData)); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid JPEG image data: "+err.Error())
			return
		}
	} else {
		respondWithError(w, http.StatusBadRequest, "Invalid image format. Only PNG and JPEG are allowed.")
		return
	}

	err = h.repo.UpdateOgpImage(id, imgData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to save OGP image: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// GetPublicKifuOgImage retrieves and returns OGP PNG image for public kifu
func (h *KifuHandler) GetPublicKifuOgImage(w http.ResponseWriter, r *http.Request) {
	userId := r.PathValue("userId")
	kifuId := r.PathValue("kifuId")

	kifu, err := h.repo.FindByIDAndUser(kifuId, userId)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Kifu not found")
		return
	}
	if kifu == nil || kifu.IsPrivate {
		respondWithError(w, http.StatusForbidden, "Forbidden")
		return
	}

	// Try getting from DB first
	imgData, err := h.repo.GetOgpImage(kifuId)
	if err == nil && len(imgData) > 0 {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 1 day
		_, _ = w.Write(imgData)
		return
	}

	// Fallback to auto generation
	h.serveGeneratedOgImage(w, kifu)
}

func (h *KifuHandler) serveGeneratedOgImage(w http.ResponseWriter, kifu *model.Kifu) {
	rootNode, err := sgf.Parse(kifu.SgfData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to parse SGF: "+err.Error())
		return
	}

	meta := sgf.ExtractMetadata(rootNode)
	lastNode := rootNode.GetEnd()
	board := lastNode.Board()

	img := sgf.GenerateBoardImage(board, rootNode, meta.Size)

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 1 day
	if err := png.Encode(w, img); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to encode PNG: "+err.Error())
	}
}

func resolveSchemeAndHost(r *http.Request) (string, string) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	return scheme, r.Host
}

func BuildKifuOgpMeta(kifu *model.Kifu, userId string, kifuId string, scheme string, host string, escapedTabName string) string {
	title := html.EscapeString(kifu.Title)
	escapedBlack := html.EscapeString(kifu.BlackPlayer)
	escapedWhite := html.EscapeString(kifu.WhitePlayer)
	escapedResult := html.EscapeString(kifu.Result)
	description := fmt.Sprintf("対局者: 黒 %s vs 白 %s", escapedBlack, escapedWhite)
	if kifu.Result != "" {
		description += fmt.Sprintf(" (結果: %s)", escapedResult)
	}

	robotsTag := ""
	if kifu.IsPrivate {
		robotsTag = `<meta name="robots" content="noindex, nofollow" />`
	} else {
		robotsTag = `<meta name="robots" content="index, follow" />`
	}

	ogImageUrl := ""
	if kifu.ShareToken != nil && *kifu.ShareToken != "" {
		ogImageUrl = fmt.Sprintf("%s://%s/api/share/%s/og-image?t=%d", scheme, html.EscapeString(host), *kifu.ShareToken, kifu.UpdatedAt.Unix())
	} else if !kifu.IsPrivate {
		ogImageUrl = fmt.Sprintf("%s://%s/api/u/%s/kifus/%s/og-image?t=%d", scheme, html.EscapeString(host), userId, kifuId, kifu.UpdatedAt.Unix())
	}

	ogpMeta := fmt.Sprintf(`
	%s
	<meta property="og:title" content="%s | %s" />
	<meta property="og:description" content="%s" />
	<meta property="og:type" content="website" />
	<meta name="twitter:card" content="summary_large_image" />
	<meta name="twitter:title" content="%s | %s" />
	<meta name="twitter:description" content="%s" />`,
		robotsTag, title, escapedTabName, description,
		title, escapedTabName, description,
	)

	if ogImageUrl != "" {
		ogpMeta += fmt.Sprintf(`
		<meta property="og:image" content="%s" />
		<meta name="twitter:image" content="%s" />`, ogImageUrl, ogImageUrl)
	}
	return ogpMeta
}

func BuildSharedKifuOgpMeta(kifu *model.Kifu, shareToken string, scheme string, host string, escapedTabName string) string {
	title := html.EscapeString(kifu.Title)
	escapedBlack := html.EscapeString(kifu.BlackPlayer)
	escapedWhite := html.EscapeString(kifu.WhitePlayer)
	escapedResult := html.EscapeString(kifu.Result)
	description := fmt.Sprintf("対局者: 黒 %s vs 白 %s", escapedBlack, escapedWhite)
	if kifu.Result != "" {
		description += fmt.Sprintf(" (結果: %s)", escapedResult)
	}

	ogImageUrl := fmt.Sprintf("%s://%s/api/share/%s/og-image?t=%d", scheme, html.EscapeString(host), shareToken, kifu.UpdatedAt.Unix())

	return fmt.Sprintf(`
	<meta property="og:title" content="%s | %s" />
	<meta property="og:description" content="%s" />
	<meta property="og:image" content="%s" />
	<meta property="og:type" content="website" />
	<meta name="twitter:card" content="summary_large_image" />
	<meta name="twitter:title" content="%s | %s" />
	<meta name="twitter:description" content="%s" />
	<meta name="twitter:image" content="%s" />`,
		title, escapedTabName, description, ogImageUrl,
		title, escapedTabName, description, ogImageUrl,
	)
}

func BuildUserListOgpMeta(escapedTabName string) string {
	return fmt.Sprintf(`
	<meta name="robots" content="index, follow" />
	<meta property="og:title" content="公開棋譜一覧 | %s" />
	<meta property="og:description" content="ユーザーの一般公開棋譜一覧です。" />
	<meta property="og:type" content="website" />`, escapedTabName)
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

	htmlContent := string(htmlBytes)

	// Replace title tag
	tabName := settings["tab_name"]
	if tabName == "" {
		tabName = "kifu_store"
	}
	escapedTabName := html.EscapeString(tabName)
	htmlContent = strings.Replace(htmlContent, "<title>kifu_store</title>", "<title>"+escapedTabName+"</title>", 1)

	// Replace favicon if configured
	favicon := settings["favicon"]
	if favicon != "" {
		// Validate that the favicon starts with http://, https://, or a safe relative path (e.g. /) to prevent javascript: XSS
		isSafeFavicon := strings.HasPrefix(favicon, "http://") || strings.HasPrefix(favicon, "https://") || strings.HasPrefix(favicon, "/")
		if !isSafeFavicon {
			favicon = "/kifu-favicon.ico"
		}
		escapedFavicon := html.EscapeString(favicon)
		htmlContent = strings.Replace(htmlContent, `<link rel="icon" type="image/svg+xml" href="/vite.svg" />`, `<link rel="icon" href="`+escapedFavicon+`" />`, 1)
	}

	// Inject CSS variable for theme color
	themeColor := settings["theme_color"]
	// Strictly validate themeColor hex format to prevent XSS
	isValidColor := false
	if len(themeColor) > 0 && themeColor[0] == '#' {
		isValidColor = true
		for _, c := range themeColor[1:] {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				isValidColor = false
				break
			}
		}
		if len(themeColor) != 4 && len(themeColor) != 7 {
			isValidColor = false
		}
	}
	if !isValidColor {
		themeColor = "#4e342e"
	}
	themeStyle := fmt.Sprintf("\n\t<style>:root { --theme-color: %s; }</style>", themeColor)
	htmlContent = strings.Replace(htmlContent, "</head>", themeStyle+"\n</head>", 1)

	userId := r.PathValue("userId")
	kifuId := r.PathValue("kifuId")

	var ogpMeta string

	if kifuId != "" && userId != "" {
		kifu, err := h.repo.FindByIDAndUser(kifuId, userId)
		if err == nil && kifu != nil {
			scheme, host := resolveSchemeAndHost(r)
			ogpMeta = BuildKifuOgpMeta(kifu, userId, kifuId, scheme, host, escapedTabName)
		}
	} else if userId != "" {
		ogpMeta = BuildUserListOgpMeta(escapedTabName)
	} else {
		// Fallback to share token if any
		shareToken := r.URL.Query().Get("share")
		if shareToken != "" {
			kifu, err := h.repo.FindByShareToken(shareToken)
			if err == nil && kifu != nil && (kifu.ShareExpiresAt == nil || kifu.ShareExpiresAt.After(time.Now())) {
				scheme, host := resolveSchemeAndHost(r)
				ogpMeta = BuildSharedKifuOgpMeta(kifu, shareToken, scheme, host, escapedTabName)
			}
		}
	}

	if ogpMeta != "" {
		htmlContent = strings.Replace(htmlContent, "</head>", ogpMeta+"\n</head>", 1)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(htmlContent))
}
