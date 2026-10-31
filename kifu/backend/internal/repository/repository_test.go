package repository

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/sweetfish329/go/kifu/backend/internal/model"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory sqlite: %v", err)
	}

	schema := `
	CREATE TABLE users (
		id TEXT PRIMARY KEY,
		username VARCHAR(100) UNIQUE NOT NULL,
		password_hash VARCHAR(255),
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE user_oauths (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		provider VARCHAR(50) NOT NULL,
		provider_user_id VARCHAR(255) NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT unique_provider_user UNIQUE (provider, provider_user_id)
	);
	CREATE TABLE kifus (
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
		thumbnail_image BLOB,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE reviews (
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
	);
	CREATE TABLE oauth_settings (
		provider VARCHAR(50) PRIMARY KEY,
		client_id TEXT NOT NULL,
		client_secret TEXT NOT NULL,
		redirect_url TEXT NOT NULL,
		enabled BOOLEAN NOT NULL DEFAULT true,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE site_settings (
		key VARCHAR(100) PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	`

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("Failed to initialize test schema: %v", err)
	}

	return db
}

func TestUserRepository(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)

	pwdHash := "hash123"
	user := &model.User{
		Username:     "tester",
		PasswordHash: &pwdHash,
	}

	if err := repo.Create(user); err != nil {
		t.Fatalf("Create user failed: %v", err)
	}
	if user.ID == "" {
		t.Fatal("Expected non-empty user ID")
	}

	// FindByID
	found, err := repo.FindByID(user.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if found == nil || found.Username != "tester" {
		t.Errorf("Expected username 'tester', got %v", found)
	}

	// UpdateUsername
	if err := repo.UpdateUsername(user.ID, "tester_new"); err != nil {
		t.Fatalf("UpdateUsername failed: %v", err)
	}
	found, _ = repo.FindByID(user.ID)
	if found.Username != "tester_new" {
		t.Errorf("Expected username 'tester_new', got %q", found.Username)
	}

	// CreateWithOAuth
	oauthUser := &model.User{Username: "oauth_tester"}
	oauth := &model.UserOAuth{Provider: "google", ProviderUserID: "g-12345"}
	if err := repo.CreateWithOAuth(oauthUser, oauth); err != nil {
		t.Fatalf("CreateWithOAuth failed: %v", err)
	}

	foundOAuth, err := repo.FindByOAuth("google", "g-12345")
	if err != nil {
		t.Fatalf("FindByOAuth failed: %v", err)
	}
	if foundOAuth == nil || foundOAuth.Username != "oauth_tester" {
		t.Errorf("Expected oauth user 'oauth_tester', got %v", foundOAuth)
	}
}

