package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/sweetfish329/go/kifu/backend/internal/model"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *model.User) error {
	query := `
	INSERT INTO users (username, password_hash)
	VALUES ($1, $2)
	RETURNING id, created_at, updated_at`

	err := r.db.QueryRow(query, user.Username, user.PasswordHash).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func (r *UserRepository) FindByUsername(username string) (*model.User, error) {
	query := `
	SELECT id, username, password_hash, created_at, updated_at
	FROM users
	WHERE username = $1`

	var user model.User
	err := r.db.QueryRow(query, username).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find user by username: %w", err)
	}

	return &user, nil
}

func (r *UserRepository) FindByID(id string) (*model.User, error) {
	query := `
	SELECT id, username, password_hash, created_at, updated_at
	FROM users
	WHERE id = $1`

	var user model.User
	err := r.db.QueryRow(query, id).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find user by id: %w", err)
	}

	return &user, nil
}
