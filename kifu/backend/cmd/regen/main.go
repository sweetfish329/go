package main

import (
	"bytes"
	"database/sql"
	"image/png"
	"log"
	"os"

	"github.com/sweetfish329/go/kifu/backend/internal/imgutil"
	"github.com/sweetfish329/go/kifu/backend/internal/sgf"
	"github.com/sweetfish329/go/kifu/backend/internal/sgfcompress"
	_ "modernc.org/sqlite"
)

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "/data/kifu.db"
	}

	log.Printf("Opening database at %s...", dbPath)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, title, sgf_data FROM kifus")
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	type Item struct {
		id     string
		title  string
		rawSgf []byte
	}
	var items []Item

	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.id, &it.title, &it.rawSgf); err != nil {
			log.Fatalf("Scan failed: %v", err)
		}
		items = append(items, it)
	}

	log.Printf("Found %d kifus to regenerate.", len(items))

	updateStmt, err := db.Prepare("UPDATE kifus SET thumbnail_image = ?, ogp_image = ? WHERE id = ?")
	if err != nil {
		log.Fatalf("Prepare update failed: %v", err)
	}
	defer updateStmt.Close()

	var successCount, errorCount int
	for i, it := range items {
		sgfStr, err := sgfcompress.Decompress(it.rawSgf)
		if err != nil {
			log.Printf("[%d/%d] Error decompressing SGF for %s (%s): %v", i+1, len(items), it.id, it.title, err)
			errorCount++
			continue
		}

		rootNode, err := sgf.Parse(sgfStr)
		if err != nil {
			log.Printf("[%d/%d] Error parsing SGF for %s (%s): %v", i+1, len(items), it.id, it.title, err)
			errorCount++
			continue
		}

		meta := sgf.ExtractMetadata(rootNode)
		lastNode := rootNode.GetEnd()
		board := lastNode.Board()

		// 1. Generate Square Thumbnail (600x600 Flat WebP)
		thumbImg := sgf.GenerateSquareThumbnail(board, rootNode, meta.Size)
		thumbBytes, err := imgutil.EncodeToWebP(thumbImg)
		if err != nil {
			log.Printf("[%d/%d] Error encoding WebP thumbnail for %s: %v", i+1, len(items), it.id, err)
			errorCount++
			continue
		}

		// 2. Generate OGP Image (1200x630 Washi Clay PNG)
		ogpImg := sgf.GenerateBoardImage(board, rootNode, meta.Size)
		var ogpBuf bytes.Buffer
		if err := png.Encode(&ogpBuf, ogpImg); err != nil {
			log.Printf("[%d/%d] Error encoding PNG OGP for %s: %v", i+1, len(items), it.id, err)
			errorCount++
			continue
		}
		ogpBytes := ogpBuf.Bytes()

		// 3. Update DB
		_, err = updateStmt.Exec(thumbBytes, ogpBytes, it.id)
		if err != nil {
			log.Printf("[%d/%d] Failed to update DB for %s: %v", i+1, len(items), it.id, err)
			errorCount++
			continue
		}

		log.Printf("[%d/%d] Regenerated: ID=%s, Title='%s' (Thumbnail: %d bytes, OGP: %d bytes)",
			i+1, len(items), it.id, it.title, len(thumbBytes), len(ogpBytes))
		successCount++
	}

	log.Printf("Regeneration complete: %d succeeded, %d failed", successCount, errorCount)
}