func TestKifuRepository(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := NewUserRepository(db)
	user := &model.User{Username: "kifu_owner"}
	if err := userRepo.Create(user); err != nil {
		t.Fatalf("Create user failed: %v", err)
	}

	repo := NewKifuRepository(db)
	kifu := &model.Kifu{
		Title:          "Test Match",
		BlackPlayer:    "Player A",
		BlackRank:      "1d",
		WhitePlayer:    "Player B",
		WhiteRank:      "2d",
		GameDate:       "2026-03-19",
		Result:         "B+R",
		Komi:           6.5,
		Handicap:       0,
		SgfData:        "(;GM[1])",
		ThumbnailImage: []byte("original_thumbnail_bytes"),
		UploadedBy:     &user.ID,
		IsPrivate:      true,
	}

	if err := repo.Save(kifu); err != nil {
		t.Fatalf("Save kifu failed: %v", err)
	}
	if kifu.ID == "" {
		t.Fatal("Expected non-empty kifu ID")
	}

	// FindByID
	found, err := repo.FindByID(kifu.ID)
	if err != nil || found == nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if found.Title != "Test Match" {
		t.Errorf("Expected title 'Test Match', got %q", found.Title)
	}
	if found.SgfData != "(;GM[1])" {
		t.Errorf("Expected SgfData '(;GM[1])', got %q", found.SgfData)
	}

	// Legacy plain text compatibility test
	_, err = db.Exec(`
		INSERT INTO kifus (
			id, title, black_player, black_rank, white_player, white_rank,
			game_date, result, komi, handicap, sgf_data, uploaded_by, is_private
		) VALUES ('legacy-id', 'Legacy Match', 'P1', '1d', 'P2', '2d', '2026-01-01', 'B+R', 6.5, 0, '(;GM[1]SZ[19])', $1, true)
	`, user.ID)
	if err != nil {
		t.Fatalf("Failed to insert legacy plain text kifu: %v", err)
	}
	legacyFound, err := repo.FindByID("legacy-id")
	if err != nil || legacyFound == nil {
		t.Fatalf("Failed to find legacy kifu: %v", err)
	}
	if legacyFound.SgfData != "(;GM[1]SZ[19])" {
		t.Errorf("Expected legacy SgfData '(;GM[1]SZ[19])', got %q", legacyFound.SgfData)
	}

	// UpdateShare & FindByShareToken
	token := "share-token-xyz"
	var exp interface{} = time.Now().Add(24 * time.Hour)
	if err := repo.UpdateShare(kifu.ID, &token, &exp); err != nil {
		t.Fatalf("UpdateShare failed: %v", err)
	}

	shared, err := repo.FindByShareToken(token)
	if err != nil || shared == nil {
		t.Fatalf("FindByShareToken failed: %v", err)
	}
	if shared.ID != kifu.ID {
		t.Errorf("Expected shared kifu ID %q, got %q", kifu.ID, shared.ID)
	}

	// UpdatePrivacy & FindAllPublicByUser
	if err := repo.UpdatePrivacy(kifu.ID, false); err != nil {
		t.Fatalf("UpdatePrivacy failed: %v", err)
	}
	publicKifus, err := repo.FindAllPublicByUser(user.ID)
	if err != nil {
		t.Fatalf("FindAllPublicByUser failed: %v", err)
	}
	if len(publicKifus) != 1 {
		t.Errorf("Expected 1 public kifu, got %d", len(publicKifus))
	}

	// UpdateOgpImage should update ogp_image without overwriting thumbnail_image
	ogpData := []byte("fake_ogp_image_bytes")
	if err := repo.UpdateOgpImage(kifu.ID, ogpData); err != nil {
		t.Fatalf("UpdateOgpImage failed: %v", err)
	}
	gotOgp, err := repo.GetOgpImage(kifu.ID)
	if err != nil || string(gotOgp) != "fake_ogp_image_bytes" {
		t.Errorf("GetOgpImage failed, got: %s", string(gotOgp))
	}
	gotThumb, err := repo.GetThumbnailImage(kifu.ID)
	if err != nil || string(gotThumb) != "original_thumbnail_bytes" {
		t.Errorf("GetThumbnailImage should remain unchanged, got: %s", string(gotThumb))
	}

	// Delete
	if err := repo.Delete(kifu.ID); err != nil {
		t.Fatalf("Delete kifu failed: %v", err)
	}
	foundAfterDelete, _ := repo.FindByID(kifu.ID)
	if foundAfterDelete != nil {
		t.Errorf("Expected nil after delete, got %v", foundAfterDelete)
	}
}

func TestReviewRepository(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := NewUserRepository(db)
	user := &model.User{Username: "reviewer"}
	_ = userRepo.Create(user)

	kifuRepo := NewKifuRepository(db)
	kifu := &model.Kifu{
		Title:          "Review Game",
		BlackPlayer:    "A",
		WhitePlayer:    "B",
		GameDate:       "2026-03-19",
		Result:         "B+R",
		SgfData:        "(;GM[1])",
		ThumbnailImage: []byte("original_thumbnail_bytes"),
		UploadedBy:     &user.ID,
		IsPrivate:      true,
	}
	_ = kifuRepo.Save(kifu)

	reviewRepo := NewReviewRepository(db)
	review := &model.Review{
		KifuID:       kifu.ID,
		UserID:       &user.ID,
		MoveNumber:   10,
		NodePath:     "0_10",
		ReviewerName: "Sensei",
		Comment:      "Nice move here",
		Variations:   "W[dp]",
	}

	if err := reviewRepo.Save(review); err != nil {
		t.Fatalf("Save review failed: %v", err)
	}
	if review.ID == "" {
		t.Fatal("Expected non-empty review ID")
	}

	// FindByKifuID
	reviews, err := reviewRepo.FindByKifuID(kifu.ID)
	if err != nil {
		t.Fatalf("FindByKifuID failed: %v", err)
	}
	if len(reviews) != 1 || reviews[0].Comment != "Nice move here" {
		t.Errorf("Expected 1 review with 'Nice move here', got %v", reviews)
	}

	// Update
	review.Comment = "Even better move"
	if err := reviewRepo.Update(review); err != nil {
		t.Fatalf("Update review failed: %v", err)
	}
	found, _ := reviewRepo.FindByID(review.ID)
	if found.Comment != "Even better move" {
		t.Errorf("Expected comment 'Even better move', got %q", found.Comment)
	}

	// Delete
	if err := reviewRepo.Delete(review.ID); err != nil {
		t.Fatalf("Delete review failed: %v", err)
	}
	foundAfterDelete, _ := reviewRepo.FindByID(review.ID)
	if foundAfterDelete != nil {
		t.Errorf("Expected nil after delete, got %v", foundAfterDelete)
	}
}

func TestSiteSettingRepository(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSiteSettingRepository(db)
	if err := repo.Save("title", "Custom Title"); err != nil {
		t.Fatalf("Save site setting failed: %v", err)
	}

	settings, err := repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll site settings failed: %v", err)
	}
	if settings["title"] != "Custom Title" {
		t.Errorf("Expected title 'Custom Title', got %q", settings["title"])
	}
}
