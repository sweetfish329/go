package repository

import (
	"database/sql"
	"fmt"
)

type SiteSettingRepository struct {
	db *sql.DB
}

func NewSiteSettingRepository(db *sql.DB) *SiteSettingRepository {
	return &SiteSettingRepository{db: db}
}

func (r *SiteSettingRepository) FindAll() (map[string]string, error) {
	query := `SELECT key, value FROM site_settings`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch site settings: %w", err)
	}
	defer rows.Close()

	settings := make(map[string]string)
	// Initialize default values just in case
	settings["title"] = "kifu_store"
	settings["tab_name"] = "kifu_store"
	settings["favicon"] = ""
	settings["theme_color"] = "#4e342e"
	settings["external_url"] = ""

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan site setting: %w", err)
		}
		settings[key] = value
	}
	return settings, nil
}

func (r *SiteSettingRepository) Save(key, value string) error {
	query := `
	INSERT INTO site_settings (key, value, updated_at)
	VALUES ($1, $2, CURRENT_TIMESTAMP)
	ON CONFLICT (key) DO UPDATE
	SET value = EXCLUDED.value,
	    updated_at = CURRENT_TIMESTAMP`

	_, err := r.db.Exec(query, key, value)
	if err != nil {
		return fmt.Errorf("failed to save site setting: %w", err)
	}
	return nil
}
