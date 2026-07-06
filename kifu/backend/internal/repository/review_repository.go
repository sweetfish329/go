package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/sweetfish329/go/kifu/backend/internal/model"
)

type ReviewRepository struct {
	db *sql.DB
}

func NewReviewRepository(db *sql.DB) *ReviewRepository {
	return &ReviewRepository{db: db}
}

func (r *ReviewRepository) Save(review *model.Review) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("failed to generate UUIDv7: %w", err)
	}
	review.ID = id.String()

	query := `
	INSERT INTO reviews (
		id, kifu_id, move_number, node_path, reviewer_name, comment, variations
	) VALUES ($1, $2, $3, $4, $5, $6, $7)
	RETURNING created_at, updated_at`

	err = r.db.QueryRow(
		query,
		review.ID,
		review.KifuID,
		review.MoveNumber,
		review.NodePath,
		review.ReviewerName,
		review.Comment,
		review.Variations,
	).Scan(&review.CreatedAt, &review.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to save review: %w", err)
	}

	return nil
}

func (r *ReviewRepository) FindByKifuID(kifuID string) ([]*model.Review, error) {
	query := `
	SELECT id, kifu_id, move_number, node_path, reviewer_name, comment, variations, created_at, updated_at
	FROM reviews
	WHERE kifu_id = $1
	ORDER BY move_number ASC, created_at ASC`

	rows, err := r.db.Query(query, kifuID)
	if err != nil {
		return nil, fmt.Errorf("failed to find reviews by kifu_id: %w", err)
	}
	defer rows.Close()

	reviews := []*model.Review{}
	for rows.Next() {
		rev := &model.Review{}
		err := rows.Scan(
			&rev.ID, &rev.KifuID, &rev.MoveNumber, &rev.NodePath,
			&rev.ReviewerName, &rev.Comment, &rev.Variations,
			&rev.CreatedAt, &rev.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan review row: %w", err)
		}
		reviews = append(reviews, rev)
	}

	return reviews, nil
}

func (r *ReviewRepository) Update(review *model.Review) error {
	query := `
	UPDATE reviews
	SET reviewer_name = $1, comment = $2, variations = $3, updated_at = CURRENT_TIMESTAMP
	WHERE id = $4
	RETURNING updated_at`

	err := r.db.QueryRow(
		query,
		review.ReviewerName,
		review.Comment,
		review.Variations,
		review.ID,
	).Scan(&review.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("review not found for update")
		}
		return fmt.Errorf("failed to update review: %w", err)
	}

	return nil
}

func (r *ReviewRepository) Delete(id string) error {
	query := `DELETE FROM reviews WHERE id = $1`
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete review: %w", err)
	}
	return nil
}
