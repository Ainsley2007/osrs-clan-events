package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (s *SQLiteStore) UpsertMissingAccountNotificationFailure(ctx context.Context, notification *MissingAccountNotification) error {
	updateQuery := `
		UPDATE missing_account_notifications
		SET last_failed_at = ?, rsn = ?, discord_user_id = ?
		WHERE account_id = ? AND guild_id = ? AND resolved_at IS NULL`

	res, err := s.db.ExecContext(
		ctx,
		updateQuery,
		notification.LastFailedAt,
		notification.RSN,
		notification.DiscordUserID,
		notification.AccountID,
		notification.GuildID,
	)
	if err != nil {
		return fmt.Errorf("failed to update missing account notification: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read update rows affected: %w", err)
	}
	if rows > 0 {
		return nil
	}

	insertQuery := `
		INSERT INTO missing_account_notifications (
			account_id, discord_user_id, guild_id, rsn, first_failed_at, last_failed_at
		) VALUES (?, ?, ?, ?, ?, ?)`

	_, err = s.db.ExecContext(
		ctx,
		insertQuery,
		notification.AccountID,
		notification.DiscordUserID,
		notification.GuildID,
		notification.RSN,
		notification.FirstFailedAt,
		notification.LastFailedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert missing account notification: %w", err)
	}

	return nil
}

func (s *SQLiteStore) GetPendingMissingAccountNotifications(ctx context.Context) ([]*MissingAccountNotification, error) {
	query := `
		SELECT id, account_id, discord_user_id, guild_id, rsn, first_failed_at, last_failed_at, dm_sent_at, resolved_at
		FROM missing_account_notifications
		WHERE resolved_at IS NULL AND dm_sent_at IS NULL
		ORDER BY first_failed_at ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending missing account notifications: %w", err)
	}
	defer rows.Close()

	var result []*MissingAccountNotification
	for rows.Next() {
		notification, err := scanMissingAccountNotification(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, notification)
	}

	return result, rows.Err()
}

func (s *SQLiteStore) MarkMissingAccountNotificationDMSent(ctx context.Context, notificationID int64, sentAt time.Time) error {
	query := `
		UPDATE missing_account_notifications
		SET dm_sent_at = ?
		WHERE id = ?`
	if _, err := s.db.ExecContext(ctx, query, sentAt, notificationID); err != nil {
		return fmt.Errorf("failed to mark missing account notification DM sent: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ResolveMissingAccountNotification(ctx context.Context, accountID int64, guildID string, resolvedAt time.Time) error {
	query := `
		UPDATE missing_account_notifications
		SET resolved_at = ?
		WHERE account_id = ? AND guild_id = ? AND resolved_at IS NULL`
	if _, err := s.db.ExecContext(ctx, query, resolvedAt, accountID, guildID); err != nil {
		return fmt.Errorf("failed to resolve missing account notification: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetUnresolvedMissingAccountNotificationsByGuild(ctx context.Context, guildID string) ([]*MissingAccountNotification, error) {
	query := `
		SELECT id, account_id, discord_user_id, guild_id, rsn, first_failed_at, last_failed_at, dm_sent_at, resolved_at
		FROM missing_account_notifications
		WHERE guild_id = ? AND resolved_at IS NULL
		ORDER BY last_failed_at DESC`

	rows, err := s.db.QueryContext(ctx, query, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to query unresolved missing account notifications: %w", err)
	}
	defer rows.Close()

	var result []*MissingAccountNotification
	for rows.Next() {
		notification, err := scanMissingAccountNotification(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, notification)
	}

	return result, rows.Err()
}

func (s *SQLiteStore) ShouldSendMissingAccountWeeklySummary(ctx context.Context, guildID, weekKey string) (bool, error) {
	var lastSentWeek string
	query := `SELECT last_sent_week FROM missing_account_weekly_summaries WHERE guild_id = ?`
	err := s.db.QueryRowContext(ctx, query, guildID).Scan(&lastSentWeek)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to read weekly summary state: %w", err)
	}
	return lastSentWeek != weekKey, nil
}

func (s *SQLiteStore) MarkMissingAccountWeeklySummarySent(ctx context.Context, guildID, weekKey string, sentAt time.Time) error {
	query := `
		INSERT INTO missing_account_weekly_summaries(guild_id, last_sent_week, last_sent_at)
		VALUES (?, ?, ?)
		ON CONFLICT(guild_id) DO UPDATE SET
			last_sent_week = excluded.last_sent_week,
			last_sent_at = excluded.last_sent_at`
	if _, err := s.db.ExecContext(ctx, query, guildID, weekKey, sentAt); err != nil {
		return fmt.Errorf("failed to mark weekly summary sent: %w", err)
	}
	return nil
}

func scanMissingAccountNotification(scanner interface {
	Scan(dest ...any) error
}) (*MissingAccountNotification, error) {
	var notification MissingAccountNotification
	var dmSentAt sql.NullTime
	var resolvedAt sql.NullTime

	if err := scanner.Scan(
		&notification.ID,
		&notification.AccountID,
		&notification.DiscordUserID,
		&notification.GuildID,
		&notification.RSN,
		&notification.FirstFailedAt,
		&notification.LastFailedAt,
		&dmSentAt,
		&resolvedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan missing account notification: %w", err)
	}

	if dmSentAt.Valid {
		t := dmSentAt.Time
		notification.DMSentAt = &t
	}
	if resolvedAt.Valid {
		t := resolvedAt.Time
		notification.ResolvedAt = &t
	}

	return &notification, nil
}
