package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/sweetfish329/go/kifu/backend/internal/model"
)

type KifuRepository struct {
	db *sql.DB
}

func NewKifuRepository(db *sql.DB) *KifuRepository {
	return &KifuRepository{db: db}
}

func (r *KifuRepository) Save(kifu *model.Kifu) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("failed to generate UUIDv7: %w", err)
	}
	kifu.ID = id.String()

	query := `
	INSERT INTO kifus (
		id, title, black_player, black_rank, white_player, white_rank,
		game_date, result, komi, handicap, sgf_data, uploaded_by, is_private
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	RETURNING created_at, updated_at`

	err = r.db.QueryRow(
		query,
		kifu.ID,
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
		kifu.UploadedBy,
		kifu.IsPrivate,
	).Scan(&kifu.CreatedAt, &kifu.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to save kifu: %w", err)
	}

	return nil
}

func (r *KifuRepository) FindAllByUser(userID string) ([]*model.Kifu, error) {
	query := `
	SELECT id, title, black_player, black_rank, white_player, white_rank,
	       game_date, result, komi, handicap, uploaded_by, share_token, share_expires_at, is_private, created_at, updated_at,
	       (ogp_image IS NOT NULL AND octet_length(ogp_image) > 0) AS has_ogp
	FROM kifus
	WHERE uploaded_by = $1
	ORDER BY created_at DESC`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to find all user kifus: %w", err)
	}
	defer rows.Close()

	kifus := []*model.Kifu{}
	for rows.Next() {
		k := &model.Kifu{}
		var gameDate string
		err := rows.Scan(
			&k.ID, &k.Title, &k.BlackPlayer, &k.BlackRank, &k.WhitePlayer, &k.WhiteRank,
			&gameDate, &k.Result, &k.Komi, &k.Handicap, &k.UploadedBy, &k.ShareToken, &k.ShareExpiresAt, &k.IsPrivate, &k.CreatedAt, &k.UpdatedAt,
			&k.HasOgp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan kifu row: %w", err)
		}
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
	       game_date, result, komi, handicap, sgf_data, uploaded_by, share_token, share_expires_at, is_private, created_at, updated_at
	FROM kifus
	WHERE id = $1`

	k := &model.Kifu{}
	var gameDate string
	err := r.db.QueryRow(query, id).Scan(
		&k.ID, &k.Title, &k.BlackPlayer, &k.BlackRank, &k.WhitePlayer, &k.WhiteRank,
		&gameDate, &k.Result, &k.Komi, &k.Handicap, &k.SgfData, &k.UploadedBy, &k.ShareToken, &k.ShareExpiresAt, &k.IsPrivate, &k.CreatedAt, &k.UpdatedAt,
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

func (r *KifuRepository) FindByShareToken(token string) (*model.Kifu, error) {
	query := `
	SELECT id, title, black_player, black_rank, white_player, white_rank,
	       game_date, result, komi, handicap, sgf_data, uploaded_by, share_token, share_expires_at, is_private, created_at, updated_at
	FROM kifus
	WHERE share_token = $1`

	k := &model.Kifu{}
	var gameDate string
	err := r.db.QueryRow(query, token).Scan(
		&k.ID, &k.Title, &k.BlackPlayer, &k.BlackRank, &k.WhitePlayer, &k.WhiteRank,
		&gameDate, &k.Result, &k.Komi, &k.Handicap, &k.SgfData, &k.UploadedBy, &k.ShareToken, &k.ShareExpiresAt, &k.IsPrivate, &k.CreatedAt, &k.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find kifu by share token: %w", err)
	}

	if len(gameDate) >= 10 {
		k.GameDate = gameDate[:10]
	} else {
		k.GameDate = gameDate
	}

	return k, nil
}

func (r *KifuRepository) FindAllPublicByUser(userID string) ([]*model.Kifu, error) {
	query := `
	SELECT id, title, black_player, black_rank, white_player, white_rank,
	       game_date, result, komi, handicap, uploaded_by, share_token, share_expires_at, is_private, created_at, updated_at,
	       (ogp_image IS NOT NULL AND octet_length(ogp_image) > 0) AS has_ogp
	FROM kifus
	WHERE uploaded_by = $1 AND is_private = false
	ORDER BY created_at DESC`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to find all public user kifus: %w", err)
	}
	defer rows.Close()

	kifus := []*model.Kifu{}
	for rows.Next() {
		k := &model.Kifu{}
		var gameDate string
		err := rows.Scan(
			&k.ID, &k.Title, &k.BlackPlayer, &k.BlackRank, &k.WhitePlayer, &k.WhiteRank,
			&gameDate, &k.Result, &k.Komi, &k.Handicap, &k.UploadedBy, &k.ShareToken, &k.ShareExpiresAt, &k.IsPrivate, &k.CreatedAt, &k.UpdatedAt,
			&k.HasOgp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan public kifu row: %w", err)
		}
		if len(gameDate) >= 10 {
			k.GameDate = gameDate[:10]
		} else {
			k.GameDate = gameDate
		}
		kifus = append(kifus, k)
	}

	return kifus, nil
}

func (r *KifuRepository) FindByIDAndUser(id string, userID string) (*model.Kifu, error) {
	query := `
	SELECT id, title, black_player, black_rank, white_player, white_rank,
	       game_date, result, komi, handicap, sgf_data, uploaded_by, share_token, share_expires_at, is_private, created_at, updated_at
	FROM kifus
	WHERE id = $1 AND uploaded_by = $2`

	k := &model.Kifu{}
	var gameDate string
	err := r.db.QueryRow(query, id, userID).Scan(
		&k.ID, &k.Title, &k.BlackPlayer, &k.BlackRank, &k.WhitePlayer, &k.WhiteRank,
		&gameDate, &k.Result, &k.Komi, &k.Handicap, &k.SgfData, &k.UploadedBy, &k.ShareToken, &k.ShareExpiresAt, &k.IsPrivate, &k.CreatedAt, &k.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // not found
		}
		return nil, fmt.Errorf("failed to find kifu by id and user: %w", err)
	}

	if len(gameDate) >= 10 {
		k.GameDate = gameDate[:10]
	} else {
		k.GameDate = gameDate
	}

	return k, nil
}

func (r *KifuRepository) UpdateShare(id string, token *string, expiresAt *interface{}) error {
	query := `
	UPDATE kifus
	SET share_token = $1, share_expires_at = $2, updated_at = CURRENT_TIMESTAMP
	WHERE id = $3`

	_, err := r.db.Exec(query, token, expiresAt, id)
	if err != nil {
		return fmt.Errorf("failed to update kifu share settings: %w", err)
	}
	return nil
}

func (r *KifuRepository) UpdatePrivacy(id string, isPrivate bool) error {
	query := `
	UPDATE kifus
	SET is_private = $1, updated_at = CURRENT_TIMESTAMP
	WHERE id = $2`

	_, err := r.db.Exec(query, isPrivate, id)
	if err != nil {
		return fmt.Errorf("failed to update kifu privacy settings: %w", err)
	}
	return nil
}

func (r *KifuRepository) Delete(id string) error {
	query := `DELETE FROM kifus WHERE id = $1`
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete kifu: %w", err)
	}
	return nil
}

func (r *KifuRepository) UpdateOgpImage(id string, imgData []byte) error {
	query := `UPDATE kifus SET ogp_image = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`
	_, err := r.db.Exec(query, imgData, id)
	if err != nil {
		return fmt.Errorf("failed to update ogp image: %w", err)
	}
	return nil
}

func (r *KifuRepository) GetOgpImage(id string) ([]byte, error) {
	query := `SELECT ogp_image FROM kifus WHERE id = $1`
	var imgData []byte
	err := r.db.QueryRow(query, id).Scan(&imgData)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get ogp image: %w", err)
	}
	return imgData, nil
}

func (r *KifuRepository) GetOgpImageByShareToken(token string) ([]byte, error) {
	query := `SELECT ogp_image FROM kifus WHERE share_token = $1`
	var imgData []byte
	err := r.db.QueryRow(query, token).Scan(&imgData)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get ogp image by share token: %w", err)
	}
	return imgData, nil
}

func (r *KifuRepository) FindRandomPublic() (*model.Kifu, error) {
	query := `
	SELECT id, title, black_player, black_rank, white_player, white_rank,
	       game_date, result, komi, handicap, sgf_data, uploaded_by, share_token, share_expires_at, is_private, created_at, updated_at
	FROM kifus
	WHERE is_private = false
	ORDER BY RANDOM()
	LIMIT 1`

	k := &model.Kifu{}
	var gameDate string
	err := r.db.QueryRow(query).Scan(
		&k.ID, &k.Title, &k.BlackPlayer, &k.BlackRank, &k.WhitePlayer, &k.WhiteRank,
		&gameDate, &k.Result, &k.Komi, &k.Handicap, &k.SgfData, &k.UploadedBy, &k.ShareToken, &k.ShareExpiresAt, &k.IsPrivate, &k.CreatedAt, &k.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // not found
		}
		return nil, fmt.Errorf("failed to find random public kifu: %w", err)
	}

	if len(gameDate) >= 10 {
		k.GameDate = gameDate[:10]
	} else {
		k.GameDate = gameDate
	}

	return k, nil
}
