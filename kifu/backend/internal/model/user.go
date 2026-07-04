package model

import "time"

// User represents a user who can upload and review kifus.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash *string   `json:"-"` // Pointer to support nullable password_hash
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserOAuth represents an OAuth identity linked to a user account.
type UserOAuth struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	Provider       string    `json:"provider"`
	ProviderUserID string    `json:"provider_user_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
