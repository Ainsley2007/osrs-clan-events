package database

import (
	"context"
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
			start_time DATETIME,
			end_time DATETIME,
			is_active BOOLEAN DEFAULT 1,
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

// Guilds
func (s *SQLiteStore) SaveGuild(ctx context.Context, g *Guild) error {
	query := `INSERT INTO guilds (
		guild_id, log_channel_id, botw_category_id, botw_channel_id, botw_overall_channel_id,
		botw_msg_id, botw_overall_msg_id, sotw_category_id, sotw_channel_id, sotw_overall_channel_id,
		sotw_msg_id, sotw_overall_msg_id, interval_day, interval_time
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(guild_id) DO UPDATE SET
		log_channel_id = excluded.log_channel_id,
		botw_category_id = excluded.botw_category_id,
		botw_channel_id = excluded.botw_channel_id,
		botw_overall_channel_id = excluded.botw_overall_channel_id,
		botw_msg_id = excluded.botw_msg_id,
		botw_overall_msg_id = excluded.botw_overall_msg_id,
		sotw_category_id = excluded.sotw_category_id,
		sotw_channel_id = excluded.sotw_channel_id,
		sotw_overall_channel_id = excluded.sotw_overall_channel_id,
		sotw_msg_id = excluded.sotw_msg_id,
		sotw_overall_msg_id = excluded.sotw_overall_msg_id,
		interval_day = excluded.interval_day,
		interval_time = excluded.interval_time;`

	_, err := s.db.ExecContext(ctx, query,
		g.GuildID, g.LogChannelID, g.BotwCategoryID, g.BotwChannelID, g.BotwOverallChannelID,
		g.BotwMsgID, g.BotwOverallMsgID, g.SotwCategoryID, g.SotwChannelID, g.SotwOverallChannelID,
		g.SotwMsgID, g.SotwOverallMsgID, g.IntervalDay, g.IntervalTime,
	)
	return err
}

func (s *SQLiteStore) GetGuild(ctx context.Context, guildID string) (*Guild, error) {
	query := `SELECT guild_id, log_channel_id, botw_category_id, botw_channel_id, botw_overall_channel_id,
		botw_msg_id, botw_overall_msg_id, sotw_category_id, sotw_channel_id, sotw_overall_channel_id,
		sotw_msg_id, sotw_overall_msg_id, interval_day, interval_time
		FROM guilds WHERE guild_id = ?`

	var g Guild
	err := s.db.QueryRowContext(ctx, query, guildID).Scan(
		&g.GuildID, &g.LogChannelID, &g.BotwCategoryID, &g.BotwChannelID, &g.BotwOverallChannelID,
		&g.BotwMsgID, &g.BotwOverallMsgID, &g.SotwCategoryID, &g.SotwChannelID, &g.SotwOverallChannelID,
		&g.SotwMsgID, &g.SotwOverallMsgID, &g.IntervalDay, &g.IntervalTime,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("guild not found: %s", guildID)
	}
	return &g, err
}

// Participants
func (s *SQLiteStore) SaveParticipant(ctx context.Context, p *Participant) error {
	query := `INSERT INTO participants (discord_user_id, guild_id, total_points_botw, total_points_sotw)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(discord_user_id, guild_id) DO UPDATE SET
		total_points_botw = excluded.total_points_botw,
		total_points_sotw = excluded.total_points_sotw;`

	_, err := s.db.ExecContext(ctx, query, p.DiscordUserID, p.GuildID, p.TotalPointsBotw, p.TotalPointsSotw)
	return err
}

func (s *SQLiteStore) GetParticipant(ctx context.Context, discordUserID, guildID string) (*Participant, error) {
	query := `SELECT discord_user_id, guild_id, total_points_botw, total_points_sotw
		FROM participants WHERE discord_user_id = ? AND guild_id = ?`

	var p Participant
	err := s.db.QueryRowContext(ctx, query, discordUserID, guildID).Scan(
		&p.DiscordUserID, &p.GuildID, &p.TotalPointsBotw, &p.TotalPointsSotw,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("participant not found")
	}
	return &p, err
}

func (s *SQLiteStore) DeleteParticipant(ctx context.Context, discordUserID, guildID string) error {
	query := `DELETE FROM participants WHERE discord_user_id = ? AND guild_id = ?`
	_, err := s.db.ExecContext(ctx, query, discordUserID, guildID)
	return err
}

// Accounts
func (s *SQLiteStore) SaveAccount(ctx context.Context, acc *Account) error {
	if acc.ID == 0 {
		query := `INSERT INTO accounts (rsn, discord_user_id, error_count, is_active) VALUES (?, ?, ?, ?)`
		res, err := s.db.ExecContext(ctx, query, acc.RSN, acc.DiscordUserID, acc.ErrorCount, acc.IsActive)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		acc.ID = id
		return nil
	}

	query := `UPDATE accounts SET rsn = ?, discord_user_id = ?, error_count = ?, is_active = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, acc.RSN, acc.DiscordUserID, acc.ErrorCount, acc.IsActive, acc.ID)
	return err
}

func (s *SQLiteStore) GetAccount(ctx context.Context, id int64) (*Account, error) {
	query := `SELECT id, rsn, discord_user_id, error_count, is_active FROM accounts WHERE id = ?`
	var acc Account
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&acc.ID, &acc.RSN, &acc.DiscordUserID, &acc.ErrorCount, &acc.IsActive,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("account not found")
	}
	return &acc, err
}

func (s *SQLiteStore) GetAccountsByDiscordID(ctx context.Context, discordUserID string) ([]*Account, error) {
	query := `SELECT id, rsn, discord_user_id, error_count, is_active FROM accounts WHERE discord_user_id = ?`
	rows, err := s.db.QueryContext(ctx, query, discordUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*Account
	for rows.Next() {
		var acc Account
		if err := rows.Scan(&acc.ID, &acc.RSN, &acc.DiscordUserID, &acc.ErrorCount, &acc.IsActive); err != nil {
			return nil, err
		}
		accounts = append(accounts, &acc)
	}
	return accounts, rows.Err()
}

func (s *SQLiteStore) GetActiveAccounts(ctx context.Context) ([]*Account, error) {
	query := `SELECT id, rsn, discord_user_id, error_count, is_active FROM accounts WHERE is_active = 1`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*Account
	for rows.Next() {
		var acc Account
		if err := rows.Scan(&acc.ID, &acc.RSN, &acc.DiscordUserID, &acc.ErrorCount, &acc.IsActive); err != nil {
			return nil, err
		}
		accounts = append(accounts, &acc)
	}
	return accounts, rows.Err()
}

func (s *SQLiteStore) GetAccountByRSN(ctx context.Context, rsn, discordUserID string) (*Account, error) {
	query := `SELECT id, rsn, discord_user_id, error_count, is_active 
		FROM accounts WHERE LOWER(rsn) = LOWER(?) AND discord_user_id = ?`

	var acc Account
	err := s.db.QueryRowContext(ctx, query, rsn, discordUserID).Scan(
		&acc.ID, &acc.RSN, &acc.DiscordUserID, &acc.ErrorCount, &acc.IsActive,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("account not found")
	}
	return &acc, err
}

func (s *SQLiteStore) DeleteAccount(ctx context.Context, id int64) error {
	query := `DELETE FROM accounts WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

func (s *SQLiteStore) UpdateAccountRSN(ctx context.Context, id int64, newRSN string) error {
	query := `UPDATE accounts SET rsn = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, newRSN, id)
	return err
}

// Events
func (s *SQLiteStore) SaveEvent(ctx context.Context, e *Event) error {
	if e.ID == 0 {
		query := `INSERT INTO events (guild_id, type, week_number, metric_json_id, start_time, end_time, is_active)
			VALUES (?, ?, ?, ?, ?, ?, ?)`
		res, err := s.db.ExecContext(ctx, query, e.GuildID, e.Type, e.WeekNumber, e.MetricJsonID, e.StartTime, e.EndTime, e.IsActive)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		e.ID = id
		return nil
	}

	query := `UPDATE events SET guild_id = ?, type = ?, week_number = ?, metric_json_id = ?, start_time = ?, end_time = ?, is_active = ?
		WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, e.GuildID, e.Type, e.WeekNumber, e.MetricJsonID, e.StartTime, e.EndTime, e.IsActive, e.ID)
	return err
}

func (s *SQLiteStore) GetEvent(ctx context.Context, id int64) (*Event, error) {
	query := `SELECT id, guild_id, type, week_number, metric_json_id, start_time, end_time, is_active FROM events WHERE id = ?`
	var e Event
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&e.ID, &e.GuildID, &e.Type, &e.WeekNumber, &e.MetricJsonID, &e.StartTime, &e.EndTime, &e.IsActive,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("event not found")
	}
	return &e, err
}

