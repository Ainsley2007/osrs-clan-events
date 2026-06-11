package database

import (
	"context"
	"database/sql"
	"fmt"
)

func (s *SQLiteStore) SaveGuild(ctx context.Context, g *Guild) error {
	query := `INSERT INTO guilds (
		guild_id, log_channel_id, botw_category_id, botw_channel_id, botw_overall_channel_id,
		botw_msg_id, botw_overall_msg_id, sotw_category_id, sotw_channel_id, sotw_overall_channel_id,
		sotw_msg_id, sotw_overall_msg_id, pb_category_id, pb_leaderboard_channel_id, pb_proofs_channel_id,
		donation_channel_id, donation_msg_id, interval_day, interval_time
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		pb_category_id = excluded.pb_category_id,
		pb_leaderboard_channel_id = excluded.pb_leaderboard_channel_id,
		pb_proofs_channel_id = excluded.pb_proofs_channel_id,
		donation_channel_id = excluded.donation_channel_id,
		donation_msg_id = excluded.donation_msg_id,
		interval_day = excluded.interval_day,
		interval_time = excluded.interval_time;`

	_, err := s.db.ExecContext(ctx, query,
		g.GuildID, g.LogChannelID, g.BotwCategoryID, g.BotwChannelID, g.BotwOverallChannelID,
		g.BotwMsgID, g.BotwOverallMsgID, g.SotwCategoryID, g.SotwChannelID, g.SotwOverallChannelID,
		g.SotwMsgID, g.SotwOverallMsgID, g.PbCategoryID, g.PbLeaderboardChannelID, g.PbProofsChannelID,
		g.DonationChannelID, g.DonationMsgID, g.IntervalDay, g.IntervalTime,
	)
	return err
}

func (s *SQLiteStore) GetGuild(ctx context.Context, guildID string) (*Guild, error) {
	query := `SELECT guild_id, log_channel_id, botw_category_id, botw_channel_id, botw_overall_channel_id,
		botw_msg_id, botw_overall_msg_id, sotw_category_id, sotw_channel_id, sotw_overall_channel_id,
		sotw_msg_id, sotw_overall_msg_id, COALESCE(pb_category_id, '') as pb_category_id,
		COALESCE(pb_leaderboard_channel_id, '') as pb_leaderboard_channel_id,
		COALESCE(pb_proofs_channel_id, '') as pb_proofs_channel_id,
		COALESCE(donation_channel_id, '') as donation_channel_id, 
		COALESCE(donation_msg_id, '') as donation_msg_id, interval_day, interval_time
		FROM guilds WHERE guild_id = ?`

	var g Guild
	err := s.db.QueryRowContext(ctx, query, guildID).Scan(
		&g.GuildID, &g.LogChannelID, &g.BotwCategoryID, &g.BotwChannelID, &g.BotwOverallChannelID,
		&g.BotwMsgID, &g.BotwOverallMsgID, &g.SotwCategoryID, &g.SotwChannelID, &g.SotwOverallChannelID,
		&g.SotwMsgID, &g.SotwOverallMsgID, &g.PbCategoryID, &g.PbLeaderboardChannelID, &g.PbProofsChannelID,
		&g.DonationChannelID, &g.DonationMsgID, &g.IntervalDay, &g.IntervalTime,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("guild not found: %s", guildID)
	}
	return &g, err
}

func (s *SQLiteStore) DeleteGuild(ctx context.Context, guildID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := deleteGuildData(ctx, tx, guildID); err != nil {
		return err
	}
	return tx.Commit()
}

func deleteGuildData(ctx context.Context, tx *sql.Tx, guildID string) error {
	queries := []string{
		`DELETE FROM snapshots WHERE event_id IN (SELECT id FROM events WHERE guild_id = ?)`,
		`DELETE FROM events WHERE guild_id = ?`,
		`DELETE FROM missing_account_notifications WHERE guild_id = ?`,
		`DELETE FROM missing_account_weekly_summaries WHERE guild_id = ?`,
		`DELETE FROM pb_records WHERE guild_id = ?`,
		`DELETE FROM pb_submissions WHERE guild_id = ?`,
		`DELETE FROM pb_group_bundle_messages WHERE guild_id = ?`,
		`DELETE FROM donations WHERE guild_id = ?`,
		`DELETE FROM donation_spending WHERE guild_id = ?`,
		`DELETE FROM participants WHERE guild_id = ?`,
		`DELETE FROM guilds WHERE guild_id = ?`,
	}
	for _, q := range queries {
		if _, err := tx.ExecContext(ctx, q, guildID); err != nil {
			return fmt.Errorf("delete guild %s: %w", guildID, err)
		}
	}
	return nil
}

func (s *SQLiteStore) ListGuildIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT guild_id FROM guilds`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *SQLiteStore) PurgeOrphanedEvents(ctx context.Context) (int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT e.guild_id FROM events e
		LEFT JOIN guilds g ON e.guild_id = g.guild_id
		WHERE g.guild_id IS NULL`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var guildIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		guildIDs = append(guildIDs, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	purged := 0
	for _, guildID := range guildIDs {
		if err := s.DeleteGuild(ctx, guildID); err != nil {
			return purged, err
		}
		purged++
	}
	return purged, nil
}
