package database

import (
	"context"
	"database/sql"
	"fmt"
)

// Events
func (s *SQLiteStore) SaveEvent(ctx context.Context, e *Event) error {
	if e.ID == 0 {
		query := `INSERT INTO events (guild_id, type, week_number, metric_json_id, bosses_to_track, start_time, end_time, is_active, points_per_kc, points_per_xp, threshold_kc, xp_threshold)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		res, err := s.db.ExecContext(ctx, query, e.GuildID, e.Type, e.WeekNumber, e.MetricJsonID, e.BossesToTrack, e.StartTime, e.EndTime, e.IsActive, e.PointsPerKC, e.PointsPerXP, e.ThresholdKC, e.XPThreshold)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		e.ID = id
		return nil
	}

	query := `UPDATE events SET guild_id = ?, type = ?, week_number = ?, metric_json_id = ?, bosses_to_track = ?, start_time = ?, end_time = ?, is_active = ?, points_per_kc = ?, points_per_xp = ?, threshold_kc = ?, xp_threshold = ?
		WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, e.GuildID, e.Type, e.WeekNumber, e.MetricJsonID, e.BossesToTrack, e.StartTime, e.EndTime, e.IsActive, e.PointsPerKC, e.PointsPerXP, e.ThresholdKC, e.XPThreshold, e.ID)
	return err
}

func (s *SQLiteStore) GetEvent(ctx context.Context, id int64) (*Event, error) {
	query := `SELECT id, guild_id, type, week_number, metric_json_id, bosses_to_track, start_time, end_time, is_active, points_per_kc, points_per_xp, threshold_kc, xp_threshold FROM events WHERE id = ?`
	var e Event
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&e.ID, &e.GuildID, &e.Type, &e.WeekNumber, &e.MetricJsonID, &e.BossesToTrack, &e.StartTime, &e.EndTime, &e.IsActive, &e.PointsPerKC, &e.PointsPerXP, &e.ThresholdKC, &e.XPThreshold,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("event not found")
	}
	return &e, err
}

func (s *SQLiteStore) GetActiveEvent(ctx context.Context, guildID string, eventType string) (*Event, error) {
	query := `SELECT id, guild_id, type, week_number, metric_json_id, bosses_to_track, start_time, end_time, is_active, points_per_kc, points_per_xp, threshold_kc, xp_threshold
		FROM events WHERE guild_id = ? AND type = ? AND is_active = 1 ORDER BY start_time DESC LIMIT 1`

	var e Event
	err := s.db.QueryRowContext(ctx, query, guildID, eventType).Scan(
		&e.ID, &e.GuildID, &e.Type, &e.WeekNumber, &e.MetricJsonID, &e.BossesToTrack, &e.StartTime, &e.EndTime, &e.IsActive, &e.PointsPerKC, &e.PointsPerXP, &e.ThresholdKC, &e.XPThreshold,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no active event found")
	}
	return &e, err
}

func (s *SQLiteStore) GetActiveEvents(ctx context.Context, guildID string, eventType string) ([]*Event, error) {
	query := `SELECT id, guild_id, type, week_number, metric_json_id, bosses_to_track, start_time, end_time, is_active, points_per_kc, points_per_xp, threshold_kc, xp_threshold
		FROM events WHERE guild_id = ? AND type = ? AND is_active = 1 ORDER BY start_time DESC`

	rows, err := s.db.QueryContext(ctx, query, guildID, eventType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.GuildID, &e.Type, &e.WeekNumber, &e.MetricJsonID, &e.BossesToTrack, &e.StartTime, &e.EndTime, &e.IsActive, &e.PointsPerKC, &e.PointsPerXP, &e.ThresholdKC, &e.XPThreshold); err != nil {
			return nil, err
		}
		events = append(events, &e)
	}
	return events, rows.Err()
}

func (s *SQLiteStore) GetAllEventsByGuildAndType(ctx context.Context, guildID string, eventType string) ([]*Event, error) {
	query := `SELECT id, guild_id, type, week_number, metric_json_id, bosses_to_track, start_time, end_time, is_active, points_per_kc, points_per_xp, threshold_kc, xp_threshold
		FROM events WHERE guild_id = ? AND type = ? ORDER BY start_time DESC`

	rows, err := s.db.QueryContext(ctx, query, guildID, eventType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.GuildID, &e.Type, &e.WeekNumber, &e.MetricJsonID, &e.BossesToTrack, &e.StartTime, &e.EndTime, &e.IsActive, &e.PointsPerKC, &e.PointsPerXP, &e.ThresholdKC, &e.XPThreshold); err != nil {
			return nil, err
		}
		events = append(events, &e)
	}
	return events, rows.Err()
}

