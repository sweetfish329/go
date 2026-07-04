package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/sweetfish329/go/kifu/backend/internal/model"
)

type KifuRepository struct {
	db *sql.DB
}

func NewKifuRepository(db *sql.DB) *KifuRepository {
	return &KifuRepository{db: db}
}

func (r *KifuRepository) Save(kifu *model.Kifu) error {
	query := `
	INSERT INTO kifus (
		title, black_player, black_rank, white_player, white_rank,
		game_date, result, komi, handicap, sgf_data
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	RETURNING id, created_at, updated_at`

	err := r.db.QueryRow(
		query,
		kifu.Title,
		kifu.BlackPlayer,
		kifu.BlackRank,
		kifu.WhitePlayer,
		kifu.WhiteRank,
		kifu.GameDate,
		kifu.Result,
		kifu.Komi,
		kifu.Handicap,
		kifu.SgfData,
	).Scan(&kifu.ID, &kifu.CreatedAt, &kifu.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to save kifu: %w", err)
	}

	return nil
}

func (r *KifuRepository) FindAll() ([]*model.Kifu, error) {
	query := `
	SELECT id, title, black_player, black_rank, white_player, white_rank,
	       game_date, result, komi, handicap, created_at, updated_at
	FROM kifus
	ORDER BY created_at DESC`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to find all kifus: %w", err)
	}
	defer rows.Close()

	kifus := []*model.Kifu{}
	for rows.Next() {
		k := &model.Kifu{}
		var gameDate string
		err := rows.Scan(
			&k.ID, &k.Title, &k.BlackPlayer, &k.BlackRank, &k.WhitePlayer, &k.WhiteRank,
			&gameDate, &k.Result, &k.Komi, &k.Handicap, &k.CreatedAt, &k.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan kifu row: %w", err)
		}
		// Formatting PostgreSQL date output (e.g. 2026-07-04T00:00:00Z) to YYYY-MM-DD
		if len(gameDate) >= 10 {
			k.GameDate = gameDate[:10]
		} else {
			k.GameDate = gameDate
		}
		kifus = append(kifus, k)
	}

	return kifus, nil
}

func (r *KifuRepository) FindByID(id string) (*model.Kifu, error) {
	query := `
	SELECT id, title, black_player, black_rank, white_player, white_rank,
	       game_date, result, komi, handicap, sgf_data, created_at, updated_at
	FROM kifus
	WHERE id = $1`

	k := &model.Kifu{}
	var gameDate string
	err := r.db.QueryRow(query, id).Scan(
		&k.ID, &k.Title, &k.BlackPlayer, &k.BlackRank, &k.WhitePlayer, &k.WhiteRank,
		&gameDate, &k.Result, &k.Komi, &k.Handicap, &k.SgfData, &k.CreatedAt, &k.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // not found
		}
		return nil, fmt.Errorf("failed to find kifu by id: %w", err)
	}

	if len(gameDate) >= 10 {
		k.GameDate = gameDate[:10]
	} else {
		k.GameDate = gameDate
	}

	return k, nil
}

func (r *KifuRepository) Delete(id string) error {
	query := `DELETE FROM kifus WHERE id = $1`
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete kifu: %w", err)
	}
	return nil
}
