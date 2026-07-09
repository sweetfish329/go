package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/sweetfish329/go/kifu/backend/internal/model"
	"github.com/sweetfish329/go/kifu/backend/internal/repository"
	"golang.org/x/oauth2"
)

type AuthHandler struct {
	repo            *repository.UserRepository
	oauthRepo       *repository.OAuthRepository
	siteSettingRepo *repository.SiteSettingRepository
}

func NewAuthHandler(repo *repository.UserRepository, oauthRepo *repository.OAuthRepository, siteSettingRepo *repository.SiteSettingRepository) *AuthHandler {
	return &AuthHandler{repo: repo, oauthRepo: oauthRepo, siteSettingRepo: siteSettingRepo}
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
	mux.HandleFunc("POST /api/auth/logout", h.Logout)
	mux.HandleFunc("POST /api/auth/oauth", h.OAuthLogin)
	mux.HandleFunc("GET /api/auth/providers", h.GetEnabledProviders)
	mux.HandleFunc("GET /api/users/{userId}/username", h.GetUsername)

	// Real OAuth2 flow endpoints
	mux.HandleFunc("GET /api/auth/oauth/redirect/{provider}", h.OAuth2Redirect)
	mux.HandleFunc("GET /api/auth/oauth/callback/{provider}", h.OAuth2Callback)

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

func (h *AuthHandler) OAuth2Redirect(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	if provider == "" {
		respondWithError(w, http.StatusBadRequest, "Missing provider")
		return
	}

	setting, err := h.oauthRepo.FindByProvider(provider)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if setting == nil || !setting.Enabled {
		respondWithError(w, http.StatusBadRequest, "Provider not configured or disabled")
		return
	}

	config := h.getOAuthConfig(provider, setting, r)
	if config == nil {
		respondWithError(w, http.StatusBadRequest, "Unsupported provider")
		return
	}

	stateBytes := make([]byte, 16)
	_, _ = rand.Read(stateBytes)
	state := hex.EncodeToString(stateBytes)

	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" || os.Getenv("COOKIE_SECURE") == "true"
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state_" + provider,
		Value:    state,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   secure,
	})

	// PKCE
	verifier, err := generateVerifier()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate verifier")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_verifier_" + provider,
		Value:    verifier,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   secure,
	})

	challenge := generateChallenge(verifier)
	url := config.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	http.Redirect(w, r, url, http.StatusFound)
}

