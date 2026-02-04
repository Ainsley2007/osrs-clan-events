package database

import (
	"context"
	"fmt"
)

func (s *SQLiteStore) SaveDonation(ctx context.Context, donation *Donation) error {
	query := `INSERT INTO donations (guild_id, discord_user_id, amount, created_at, created_by)
		VALUES (?, ?, ?, ?, ?)`

	result, err := s.db.ExecContext(ctx, query,
		donation.GuildID, donation.DiscordUserID, donation.Amount, donation.CreatedAt, donation.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("failed to save donation: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get donation ID: %w", err)
	}

	donation.ID = id
	return nil
}

func (s *SQLiteStore) GetDonationsByGuild(ctx context.Context, guildID string) ([]*Donation, error) {
	query := `SELECT id, guild_id, discord_user_id, amount, created_at, created_by
		FROM donations WHERE guild_id = ? ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to query donations: %w", err)
	}
	defer rows.Close()

	var donations []*Donation
	for rows.Next() {
		var d Donation
		err := rows.Scan(&d.ID, &d.GuildID, &d.DiscordUserID, &d.Amount, &d.CreatedAt, &d.CreatedBy)
		if err != nil {
			return nil, fmt.Errorf("failed to scan donation: %w", err)
		}
		donations = append(donations, &d)
	}

	return donations, rows.Err()
}

func (s *SQLiteStore) GetTotalDonatedByUser(ctx context.Context, guildID, discordUserID string) (int64, error) {
	query := `SELECT COALESCE(SUM(amount), 0) FROM donations WHERE guild_id = ? AND discord_user_id = ?`

	var total int64
	err := s.db.QueryRowContext(ctx, query, guildID, discordUserID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to get total donated: %w", err)
	}

	return total, nil
}

func (s *SQLiteStore) SaveDonationSpending(ctx context.Context, spending *DonationSpending) error {
	query := `INSERT INTO donation_spending (guild_id, amount, description, created_at, created_by)
		VALUES (?, ?, ?, ?, ?)`

	result, err := s.db.ExecContext(ctx, query,
		spending.GuildID, spending.Amount, spending.Description, spending.CreatedAt, spending.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("failed to save donation spending: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get spending ID: %w", err)
	}

	spending.ID = id
	return nil
}

func (s *SQLiteStore) GetTotalSpent(ctx context.Context, guildID string) (int64, error) {
	query := `SELECT COALESCE(SUM(amount), 0) FROM donation_spending WHERE guild_id = ?`

	var total int64
	err := s.db.QueryRowContext(ctx, query, guildID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to get total spent: %w", err)
	}

	return total, nil
}

func (s *SQLiteStore) UpdateDonationChannel(ctx context.Context, guildID, channelID string) error {
	guild, err := s.GetGuild(ctx, guildID)
	if err != nil {
		return fmt.Errorf("failed to get guild: %w", err)
	}

	guild.DonationChannelID = channelID
	return s.SaveGuild(ctx, guild)
}

func (s *SQLiteStore) UpdateDonationMessageID(ctx context.Context, guildID, messageID string) error {
	guild, err := s.GetGuild(ctx, guildID)
	if err != nil {
		return fmt.Errorf("failed to get guild: %w", err)
	}

	guild.DonationMsgID = messageID
	return s.SaveGuild(ctx, guild)
}
