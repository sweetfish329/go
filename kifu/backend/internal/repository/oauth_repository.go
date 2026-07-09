package repository

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"github.com/sweetfish329/go/kifu/backend/internal/model"
)

type OAuthRepository struct {
	db *sql.DB
}

func NewOAuthRepository(db *sql.DB) *OAuthRepository {
	return &OAuthRepository{db: db}
}

func getEncryptionKey() []byte {
	keyStr := os.Getenv("ENCRYPTION_KEY")
	if keyStr == "" {
		keyStr = "kifu-default-encryption-key-for-oauth"
	}
	hash := sha256.Sum256([]byte(keyStr))
	return hash[:]
}

func encrypt(text string) (string, error) {
	key := getEncryptionKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(text), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decrypt(cryptoText string) (string, error) {
	key := getEncryptionKey()
	ciphertext, err := base64.StdEncoding.DecodeString(cryptoText)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
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
		if s.ClientSecret != "" {
			decrypted, err := decrypt(s.ClientSecret)
			if err == nil {
				s.ClientSecret = decrypted
			}
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
	if s.ClientSecret != "" {
		decrypted, err := decrypt(s.ClientSecret)
		if err == nil {
			s.ClientSecret = decrypted
		}
	}
	return s, nil
}

func (r *OAuthRepository) Save(s *model.OAuthSetting) error {
	encryptedSecret := s.ClientSecret
	if s.ClientSecret != "" {
		enc, err := encrypt(s.ClientSecret)
		if err == nil {
			encryptedSecret = enc
		}
	}

	query := `
	INSERT INTO oauth_settings (provider, client_id, client_secret, redirect_url, enabled, updated_at)
	VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
	ON CONFLICT (provider) DO UPDATE
	SET client_id = EXCLUDED.client_id,
	    client_secret = EXCLUDED.client_secret,
	    redirect_url = EXCLUDED.redirect_url,
	    enabled = EXCLUDED.enabled,
	    updated_at = CURRENT_TIMESTAMP`

	_, err := r.db.Exec(query, s.Provider, s.ClientID, encryptedSecret, s.RedirectURL, s.Enabled)
	if err != nil {
		return fmt.Errorf("failed to save oauth setting: %w", err)
	}
	return nil
}
