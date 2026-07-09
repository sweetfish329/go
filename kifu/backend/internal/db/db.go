package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// InitDB initializes the database connection and runs migrations.
func InitDB() (*sql.DB, error) {
	dbPath := getEnv("DB_PATH", "kifu.db")

	// Ensure the parent directory of dbPath exists (especially important in container)
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	var db *sql.DB
	var err error

	// SQLite usually starts instantly, but retrying is harmless
	for i := 0; i < 5; i++ {
		db, err = sql.Open("sqlite", dbPath)
		if err == nil {
			err = db.Ping()
			if err == nil {
				break
			}
		}
		log.Printf("Waiting for database connection (attempt %d/5)... error: %v", i+1, err)
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("could not connect to SQLite database: %w", err)
	}

	log.Printf("SQLite database connection established successfully at %s.", dbPath)

	// Enable Write-Ahead Logging (WAL) mode for better concurrent performance in SQLite
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		log.Printf("Warning: failed to enable WAL mode: %v", err)
	}
	// Enable foreign key support
	if _, err := db.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		log.Printf("Warning: failed to enable foreign keys: %v", err)
	}

	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func runMigrations(db *sql.DB) error {
	// Create users table (password_hash is nullable to support OAuth-only accounts)
	usersTableQuery := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username VARCHAR(100) UNIQUE NOT NULL,
		password_hash VARCHAR(255),
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	// Create user_oauths table
	oauthsTableQuery := `
	CREATE TABLE IF NOT EXISTS user_oauths (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		provider VARCHAR(50) NOT NULL,
		provider_user_id VARCHAR(255) NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT unique_provider_user UNIQUE (provider, provider_user_id)
	);`

	// Create kifus table
	kifusTableQuery := `
	CREATE TABLE IF NOT EXISTS kifus (
		id TEXT PRIMARY KEY,
		title VARCHAR(255) NOT NULL,
		black_player VARCHAR(100) NOT NULL,
		black_rank VARCHAR(20) NOT NULL,
		white_player VARCHAR(100) NOT NULL,
		white_rank VARCHAR(20) NOT NULL,
		game_date TEXT NOT NULL,
		result VARCHAR(50) NOT NULL,
		komi NUMERIC(3, 1) NOT NULL DEFAULT 6.5,
		handicap INTEGER NOT NULL DEFAULT 0,
		sgf_data TEXT NOT NULL,
		uploaded_by TEXT REFERENCES users(id) ON DELETE SET NULL,
		share_token VARCHAR(100) UNIQUE,
		share_expires_at TIMESTAMP,
		is_private BOOLEAN NOT NULL DEFAULT TRUE,
		ogp_image BLOB,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	// Create reviews table
	reviewsTableQuery := `
	CREATE TABLE IF NOT EXISTS reviews (
		id TEXT PRIMARY KEY,
		kifu_id TEXT NOT NULL REFERENCES kifus(id) ON DELETE CASCADE,
		user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
		move_number INTEGER NOT NULL,
		node_path VARCHAR(255) NOT NULL,
		reviewer_name VARCHAR(100) NOT NULL,
		comment TEXT NOT NULL,
		variations TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	// Create index for performance
	indexQuery := `
	CREATE INDEX IF NOT EXISTS idx_reviews_kifu_id ON reviews(kifu_id);
	`

	// Create oauth_settings table
	oauthSettingsTableQuery := `
	CREATE TABLE IF NOT EXISTS oauth_settings (
		provider VARCHAR(50) PRIMARY KEY,
		client_id TEXT NOT NULL,
		client_secret TEXT NOT NULL,
		redirect_url TEXT NOT NULL,
		enabled BOOLEAN NOT NULL DEFAULT true,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	// Create site_settings table
	siteSettingsTableQuery := `
	CREATE TABLE IF NOT EXISTS site_settings (
		key VARCHAR(100) PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := db.Exec(usersTableQuery)
	if err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	_, err = db.Exec(oauthsTableQuery)
	if err != nil {
		return fmt.Errorf("failed to create user_oauths table: %w", err)
	}

	_, err = db.Exec(kifusTableQuery)
	if err != nil {
		return fmt.Errorf("failed to create kifus table: %w", err)
	}

	_, err = db.Exec(reviewsTableQuery)
	if err != nil {
		return fmt.Errorf("failed to create reviews table: %w", err)
	}

	_, err = db.Exec(indexQuery)
	if err != nil {
		return fmt.Errorf("failed to create reviews index: %w", err)
	}

	_, err = db.Exec(oauthSettingsTableQuery)
	if err != nil {
		return fmt.Errorf("failed to create oauth_settings table: %w", err)
	}

	_, err = db.Exec(siteSettingsTableQuery)
	if err != nil {
		return fmt.Errorf("failed to create site_settings table: %w", err)
	}

	// Run migration to add user_id to reviews if it doesn't exist
	var hasUserID bool
	rows, err := db.Query("PRAGMA table_info(reviews)")
	if err != nil {
		return fmt.Errorf("failed to check reviews table info: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dfltValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err == nil {
			if name == "user_id" {
				hasUserID = true
			}
		}
	}
	if !hasUserID {
		_, err = db.Exec("ALTER TABLE reviews ADD COLUMN user_id TEXT REFERENCES users(id) ON DELETE SET NULL")
		if err != nil {
			log.Printf("Warning: failed to add user_id column to reviews table (might already exist): %v", err)
		}
	}

	// Insert default settings
	insertDefaultSettingsQuery := `
	INSERT INTO site_settings (key, value) VALUES
	('title', 'kifu_store'),
	('tab_name', 'kifu_store'),
	('favicon', '/kifu-favicon.ico'),
	('theme_color', '#4e342e'),
	('external_url', '')
	ON CONFLICT (key) DO NOTHING;`

	_, err = db.Exec(insertDefaultSettingsQuery)
	if err != nil {
		return fmt.Errorf("failed to insert default site settings: %w", err)
	}

	log.Println("Database migrations applied successfully.")
	return nil
}
