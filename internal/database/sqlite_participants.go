package database

import (
	"context"
	"database/sql"
	"fmt"
)

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
