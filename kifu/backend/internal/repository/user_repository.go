package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/sweetfish329/go/kifu/backend/internal/model"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *model.User) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("failed to generate UUIDv7: %w", err)
	}
	user.ID = id.String()

	query := `
	INSERT INTO users (id, username, password_hash)
	VALUES ($1, $2, $3)
	RETURNING created_at, updated_at`

	err = r.db.QueryRow(query, user.ID, user.Username, user.PasswordHash).Scan(&user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func (r *UserRepository) UpdateUsername(userID string, newUsername string) error {
	query := `
	UPDATE users
	SET username = $1, updated_at = CURRENT_TIMESTAMP
	WHERE id = $2`

	_, err := r.db.Exec(query, newUsername, userID)
	if err != nil {
		return fmt.Errorf("failed to update username: %w", err)
	}
	return nil
}

func (r *UserRepository) CreateWithOAuth(user *model.User, oauth *model.UserOAuth) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	uID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("failed to generate User UUIDv7: %w", err)
	}
	user.ID = uID.String()

	oID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("failed to generate OAuth UUIDv7: %w", err)
	}
	oauth.ID = oID.String()

	// 1. Insert User (password_hash will be NULL)
	userQuery := `
	INSERT INTO users (id, username, password_hash)
	VALUES ($1, $2, NULL)
	RETURNING created_at, updated_at`

	err = tx.QueryRow(userQuery, user.ID, user.Username).Scan(&user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create user inside transaction: %w", err)
	}

	// 2. Insert UserOAuth
	oauthQuery := `
	INSERT INTO user_oauths (id, user_id, provider, provider_user_id)
	VALUES ($1, $2, $3, $4)
	RETURNING created_at, updated_at`

	err = tx.QueryRow(oauthQuery, oauth.ID, user.ID, oauth.Provider, oauth.ProviderUserID).Scan(&oauth.CreatedAt, &oauth.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create user oauth: %w", err)
	}

	oauth.UserID = user.ID

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *UserRepository) FindByOAuth(provider string, providerUserID string) (*model.User, error) {
	query := `
	SELECT u.id, u.username, u.password_hash, u.created_at, u.updated_at
	FROM users u
	INNER JOIN user_oauths o ON u.id = o.user_id
	WHERE o.provider = $1 AND o.provider_user_id = $2`

	var user model.User
	err := r.db.QueryRow(query, provider, providerUserID).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find user by OAuth: %w", err)
	}

	return &user, nil
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
