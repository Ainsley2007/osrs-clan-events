package database

import (
	"context"
	"database/sql"
	"fmt"
)

func (s *SQLiteStore) ListMetricQueue(ctx context.Context, guildID, eventType string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT metric_name FROM metric_queue
		WHERE guild_id = ? AND event_type = ?
		ORDER BY id ASC`, guildID, eventType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func (s *SQLiteStore) AppendMetricQueue(ctx context.Context, guildID, eventType, metricName string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO metric_queue (guild_id, event_type, metric_name, created_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
		guildID, eventType, metricName)
	return err
}

func (s *SQLiteStore) PeekMetricQueue(ctx context.Context, guildID, eventType string) (string, error) {
	var name string
	err := s.db.QueryRowContext(ctx, `
		SELECT metric_name FROM metric_queue
		WHERE guild_id = ? AND event_type = ?
		ORDER BY id ASC LIMIT 1`, guildID, eventType).Scan(&name)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return name, nil
}

func (s *SQLiteStore) PopMetricQueue(ctx context.Context, guildID, eventType string) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var id int64
	var name string
	err = tx.QueryRowContext(ctx, `
		SELECT id, metric_name FROM metric_queue
		WHERE guild_id = ? AND event_type = ?
		ORDER BY id ASC LIMIT 1`, guildID, eventType).Scan(&id, &name)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM metric_queue WHERE id = ?`, id); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return name, nil
}

func (s *SQLiteStore) RemoveMetricQueueAt(ctx context.Context, guildID, eventType string, position int) (string, error) {
	if position < 1 {
		return "", fmt.Errorf("position must be at least 1")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, metric_name FROM metric_queue
		WHERE guild_id = ? AND event_type = ?
		ORDER BY id ASC`, guildID, eventType)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var targetID int64
	var targetName string
	idx := 0
	for rows.Next() {
		idx++
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return "", err
		}
		if idx == position {
			targetID = id
			targetName = name
			break
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if targetID == 0 {
		return "", fmt.Errorf("no queue entry at position %d", position)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM metric_queue WHERE id = ?`, targetID); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return targetName, nil
}

func (s *SQLiteStore) ClearMetricQueue(ctx context.Context, guildID, eventType string) (int, error) {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM metric_queue WHERE guild_id = ? AND event_type = ?`,
		guildID, eventType)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}