func (h *AuthHandler) OAuth2Callback(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	code := r.FormValue("code")
	state := r.FormValue("state")

	if provider == "" || code == "" || state == "" {
		respondWithError(w, http.StatusBadRequest, "Missing parameters")
		return
	}

	cookie, err := r.Cookie("oauth_state_" + provider)
	if err != nil || cookie.Value != state {
		respondWithError(w, http.StatusBadRequest, "Invalid OAuth state")
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state_" + provider,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	// Retrieve PKCE verifier
	verifierCookie, err := r.Cookie("oauth_verifier_" + provider)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Missing OAuth verifier")
		return
	}
	verifier := verifierCookie.Value

	// Clear verifier cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_verifier_" + provider,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	setting, err := h.oauthRepo.FindByProvider(provider)
	if err != nil || setting == nil || !setting.Enabled {
		respondWithError(w, http.StatusBadRequest, "Provider settings error")
		return
	}

	config := h.getOAuthConfig(provider, setting, r)
	if config == nil {
		respondWithError(w, http.StatusBadRequest, "Unsupported provider")
		return
	}

	// Exchange using PKCE verifier
	token, err := config.Exchange(r.Context(), code, oauth2.SetAuthURLParam("code_verifier", verifier))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to exchange token: "+err.Error())
		return
	}

	client := config.Client(r.Context(), token)
	userInfoURL := getUserInfoURL(provider)
	resp, err := client.Get(userInfoURL)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch user profile: "+err.Error())
		return
	}
	defer resp.Body.Close()

	var profile struct {
		ID          string `json:"id"`
		Sub         string `json:"sub"`
		UserId      string `json:"userId"`
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to decode user profile: "+err.Error())
		return
	}

	var providerUserID string
	var defaultUsername string

	switch provider {
	case "google":
		providerUserID = profile.ID
		if providerUserID == "" {
			providerUserID = profile.Sub
		}
		defaultUsername = profile.Name
	case "line":
		providerUserID = profile.UserId
		defaultUsername = profile.DisplayName
	case "meta":
		providerUserID = profile.ID
		defaultUsername = profile.Name
	}

	if providerUserID == "" {
		respondWithError(w, http.StatusInternalServerError, "Could not retrieve user identity from provider")
		return
	}

	if defaultUsername == "" {
		defaultUsername = provider + "_user"
	}

	user, err := h.repo.FindByOAuth(provider, providerUserID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if user == nil {
		uniqueName := h.getUniqueUsername(defaultUsername)
		user = &model.User{
			Username: uniqueName,
		}
		oauthRecord := &model.UserOAuth{
			Provider:       provider,
			ProviderUserID: providerUserID,
		}
		if err := h.repo.CreateWithOAuth(user, oauthRecord); err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	jwtToken, err := GenerateToken(user.ID, user.Username)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" || os.Getenv("COOKIE_SECURE") == "true"
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    jwtToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect to frontend with success query parameter
	redirectURL := "/?oauth_success=true"
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *AuthHandler) getOAuthConfig(provider string, setting *model.OAuthSetting, r *http.Request) *oauth2.Config {
	var endpoint oauth2.Endpoint
	var scopes []string

	switch provider {
	case "google":
		endpoint = oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		}
		scopes = []string{"profile", "email"}
	case "line":
		endpoint = oauth2.Endpoint{
			AuthURL:  "https://access.line.me/oauth2/v2.1/authorize",
			TokenURL: "https://api.line.me/oauth2/v2.1/token",
		}
		scopes = []string{"profile"}
	case "meta":
		endpoint = oauth2.Endpoint{
			AuthURL:  "https://www.facebook.com/v12.0/dialog/oauth",
			TokenURL: "https://graph.facebook.com/v12.0/oauth/access_token",
		}
		scopes = []string{"public_profile"}
	default:
		return nil
	}

	redirectURL := h.getRedirectURL(provider, r)

	return &oauth2.Config{
		ClientID:     setting.ClientID,
		ClientSecret: setting.ClientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     endpoint,
		Scopes:       scopes,
	}
}

func (h *AuthHandler) getRedirectURL(provider string, r *http.Request) string {
	settings, err := h.siteSettingRepo.FindAll()
	var externalURL string
	if err == nil {
		externalURL = settings["external_url"]
	}

	if externalURL != "" {
		if externalURL[len(externalURL)-1] == '/' {
			externalURL = externalURL[:len(externalURL)-1]
		}
	} else {
		// Auto-resolve base URL from request
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
			scheme = proto
		}
		host := r.Host
		externalURL = fmt.Sprintf("%s://%s", scheme, host)
	}

	return fmt.Sprintf("%s/api/auth/oauth/callback/%s", externalURL, provider)
}

func getUserInfoURL(provider string) string {
	switch provider {
	case "google":
		return "https://www.googleapis.com/oauth2/v2/userinfo"
	case "line":
		return "https://api.line.me/v2/profile"
	case "meta":
		return "https://graph.facebook.com/me?fields=id,name"
	default:
		return ""
	}
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	respondWithJSON(w, http.StatusOK, map[string]string{"result": "success"})
}

func generateVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func generateChallenge(verifier string) string {
	sha := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sha[:])
}

func (h *AuthHandler) GetUsername(w http.ResponseWriter, r *http.Request) {
	userId := r.PathValue("userId")
	if userId == "" {
		respondWithError(w, http.StatusBadRequest, "Missing UserID")
		return
	}

	user, err := h.repo.FindByID(userId)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if user == nil {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"username": user.Username})
}
