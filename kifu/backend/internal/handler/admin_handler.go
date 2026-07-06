package handler

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/sweetfish329/go/kifu/backend/internal/model"
	"github.com/sweetfish329/go/kifu/backend/internal/repository"
)

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

	// Public site settings read API
	mux.HandleFunc("GET /api/site-settings", h.GetSiteSettings)

	// Protected Admin routes
	mux.Handle("GET /api/admin/oauth-settings", AdminMiddleware(http.HandlerFunc(h.GetOAuthSettings)))
	mux.Handle("PUT /api/admin/oauth-settings", AdminMiddleware(http.HandlerFunc(h.SaveOAuthSettings)))
	mux.Handle("PUT /api/admin/site-settings", AdminMiddleware(http.HandlerFunc(h.SaveSiteSettings)))
}

func (h *AdminHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req AdminLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	adminUser := os.Getenv("ADMIN_USERNAME")
	if adminUser == "" {
		adminUser = "Admin"
	}
	adminPass := os.Getenv("ADMIN_PASSWORD")
	if adminPass == "" {
		adminPass = "admin"
	}

	if req.Username != adminUser || req.Password != adminPass {
		respondWithError(w, http.StatusUnauthorized, "Invalid admin credentials")
		return
	}

	token, err := GenerateAdminToken(adminUser)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate admin token")
		return
	}

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

	if req.Provider == "" || req.ClientID == "" || req.ClientSecret == "" || req.RedirectURL == "" {
		respondWithError(w, http.StatusBadRequest, "All oauth setting fields are required")
		return
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
		if k == "title" || k == "tab_name" || k == "favicon" || k == "theme_color" {
			if err := h.siteSettingRepo.Save(k, v); err != nil {
				respondWithError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
	}

	respondWithJSON(w, http.StatusOK, req)
}
