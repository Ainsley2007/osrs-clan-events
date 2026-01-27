package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.init(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return store, nil
}

func (s *SQLiteStore) init() error {
	// Enable foreign keys
	if _, err := s.db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	queries := []string{
		`CREATE TABLE IF NOT EXISTS guilds (
			guild_id TEXT PRIMARY KEY,
			log_channel_id TEXT,
			botw_category_id TEXT,
			botw_channel_id TEXT,
			botw_overall_channel_id TEXT,
			botw_msg_id TEXT,
			botw_overall_msg_id TEXT,
			sotw_category_id TEXT,
			sotw_channel_id TEXT,
			sotw_overall_channel_id TEXT,
			sotw_msg_id TEXT,
			sotw_overall_msg_id TEXT,
			interval_day TEXT DEFAULT 'Sunday',
			interval_time TEXT DEFAULT '22:00'
		);`,
		`CREATE TABLE IF NOT EXISTS participants (
			discord_user_id TEXT,
			guild_id TEXT,
			total_points_botw INTEGER DEFAULT 0,
			total_points_sotw INTEGER DEFAULT 0,
			PRIMARY KEY (discord_user_id, guild_id),
			FOREIGN KEY (guild_id) REFERENCES guilds(guild_id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			rsn TEXT NOT NULL,
			discord_user_id TEXT NOT NULL,
			error_count INTEGER DEFAULT 0,
			is_active BOOLEAN DEFAULT 1
		);`,
		`CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			guild_id TEXT,
			type TEXT,
			week_number INTEGER,
			metric_json_id TEXT,
			bosses_to_track TEXT,
			start_time DATETIME,
			end_time DATETIME,
			is_active BOOLEAN DEFAULT 1,
			points_per_kc REAL,
			points_per_xp REAL,
			threshold_kc INTEGER,
			xp_threshold INTEGER,
			FOREIGN KEY (guild_id) REFERENCES guilds(guild_id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			event_id INTEGER,
			account_id INTEGER,
			start_value INTEGER,
			current_value INTEGER,
			FOREIGN KEY (event_id) REFERENCES events(id) ON DELETE CASCADE,
			FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
		);`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query %q: %w", query, err)
		}
	}
	return nil
}

func (s *SQLiteStore) Close() error {
	log.Println("Closing database connection...")
	return s.db.Close()
}
