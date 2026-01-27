package database

import (
	"context"
	"database/sql"
	"fmt"
)

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
