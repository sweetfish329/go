package model

import "time"

// Kifu represents the metadata and raw SGF data of a Go game.
type Kifu struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	BlackPlayer    string     `json:"black_player"`
	BlackRank      string     `json:"black_rank"`
	WhitePlayer    string     `json:"white_player"`
	WhiteRank      string     `json:"white_rank"`
	GameDate       string     `json:"game_date"`
	Result         string     `json:"result"`
	Komi           float64    `json:"komi"`
	Handicap       int        `json:"handicap"`
	SgfData        string     `json:"sgf_data"`
	UploadedBy     *string    `json:"uploaded_by,omitempty"`
	ShareToken     *string    `json:"share_token,omitempty"`
	ShareExpiresAt *time.Time `json:"share_expires_at,omitempty"`
	IsPrivate      bool       `json:"is_private"`
	OgpImage       []byte     `json:"-"` // OGP image binary (PNG)
	HasOgp         bool       `json:"has_ogp"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
