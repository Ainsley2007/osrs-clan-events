package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	pbSubmissionStatusPending  = "pending"
	pbSubmissionStatusAccepted = "accepted"
	pbSubmissionStatusRejected = "rejected"
)

func (s *SQLiteStore) GetActivePBCategories(ctx context.Context) ([]*PBCategory, error) {
	query := `
		SELECT id, slug, display_name, group_name, group_order, display_order, is_active, embed_image_url
		FROM pb_categories
		WHERE is_active = 1
		ORDER BY group_order ASC, display_order ASC, id ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active pb categories: %w", err)
	}
	defer rows.Close()

	var categories []*PBCategory
	for rows.Next() {
		var category PBCategory
		if err := rows.Scan(
			&category.ID,
			&category.Slug,
			&category.DisplayName,
			&category.GroupName,
			&category.GroupOrder,
			&category.DisplayOrder,
			&category.IsActive,
			&category.EmbedImageURL,
		); err != nil {
			return nil, fmt.Errorf("failed to scan pb category: %w", err)
		}
		categories = append(categories, &category)
	}
	return categories, rows.Err()
}

func (s *SQLiteStore) GetPBCategoryBySlug(ctx context.Context, slug string) (*PBCategory, error) {
	query := `
		SELECT id, slug, display_name, group_name, group_order, display_order, is_active, embed_image_url
		FROM pb_categories
		WHERE slug = ?`

	var category PBCategory
	if err := s.db.QueryRowContext(ctx, query, slug).Scan(
		&category.ID,
		&category.Slug,
		&category.DisplayName,
		&category.GroupName,
		&category.GroupOrder,
		&category.DisplayOrder,
		&category.IsActive,
		&category.EmbedImageURL,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("pb category not found")
		}
		return nil, fmt.Errorf("failed to load pb category: %w", err)
	}
	return &category, nil
}

func (s *SQLiteStore) CreatePBSubmission(ctx context.Context, submission *PBSubmission) error {
	query := `
		INSERT INTO pb_submissions (
			guild_id, category_slug, discord_user_id, display_name, leaderboard_display_name,
			time_text, time_centiseconds, proof_url, proof_message_id, status,
			reviewed_by_discord_id, reviewed_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var timeText any
	if submission.TimeText != nil {
		timeText = *submission.TimeText
	}
	var timeCentiseconds any
	if submission.TimeCentiseconds != nil {
		timeCentiseconds = *submission.TimeCentiseconds
	}
	var proofMessageID any
	if submission.ProofMessageID != nil {
		proofMessageID = *submission.ProofMessageID
	}
	var reviewedBy any
	if submission.ReviewedByDiscordID != nil {
		reviewedBy = *submission.ReviewedByDiscordID
	}
	var reviewedAt any
	if submission.ReviewedAt != nil {
		reviewedAt = *submission.ReviewedAt
	}

	res, err := s.db.ExecContext(
		ctx,
		query,
		submission.GuildID,
		submission.CategorySlug,
		submission.DiscordUserID,
		submission.DisplayName,
		submission.LeaderboardDisplayName,
		timeText,
		timeCentiseconds,
		submission.ProofURL,
		proofMessageID,
		submission.Status,
		reviewedBy,
		reviewedAt,
		submission.CreatedAt,
		submission.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create pb submission: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to read pb submission id: %w", err)
	}
	submission.ID = id
	return nil
}

func (s *SQLiteStore) UpdatePBSubmissionProofMessageID(ctx context.Context, submissionID int64, messageID string, updatedAt time.Time) error {
	query := `
		UPDATE pb_submissions
		SET proof_message_id = ?, updated_at = ?
		WHERE id = ?`
	if _, err := s.db.ExecContext(ctx, query, messageID, updatedAt, submissionID); err != nil {
		return fmt.Errorf("failed to update pb proof message id: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetPendingPBSubmissionByProofMessageID(ctx context.Context, guildID, proofMessageID string) (*PBSubmission, error) {
	query := `
		SELECT
			id, guild_id, category_slug, discord_user_id, display_name, leaderboard_display_name,
			time_text, time_centiseconds, proof_url, proof_message_id, status,
			reviewed_by_discord_id, reviewed_at, created_at, updated_at
		FROM pb_submissions
		WHERE guild_id = ? AND proof_message_id = ? AND status = ?`
	return s.getPBSubmissionWithArgs(ctx, query, guildID, proofMessageID, pbSubmissionStatusPending)
}

func (s *SQLiteStore) ApprovePBSubmission(ctx context.Context, submissionID int64, reviewerDiscordID string, reviewedAt time.Time) error {
	return s.updatePBSubmissionStatus(ctx, submissionID, pbSubmissionStatusAccepted, reviewerDiscordID, reviewedAt)
}

func (s *SQLiteStore) RejectPBSubmission(ctx context.Context, submissionID int64, reviewerDiscordID string, reviewedAt time.Time) error {
	return s.updatePBSubmissionStatus(ctx, submissionID, pbSubmissionStatusRejected, reviewerDiscordID, reviewedAt)
}

func (s *SQLiteStore) updatePBSubmissionStatus(ctx context.Context, submissionID int64, status, reviewerDiscordID string, reviewedAt time.Time) error {
	query := `
		UPDATE pb_submissions
		SET status = ?, reviewed_by_discord_id = ?, reviewed_at = ?, updated_at = ?
		WHERE id = ? AND status = ?`
	res, err := s.db.ExecContext(
		ctx,
		query,
		status,
		reviewerDiscordID,
		reviewedAt,
		reviewedAt,
		submissionID,
		pbSubmissionStatusPending,
	)
	if err != nil {
		return fmt.Errorf("failed to update pb submission status: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read pb submission update rows: %w", err)
	}
	if rows == 0 {
		return ErrPBSubmissionNotPending
	}
	return nil
}

func (s *SQLiteStore) UpsertPBRecordIfBetter(ctx context.Context, record *PBRecord) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("failed to begin pb record transaction: %w", err)
	}
	defer tx.Rollback()

	var existingID int64
	var existingTimeCS int64
	query := `
		SELECT id, time_centiseconds
		FROM pb_records
		WHERE guild_id = ? AND category_slug = ? AND discord_user_id = ?`
	err = tx.QueryRowContext(ctx, query, record.GuildID, record.CategorySlug, record.DiscordUserID).
		Scan(&existingID, &existingTimeCS)
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("failed to query existing pb record: %w", err)
	}

	if err == sql.ErrNoRows {
		insertQuery := `
			INSERT INTO pb_records (
				guild_id, category_slug, discord_user_id, display_name,
				time_text, time_centiseconds, proof_submission_id, proof_url, updated_at, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		_, err = tx.ExecContext(
			ctx,
			insertQuery,
			record.GuildID,
			record.CategorySlug,
			record.DiscordUserID,
			record.DisplayName,
			record.TimeText,
			record.TimeCentiseconds,
			record.ProofSubmissionID,
			record.ProofURL,
			record.UpdatedAt,
			record.CreatedAt,
		)
		if err != nil {
			return false, fmt.Errorf("failed to insert pb record: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("failed to commit inserted pb record: %w", err)
		}
		return true, nil
	}

	if existingTimeCS <= record.TimeCentiseconds {
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("failed to commit unchanged pb record: %w", err)
		}
		return false, nil
	}

	updateQuery := `
		UPDATE pb_records
		SET display_name = ?, time_text = ?, time_centiseconds = ?,
			proof_submission_id = ?, proof_url = ?, updated_at = ?
		WHERE id = ?`
	if _, err := tx.ExecContext(
		ctx,
		updateQuery,
		record.DisplayName,
		record.TimeText,
		record.TimeCentiseconds,
		record.ProofSubmissionID,
		record.ProofURL,
		record.UpdatedAt,
		existingID,
	); err != nil {
		return false, fmt.Errorf("failed to update pb record: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("failed to commit updated pb record: %w", err)
	}
	return true, nil
}

func (s *SQLiteStore) GetPBRecordsByCategory(ctx context.Context, guildID, categorySlug string) ([]*PBRecord, error) {
	query := `
		SELECT
			id, guild_id, category_slug, discord_user_id, display_name,
			time_text, time_centiseconds, proof_submission_id, proof_url, updated_at, created_at
		FROM pb_records
		WHERE guild_id = ? AND category_slug = ?
		ORDER BY time_centiseconds ASC, updated_at ASC`
	rows, err := s.db.QueryContext(ctx, query, guildID, categorySlug)
	if err != nil {
		return nil, fmt.Errorf("failed to query pb records: %w", err)
	}
	defer rows.Close()

	var records []*PBRecord
	for rows.Next() {
		var record PBRecord
		if err := rows.Scan(
			&record.ID,
			&record.GuildID,
			&record.CategorySlug,
			&record.DiscordUserID,
			&record.DisplayName,
			&record.TimeText,
			&record.TimeCentiseconds,
			&record.ProofSubmissionID,
			&record.ProofURL,
			&record.UpdatedAt,
			&record.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan pb record: %w", err)
		}
		records = append(records, &record)
	}
	return records, rows.Err()
}

func (s *SQLiteStore) GetPBGroupBundleMessage(ctx context.Context, guildID, groupName string) (*PBLeaderboardMessage, error) {
	query := `
		SELECT guild_id, group_name, channel_id, message_id, updated_at
		FROM pb_group_bundle_messages
		WHERE guild_id = ? AND group_name = ?`

	var message PBLeaderboardMessage
	if err := s.db.QueryRowContext(ctx, query, guildID, groupName).Scan(
		&message.GuildID,
		&message.GroupName,
		&message.ChannelID,
		&message.MessageID,
		&message.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("pb group bundle message not found")
		}
		return nil, fmt.Errorf("failed to load pb group bundle message: %w", err)
	}
	return &message, nil
}

func (s *SQLiteStore) UpsertPBGroupBundleMessage(ctx context.Context, message *PBLeaderboardMessage) error {
	query := `
		INSERT INTO pb_group_bundle_messages (guild_id, group_name, channel_id, message_id, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(guild_id, group_name) DO UPDATE SET
			channel_id = excluded.channel_id,
			message_id = excluded.message_id,
			updated_at = excluded.updated_at`
	if _, err := s.db.ExecContext(
		ctx,
		query,
		message.GuildID,
		message.GroupName,
		message.ChannelID,
		message.MessageID,
		message.UpdatedAt,
	); err != nil {
		return fmt.Errorf("failed to upsert pb group bundle message: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListPBGroupBundleMessagesByGuild(ctx context.Context, guildID string) ([]*PBLeaderboardMessage, error) {
	query := `
		SELECT guild_id, group_name, channel_id, message_id, updated_at
		FROM pb_group_bundle_messages
		WHERE guild_id = ?
		ORDER BY group_name ASC`
	rows, err := s.db.QueryContext(ctx, query, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to list pb group bundle messages: %w", err)
	}
	defer rows.Close()

	var messages []*PBLeaderboardMessage
	for rows.Next() {
		var message PBLeaderboardMessage
		if err := rows.Scan(
			&message.GuildID,
			&message.GroupName,
			&message.ChannelID,
			&message.MessageID,
			&message.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan pb group bundle message: %w", err)
		}
		messages = append(messages, &message)
	}
	return messages, rows.Err()
}

func (s *SQLiteStore) DeletePBGroupBundleMessagesByGuild(ctx context.Context, guildID string) error {
	query := `DELETE FROM pb_group_bundle_messages WHERE guild_id = ?`
	if _, err := s.db.ExecContext(ctx, query, guildID); err != nil {
		return fmt.Errorf("failed to delete pb group bundle messages: %w", err)
	}
	return nil
}

func (s *SQLiteStore) getPBSubmissionWithArgs(ctx context.Context, query string, args ...any) (*PBSubmission, error) {
	var submission PBSubmission
	var timeText sql.NullString
	var timeCentiseconds sql.NullInt64
	var proofMessageID sql.NullString
	var reviewedBy sql.NullString
	var reviewedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&submission.ID,
		&submission.GuildID,
		&submission.CategorySlug,
		&submission.DiscordUserID,
		&submission.DisplayName,
		&submission.LeaderboardDisplayName,
		&timeText,
		&timeCentiseconds,
		&submission.ProofURL,
		&proofMessageID,
		&submission.Status,
		&reviewedBy,
		&reviewedAt,
		&submission.CreatedAt,
		&submission.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("pb submission not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query pb submission: %w", err)
	}

	if timeText.Valid {
		t := timeText.String
		submission.TimeText = &t
	}
	if timeCentiseconds.Valid {
		t := timeCentiseconds.Int64
		submission.TimeCentiseconds = &t
	}
	if proofMessageID.Valid {
		t := proofMessageID.String
		submission.ProofMessageID = &t
	}
	if reviewedBy.Valid {
		t := reviewedBy.String
		submission.ReviewedByDiscordID = &t
	}
	if reviewedAt.Valid {
		t := reviewedAt.Time
		submission.ReviewedAt = &t
	}
	return &submission, nil
}
