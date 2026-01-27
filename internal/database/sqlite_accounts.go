package database

import (
	"context"
	"database/sql"
	"fmt"
)

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

func (s *SQLiteStore) GetAccountsByGuild(ctx context.Context, guildID string) ([]*Account, error) {
	query := `SELECT DISTINCT a.id, a.rsn, a.discord_user_id, a.error_count, a.is_active 
		FROM accounts a
		INNER JOIN participants p ON a.discord_user_id = p.discord_user_id
		WHERE p.guild_id = ? AND a.is_active = 1`
	rows, err := s.db.QueryContext(ctx, query, guildID)
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
