package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

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

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(0)

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
	if _, err := s.db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		return fmt.Errorf("failed to enable WAL mode: %w", err)
	}
	if _, err := s.db.Exec("PRAGMA busy_timeout = 5000;"); err != nil {
		return fmt.Errorf("failed to set busy timeout: %w", err)
	}
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
			pb_category_id TEXT,
			pb_leaderboard_channel_id TEXT,
			pb_proofs_channel_id TEXT,
			donation_channel_id TEXT,
			donation_msg_id TEXT,
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
		`CREATE TABLE IF NOT EXISTS missing_account_notifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account_id INTEGER NOT NULL,
			discord_user_id TEXT NOT NULL,
			guild_id TEXT NOT NULL,
			rsn TEXT NOT NULL,
			first_failed_at DATETIME NOT NULL,
			last_failed_at DATETIME NOT NULL,
			dm_sent_at DATETIME,
			resolved_at DATETIME,
			FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE,
			FOREIGN KEY (guild_id) REFERENCES guilds(guild_id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS missing_account_weekly_summaries (
			guild_id TEXT PRIMARY KEY,
			last_sent_week TEXT NOT NULL,
			last_sent_at DATETIME NOT NULL,
			FOREIGN KEY (guild_id) REFERENCES guilds(guild_id) ON DELETE CASCADE
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_missing_account_unresolved
			ON missing_account_notifications(account_id, guild_id)
			WHERE resolved_at IS NULL;`,
		`CREATE INDEX IF NOT EXISTS idx_missing_account_notifications_guild_resolved
			ON missing_account_notifications(guild_id, resolved_at);`,
		`CREATE INDEX IF NOT EXISTS idx_missing_account_notifications_pending_dm
			ON missing_account_notifications(dm_sent_at, resolved_at);`,
		`CREATE TABLE IF NOT EXISTS pb_categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL,
			group_name TEXT NOT NULL DEFAULT 'Minigames',
			group_order INTEGER NOT NULL DEFAULT 0,
			display_order INTEGER NOT NULL DEFAULT 0,
			is_active BOOLEAN NOT NULL DEFAULT 1,
			embed_image_url TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE TABLE IF NOT EXISTS pb_submissions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			guild_id TEXT NOT NULL,
			category_slug TEXT NOT NULL,
			discord_user_id TEXT NOT NULL,
			display_name TEXT NOT NULL,
			time_text TEXT,
			time_centiseconds INTEGER,
			proof_url TEXT NOT NULL,
			proof_message_id TEXT,
			status TEXT NOT NULL,
			reviewed_by_discord_id TEXT,
			reviewed_at DATETIME,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			FOREIGN KEY (guild_id) REFERENCES guilds(guild_id) ON DELETE CASCADE,
			FOREIGN KEY (category_slug) REFERENCES pb_categories(slug) ON DELETE RESTRICT
		);`,
		`CREATE TABLE IF NOT EXISTS pb_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			guild_id TEXT NOT NULL,
			category_slug TEXT NOT NULL,
			discord_user_id TEXT NOT NULL,
			display_name TEXT NOT NULL,
			time_text TEXT NOT NULL,
			time_centiseconds INTEGER NOT NULL,
			proof_submission_id INTEGER NOT NULL,
			proof_url TEXT NOT NULL,
			updated_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL,
			FOREIGN KEY (guild_id) REFERENCES guilds(guild_id) ON DELETE CASCADE,
			FOREIGN KEY (category_slug) REFERENCES pb_categories(slug) ON DELETE RESTRICT,
			FOREIGN KEY (proof_submission_id) REFERENCES pb_submissions(id) ON DELETE RESTRICT
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_pb_records_unique_user_category
			ON pb_records(guild_id, category_slug, discord_user_id);`,
		`CREATE TABLE IF NOT EXISTS pb_leaderboard_messages (
			guild_id TEXT NOT NULL,
			category_slug TEXT NOT NULL,
			channel_id TEXT NOT NULL,
			message_id TEXT NOT NULL,
			updated_at DATETIME NOT NULL,
			PRIMARY KEY (guild_id, category_slug),
			FOREIGN KEY (guild_id) REFERENCES guilds(guild_id) ON DELETE CASCADE,
			FOREIGN KEY (category_slug) REFERENCES pb_categories(slug) ON DELETE RESTRICT
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_pb_submissions_proof_message
			ON pb_submissions(guild_id, proof_message_id)
			WHERE proof_message_id IS NOT NULL;`,
		`CREATE TABLE IF NOT EXISTS pb_group_bundle_messages (
			guild_id TEXT NOT NULL,
			group_name TEXT NOT NULL,
			channel_id TEXT NOT NULL,
			message_id TEXT NOT NULL,
			updated_at DATETIME NOT NULL,
			PRIMARY KEY (guild_id, group_name),
			FOREIGN KEY (guild_id) REFERENCES guilds(guild_id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_pb_submissions_pending_message
			ON pb_submissions(guild_id, status, proof_message_id);`,
		`CREATE TABLE IF NOT EXISTS donations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			guild_id TEXT NOT NULL,
			discord_user_id TEXT NOT NULL,
			amount INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			created_by TEXT NOT NULL,
			FOREIGN KEY (guild_id) REFERENCES guilds(guild_id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS donation_spending (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			guild_id TEXT NOT NULL,
			amount INTEGER NOT NULL,
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			created_by TEXT NOT NULL,
			FOREIGN KEY (guild_id) REFERENCES guilds(guild_id) ON DELETE CASCADE
		);`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query %q: %w", query, err)
		}
	}
	if err := s.runColumnMigrations(); err != nil {
		return err
	}
	if err := s.migratePBCategoryGroupingColumns(); err != nil {
		return fmt.Errorf("failed to migrate pb category grouping columns: %w", err)
	}
	if err := s.seedPBCategories(); err != nil {
		return err
	}
	s.ensureUniqueActiveEventIndex()
	return nil
}