func (s *SQLiteStore) GetActiveEvent(ctx context.Context, guildID string, eventType string) (*Event, error) {
	query := `SELECT id, guild_id, type, week_number, metric_json_id, start_time, end_time, is_active
		FROM events WHERE guild_id = ? AND type = ? AND is_active = 1 ORDER BY start_time DESC LIMIT 1`

	var e Event
	err := s.db.QueryRowContext(ctx, query, guildID, eventType).Scan(
		&e.ID, &e.GuildID, &e.Type, &e.WeekNumber, &e.MetricJsonID, &e.StartTime, &e.EndTime, &e.IsActive,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no active event found")
	}
	return &e, err
}

// Snapshots
func (s *SQLiteStore) SaveSnapshot(ctx context.Context, snap *Snapshot) error {
	if snap.ID == 0 {
		query := `INSERT INTO snapshots (event_id, account_id, start_value, current_value) VALUES (?, ?, ?, ?)`
		res, err := s.db.ExecContext(ctx, query, snap.EventID, snap.AccountID, snap.StartValue, snap.CurrentValue)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		snap.ID = id
		return nil
	}

	query := `UPDATE snapshots SET event_id = ?, account_id = ?, start_value = ?, current_value = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, snap.EventID, snap.AccountID, snap.StartValue, snap.CurrentValue, snap.ID)
	return err
}

func (s *SQLiteStore) GetSnapshot(ctx context.Context, eventID, accountID int64) (*Snapshot, error) {
	query := `SELECT id, event_id, account_id, start_value, current_value 
		FROM snapshots WHERE event_id = ? AND account_id = ?`

	var snap Snapshot
	err := s.db.QueryRowContext(ctx, query, eventID, accountID).Scan(
		&snap.ID, &snap.EventID, &snap.AccountID, &snap.StartValue, &snap.CurrentValue,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("snapshot not found")
	}
	return &snap, err
}

func (s *SQLiteStore) GetSnapshotsByEvent(ctx context.Context, eventID int64) ([]*Snapshot, error) {
	query := `SELECT id, event_id, account_id, start_value, current_value FROM snapshots WHERE event_id = ?`
	rows, err := s.db.QueryContext(ctx, query, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []*Snapshot
	for rows.Next() {
		var snap Snapshot
		if err := rows.Scan(&snap.ID, &snap.EventID, &snap.AccountID, &snap.StartValue, &snap.CurrentValue); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, &snap)
	}
	return snapshots, rows.Err()
}

func (s *SQLiteStore) Close() error {
	log.Println("Closing database connection...")
	return s.db.Close()
}
