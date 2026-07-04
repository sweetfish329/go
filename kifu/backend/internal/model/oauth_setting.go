package model

import "time"

type OAuthSetting struct {
	Provider     string    `json:"provider"`
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"client_secret"`
	RedirectURL  string    `json:"redirect_url"`
	Enabled      bool      `json:"enabled"`
	UpdatedAt    time.Time `json:"updated_at"`
}
