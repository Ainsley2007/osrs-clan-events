package database

import (
	"context"
	"path/filepath"
	"testing"

	"database/sql"

	_ "modernc.org/sqlite"
)

func TestNewSQLiteStore_MigratesLegacyPBCategoriesSchema(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	legacyDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}

	_, err = legacyDB.Exec(`
		CREATE TABLE pb_categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT 1,
			embed_image_url TEXT NOT NULL DEFAULT ''
		)`)
	if err != nil {
		t.Fatalf("create legacy pb_categories: %v", err)
	}
	_, err = legacyDB.Exec(`
		INSERT INTO pb_categories (slug, display_name, is_active, embed_image_url)
		VALUES ('inferno', 'The Inferno', 1, 'https://example.com/inferno.png')`)
	if err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore on legacy schema: %v", err)
	}
	defer store.Close()

	categories, err := store.GetActivePBCategories(context.Background())
	if err != nil {
		t.Fatalf("GetActivePBCategories: %v", err)
	}
	if len(categories) == 0 {
		t.Fatalf("expected seeded categories after migration")
	}
	if categories[0].GroupName != "Minigames" {
		t.Fatalf("expected Minigames group, got %q", categories[0].GroupName)
	}
}
