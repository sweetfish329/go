package model

import "time"

// Review represents comments or game variations added to a specific node/move in a Kifu.
type Review struct {
	ID           string    `json:"id"`
	KifuID       string    `json:"kifu_id"`
	UserID       *string   `json:"user_id,omitempty"` // ID of the reviewer (null if anonymous/guest)
	MoveNumber   int       `json:"move_number"`
	NodePath     string    `json:"node_path"`
	ReviewerName string    `json:"reviewer_name"`
	Comment      string    `json:"comment"`
	Variations   string    `json:"variations"` // SGF subtree string for branch representation
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
