package repository

import (
	"database/sql"
	"fmt"

	"github.com/sweetfish329/go/kifu/backend/internal/model"
)

type OAuthRepository struct {
	db *sql.DB
}

func NewOAuthRepository(db *sql.DB) *OAuthRepository {
	return &OAuthRepository{db: db}
}

func (r *OAuthRepository) FindAll() ([]*model.OAuthSetting, error) {
	query := `SELECT provider, client_id, client_secret, redirect_url, enabled, updated_at FROM oauth_settings`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch oauth settings: %w", err)
	}
	defer rows.Close()

	settings := []*model.OAuthSetting{}
	for rows.Next() {
		s := &model.OAuthSetting{}
		if err := rows.Scan(&s.Provider, &s.ClientID, &s.ClientSecret, &s.RedirectURL, &s.Enabled, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan oauth setting: %w", err)
		}
		settings = append(settings, s)
	}
	return settings, nil
}

func (r *OAuthRepository) FindByProvider(provider string) (*model.OAuthSetting, error) {
	query := `SELECT provider, client_id, client_secret, redirect_url, enabled, updated_at FROM oauth_settings WHERE provider = $1`
	s := &model.OAuthSetting{}
	err := r.db.QueryRow(query, provider).Scan(&s.Provider, &s.ClientID, &s.ClientSecret, &s.RedirectURL, &s.Enabled, &s.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch oauth setting for provider %s: %w", provider, err)
	}
	return s, nil
}

func (r *OAuthRepository) Save(s *model.OAuthSetting) error {
	query := `
	INSERT INTO oauth_settings (provider, client_id, client_secret, redirect_url, enabled, updated_at)
	VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
	ON CONFLICT (provider) DO UPDATE
	SET client_id = EXCLUDED.client_id,
	    client_secret = EXCLUDED.client_secret,
	    redirect_url = EXCLUDED.redirect_url,
	    enabled = EXCLUDED.enabled,
	    updated_at = CURRENT_TIMESTAMP`

	_, err := r.db.Exec(query, s.Provider, s.ClientID, s.ClientSecret, s.RedirectURL, s.Enabled)
	if err != nil {
		return fmt.Errorf("failed to save oauth setting: %w", err)
	}
	return nil
}
