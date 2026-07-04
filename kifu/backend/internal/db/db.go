package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

// InitDB initializes the database connection and runs migrations.
func InitDB() (*sql.DB, error) {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "postgres")
	dbname := getEnv("DB_NAME", "kifu_store")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	var db *sql.DB
	var err error

	// Retry connection since DB container might start slightly after backend
	for i := 0; i < 10; i++ {
		db, err = sql.Open("postgres", connStr)
		if err == nil {
			err = db.Ping()
			if err == nil {
				break
			}
		}
		log.Printf("Waiting for database connection (attempt %d/10)... error: %v", i+1, err)
		time.Sleep(3 * time.Second) // 3 seconds
	}

	if err != nil {
		return nil, fmt.Errorf("could not connect to database after retries: %w", err)
	}

	log.Println("Database connection established successfully.")

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
	// Create kifus table
	kifusTableQuery := `
	CREATE TABLE IF NOT EXISTS kifus (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		title VARCHAR(255) NOT NULL,
		black_player VARCHAR(100) NOT NULL,
		black_rank VARCHAR(20) NOT NULL,
		white_player VARCHAR(100) NOT NULL,
		white_rank VARCHAR(20) NOT NULL,
		game_date DATE NOT NULL,
		result VARCHAR(50) NOT NULL,
		komi NUMERIC(3, 1) NOT NULL DEFAULT 6.5,
		handicap INTEGER NOT NULL DEFAULT 0,
		sgf_data TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	// Create reviews table
	reviewsTableQuery := `
	CREATE TABLE IF NOT EXISTS reviews (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		kifu_id UUID NOT NULL REFERENCES kifus(id) ON DELETE CASCADE,
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

	_, err := db.Exec(kifusTableQuery)
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

	log.Println("Database migrations applied successfully.")
	return nil
}
