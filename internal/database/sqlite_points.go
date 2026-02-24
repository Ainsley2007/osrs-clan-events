package database

import (
	"context"
	"database/sql"
	"fmt"
)

func (s *SQLiteStore) UpdateParticipantPoints(ctx context.Context, updates []*ParticipantPointUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `UPDATE participants SET total_points_botw = total_points_botw + ?, total_points_sotw = total_points_sotw + ? WHERE discord_user_id = ? AND guild_id = ?`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, update := range updates {
		if _, err := stmt.ExecContext(ctx, update.BotwPoints, update.SotwPoints, update.DiscordUserID, update.GuildID); err != nil {
			return fmt.Errorf("failed to update participant %s: %w", update.DiscordUserID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