func (s *SQLiteStore) runColumnMigrations() error {
	migrations := []string{
		`ALTER TABLE guilds ADD COLUMN donation_channel_id TEXT;`,
		`ALTER TABLE guilds ADD COLUMN donation_msg_id TEXT;`,
		`ALTER TABLE guilds ADD COLUMN pb_category_id TEXT;`,
		`ALTER TABLE guilds ADD COLUMN pb_leaderboard_channel_id TEXT;`,
		`ALTER TABLE guilds ADD COLUMN pb_proofs_channel_id TEXT;`,
	}
	for _, q := range migrations {
		if _, err := s.db.Exec(q); err != nil && !isDuplicateColumnErr(err) {
			log.Printf("Warning: migration query failed (may be expected): %v", err)
		}
	}
	return nil
}

func isDuplicateColumnErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column")
}

func (s *SQLiteStore) hasTableColumn(table, column string) (bool, error) {
	rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, fmt.Errorf("pragma table_info(%s): %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return false, fmt.Errorf("scan table_info(%s): %w", table, err)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func (s *SQLiteStore) migratePBCategoryGroupingColumns() error {
	migrations := []struct {
		column string
		ddl    string
	}{
		{column: "group_name", ddl: `ALTER TABLE pb_categories ADD COLUMN group_name TEXT DEFAULT 'Minigames'`},
		{column: "group_order", ddl: `ALTER TABLE pb_categories ADD COLUMN group_order INTEGER DEFAULT 0`},
		{column: "display_order", ddl: `ALTER TABLE pb_categories ADD COLUMN display_order INTEGER DEFAULT 0`},
	}

	for _, migration := range migrations {
		has, err := s.hasTableColumn("pb_categories", migration.column)
		if err != nil {
			return err
		}
		if has {
			continue
		}

		if _, err := s.db.Exec(migration.ddl); err != nil && !isDuplicateColumnErr(err) {
			return fmt.Errorf("add pb_categories.%s: %w", migration.column, err)
		}

		has, err = s.hasTableColumn("pb_categories", migration.column)
		if err != nil {
			return err
		}
		if !has {
			return fmt.Errorf("pb_categories.%s still missing after migration", migration.column)
		}
	}

	return nil
}

func (s *SQLiteStore) seedPBCategories() error {
	query := `
		INSERT INTO pb_categories (slug, display_name, group_name, group_order, display_order, is_active, embed_image_url)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(slug) DO UPDATE SET
			display_name = excluded.display_name,
			group_name = excluded.group_name,
			group_order = excluded.group_order,
			display_order = excluded.display_order,
			is_active = excluded.is_active,
			embed_image_url = excluded.embed_image_url`

	seeds := []struct {
		slug         string
		displayName  string
		groupName    string
		groupOrder   int
		displayOrder int
		imageURL     string
	}{
		{
			slug:         "inferno",
			displayName:  "The Inferno",
			groupName:    "Minigames",
			groupOrder:   1,
			displayOrder: 1,
			imageURL:     "https://images-ext-1.discordapp.net/external/X8UMtpv21fsvGtd_bu-JO1YOJNZgd5fxnmwlCMxWuvg/%3F2d222/https/oldschool.runescape.wiki/images/thumb/TzKal-Zuk.png/138px-TzKal-Zuk.png?format=webp&quality=lossless&width=276&height=300",
		},
		{
			slug:         "fortis_colosseum",
			displayName:  "Fortis Colosseum",
			groupName:    "Minigames",
			groupOrder:   1,
			displayOrder: 2,
			imageURL:     "https://images-ext-1.discordapp.net/external/CGjkbhxg5A4Mnqts0bl_dU3KuhjdN14K5ZgBzdQ-6CY/%3F91250/https/oldschool.runescape.wiki/images/thumb/Sol_Heredit.png/104px-Sol_Heredit.png?format=webp&quality=lossless&width=208&height=300",
		},
		{
			slug:         "fight_caves",
			displayName:  "Fight Caves",
			groupName:    "Minigames",
			groupOrder:   1,
			displayOrder: 3,
			imageURL:     "https://images-ext-1.discordapp.net/external/5DpjS_0B3SGLKRmBZeR8Yn_PJIvXNtcI-yhqN_r11_k/%3F87507/https/oldschool.runescape.wiki/images/thumb/TzTok-Jad.png/135px-TzTok-Jad.png?format=webp&quality=lossless&width=270&height=296",
		},
	}

	for _, seed := range seeds {
		if _, err := s.db.Exec(query, seed.slug, seed.displayName, seed.groupName, seed.groupOrder, seed.displayOrder, true, seed.imageURL); err != nil {
			return fmt.Errorf("failed to seed pb category %s: %w", seed.slug, err)
		}
	}
	return nil
}

func (s *SQLiteStore) ensureUniqueActiveEventIndex() {
	q := `CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_active_event ON events(guild_id, type, is_active) WHERE is_active = 1;`
	if _, err := s.db.Exec(q); err != nil {
		log.Printf("Warning: failed to create unique active event index: %v", err)
	}
}

func (s *SQLiteStore) Close() error {
	log.Println("Closing database connection...")
	return s.db.Close()
}