func (s *SQLiteStore) CreateEvent(ctx context.Context, e *Event) error {
	query := `INSERT INTO events (guild_id, type, week_number, metric_json_id, bosses_to_track, start_time, end_time, is_active, points_per_kc, points_per_xp, threshold_kc, xp_threshold)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, query, e.GuildID, e.Type, e.WeekNumber, e.MetricJsonID, e.BossesToTrack, e.StartTime, e.EndTime, e.IsActive, e.PointsPerKC, e.PointsPerXP, e.ThresholdKC, e.XPThreshold)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	e.ID = id
	return nil
}

func (s *SQLiteStore) GetPendingStartEvents(ctx context.Context) ([]*Event, error) {
	query := `SELECT id, guild_id, type, week_number, metric_json_id, bosses_to_track, start_time, end_time, is_active, points_per_kc, points_per_xp, threshold_kc, xp_threshold
		FROM events 
		WHERE is_active = 1 
		AND start_time <= datetime('now')
		AND start_time >= datetime('now', '-10 minutes')
		ORDER BY start_time ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.GuildID, &e.Type, &e.WeekNumber, &e.MetricJsonID, &e.BossesToTrack, &e.StartTime, &e.EndTime, &e.IsActive, &e.PointsPerKC, &e.PointsPerXP, &e.ThresholdKC, &e.XPThreshold); err != nil {
			return nil, err
		}
		events = append(events, &e)
	}
	return events, rows.Err()
}

func (s *SQLiteStore) GetAllActiveEvents(ctx context.Context) ([]*Event, error) {
	query := `SELECT id, guild_id, type, week_number, metric_json_id, bosses_to_track, start_time, end_time, is_active, points_per_kc, points_per_xp, threshold_kc, xp_threshold
		FROM events 
		WHERE is_active = 1 
		AND start_time <= datetime('now')
		AND end_time > datetime('now', '+10 minutes')
		ORDER BY guild_id, type`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.GuildID, &e.Type, &e.WeekNumber, &e.MetricJsonID, &e.BossesToTrack, &e.StartTime, &e.EndTime, &e.IsActive, &e.PointsPerKC, &e.PointsPerXP, &e.ThresholdKC, &e.XPThreshold); err != nil {
			return nil, err
		}
		events = append(events, &e)
	}
	return events, rows.Err()
}

func (s *SQLiteStore) GetExpiringEvents(ctx context.Context) ([]*Event, error) {
	// Find events that have ended (within 5 minute window to catch them reliably)
	// Wider window ensures we don't miss events if a previous rollover takes longer than expected
	// This ensures we process rollovers at the exact end time ± a few minutes
	query := `SELECT id, guild_id, type, week_number, metric_json_id, bosses_to_track, start_time, end_time, is_active, points_per_kc, points_per_xp, threshold_kc, xp_threshold
		FROM events 
		WHERE is_active = 1 
		AND end_time <= datetime('now')
		AND end_time >= datetime('now', '-5 minutes')
		ORDER BY end_time ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.GuildID, &e.Type, &e.WeekNumber, &e.MetricJsonID, &e.BossesToTrack, &e.StartTime, &e.EndTime, &e.IsActive, &e.PointsPerKC, &e.PointsPerXP, &e.ThresholdKC, &e.XPThreshold); err != nil {
			return nil, err
		}
		events = append(events, &e)
	}
	return events, rows.Err()
}

func (s *SQLiteStore) GetStaleEvents(ctx context.Context) ([]*Event, error) {
	query := `SELECT id, guild_id, type, week_number, metric_json_id, bosses_to_track, start_time, end_time, is_active, points_per_kc, points_per_xp, threshold_kc, xp_threshold
		FROM events 
		WHERE is_active = 1 
		AND end_time < datetime('now', '-5 minutes')
		ORDER BY end_time ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.GuildID, &e.Type, &e.WeekNumber, &e.MetricJsonID, &e.BossesToTrack, &e.StartTime, &e.EndTime, &e.IsActive, &e.PointsPerKC, &e.PointsPerXP, &e.ThresholdKC, &e.XPThreshold); err != nil {
			return nil, err
		}
		events = append(events, &e)
	}
	return events, rows.Err()
}

func (s *SQLiteStore) DeactivateEvent(ctx context.Context, eventID int64) error {
	query := `UPDATE events SET is_active = 0 WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, eventID)
	return err
}
