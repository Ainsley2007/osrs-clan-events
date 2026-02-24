package database

import (
	"context"
	"database/sql"
	"fmt"
)

func (s *SQLiteStore) CreateSnapshot(ctx context.Context, snap *Snapshot) error {
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

func (s *SQLiteStore) UpdateSnapshotCurrentValue(ctx context.Context, snapshotID int64, currentValue int64) error {
	query := `UPDATE snapshots SET current_value = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, currentValue, snapshotID)
	return err
}

func (s *SQLiteStore) GetSnapshotsWithAccounts(ctx context.Context, eventID int64) ([]*SnapshotWithAccount, error) {
	query := `SELECT s.id, s.event_id, s.account_id, s.start_value, s.current_value,
		a.id, a.rsn, a.discord_user_id, a.error_count, a.is_active
		FROM snapshots s
		JOIN accounts a ON s.account_id = a.id
		WHERE s.event_id = ?`

	rows, err := s.db.QueryContext(ctx, query, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*SnapshotWithAccount
	for rows.Next() {
		var snap Snapshot
		var acc Account
		if err := rows.Scan(&snap.ID, &snap.EventID, &snap.AccountID, &snap.StartValue, &snap.CurrentValue, &acc.ID, &acc.RSN, &acc.DiscordUserID, &acc.ErrorCount, &acc.IsActive); err != nil {
			return nil, err
		}
		result = append(result, &SnapshotWithAccount{Snapshot: &snap, Account: &acc})
	}
	return result, rows.Err()
}
