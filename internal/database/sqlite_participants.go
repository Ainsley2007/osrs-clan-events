package database

import (
	"context"
	"database/sql"
	"fmt"
)

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

func (s *SQLiteStore) GetParticipantsByGuild(ctx context.Context, guildID string) ([]*Participant, error) {
	query := `SELECT discord_user_id, guild_id, total_points_botw, total_points_sotw
		FROM participants WHERE guild_id = ?`
	rows, err := s.db.QueryContext(ctx, query, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var participants []*Participant
	for rows.Next() {
		var p Participant
		if err := rows.Scan(&p.DiscordUserID, &p.GuildID, &p.TotalPointsBotw, &p.TotalPointsSotw); err != nil {
			return nil, err
		}
		participants = append(participants, &p)
	}
	return participants, rows.Err()
}

func (s *SQLiteStore) DeleteParticipant(ctx context.Context, discordUserID, guildID string) error {
	query := `DELETE FROM participants WHERE discord_user_id = ? AND guild_id = ?`
	_, err := s.db.ExecContext(ctx, query, discordUserID, guildID)
	return err
}

func (s *SQLiteStore) GetTotalGainedByParticipant(ctx context.Context, guildID, eventType string) (map[string]int64, error) {
	query := `SELECT p.discord_user_id, SUM(s.current_value - s.start_value) AS total_gained
		FROM participants p
		INNER JOIN accounts a ON a.discord_user_id = p.discord_user_id
		INNER JOIN snapshots s ON s.account_id = a.id
		INNER JOIN events e ON s.event_id = e.id AND e.guild_id = p.guild_id AND e.type = ? AND e.is_active = 0
		WHERE p.guild_id = ?
		GROUP BY p.discord_user_id`
	rows, err := s.db.QueryContext(ctx, query, eventType, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]int64)
	for rows.Next() {
		var discordUserID string
		var totalGained int64
		if err := rows.Scan(&discordUserID, &totalGained); err != nil {
			return nil, err
		}
		result[discordUserID] = totalGained
	}
	return result, rows.Err()
}
