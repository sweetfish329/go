package handler

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/sweetfish329/go/kifu/backend/internal/model"
	"github.com/sweetfish329/go/kifu/backend/internal/repository"
)

func subtleConstantTimeCompare(a, b string) int {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b))
}

type AdminHandler struct {
	oauthRepo       *repository.OAuthRepository
	siteSettingRepo *repository.SiteSettingRepository
}

func NewAdminHandler(oauthRepo *repository.OAuthRepository, siteSettingRepo *repository.SiteSettingRepository) *AdminHandler {
	return &AdminHandler{
		oauthRepo:       oauthRepo,
		siteSettingRepo: siteSettingRepo,
	}
}

type AdminLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AdminLoginResponse struct {
	Token string `json:"token"`
}

func (h *AdminHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/admin/login", h.Login)
	mux.Handle("POST /api/admin/logout", AdminMiddleware(http.HandlerFunc(h.Logout)))

	// Public site settings read API
	mux.HandleFunc("GET /api/site-settings", h.GetSiteSettings)

	// Protected Admin routes
	mux.Handle("GET /api/admin/oauth-settings", AdminMiddleware(http.HandlerFunc(h.GetOAuthSettings)))
	mux.Handle("PUT /api/admin/oauth-settings", AdminMiddleware(http.HandlerFunc(h.SaveOAuthSettings)))
	mux.Handle("PUT /api/admin/site-settings", AdminMiddleware(http.HandlerFunc(h.SaveSiteSettings)))
}

func (h *AdminHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	respondWithJSON(w, http.StatusOK, map[string]string{"result": "success"})
}

func (h *AdminHandler) Login(w http.ResponseWriter, r *http.Request) {
	// Add simple delay to mitigate brute force
	time.Sleep(500 * time.Millisecond)

	var req AdminLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	adminUser := os.Getenv("ADMIN_USERNAME")
	adminPass := os.Getenv("ADMIN_PASSWORD")
	if adminUser == "" || adminPass == "" {
		respondWithError(w, http.StatusInternalServerError, "Admin credentials not configured")
		return
	}

	// Convert strings to byte slices for subtle comparison
	// Note: username check is also done in constant-time to avoid revealing username existence via timing
	usernameMatch := subtleConstantTimeCompare(req.Username, adminUser)
	passwordMatch := subtleConstantTimeCompare(req.Password, adminPass)

	if usernameMatch != 1 || passwordMatch != 1 {
		respondWithError(w, http.StatusUnauthorized, "Invalid admin credentials")
		return
	}

	token, err := GenerateAdminToken(adminUser)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate admin token")
		return
	}

	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" || os.Getenv("COOKIE_SECURE") == "true"
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	respondWithJSON(w, http.StatusOK, AdminLoginResponse{Token: token})
}

func (h *AdminHandler) GetOAuthSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.oauthRepo.FindAll()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, settings)
}

func (h *AdminHandler) SaveOAuthSettings(w http.ResponseWriter, r *http.Request) {
	var req model.OAuthSetting
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Provider == "" || req.ClientID == "" || req.ClientSecret == "" {
		respondWithError(w, http.StatusBadRequest, "All oauth setting fields are required")
		return
	}

	if req.RedirectURL == "" {
		req.RedirectURL = "http://dummy" // Fallback to satisfy DB NOT NULL constraint
	}

	if err := h.oauthRepo.Save(&req); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, req)
}

func (h *AdminHandler) GetSiteSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.siteSettingRepo.FindAll()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, settings)
}

func (h *AdminHandler) SaveSiteSettings(w http.ResponseWriter, r *http.Request) {
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	for k, v := range req {
		if k == "title" || k == "tab_name" || k == "favicon" || k == "theme_color" || k == "external_url" {
			if k == "theme_color" {
				isValidColor := false
				if len(v) > 0 && v[0] == '#' {
					isValidColor = true
					for _, c := range v[1:] {
						if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
							isValidColor = false
							break
						}
					}
					if len(v) != 4 && len(v) != 7 {
						isValidColor = false
					}
				}
				if !isValidColor {
					respondWithError(w, http.StatusBadRequest, "Invalid theme color format")
					return
				}
			}
			if err := h.siteSettingRepo.Save(k, v); err != nil {
				respondWithError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
	}

	respondWithJSON(w, http.StatusOK, req)
}
