package scheduler

import (
	"context"
	"time"

	"osrs-events/internal/database"
	"osrs-events/internal/discord/services"
)

type Store interface {
	GetStaleEvents(ctx context.Context) ([]*database.Event, error)
	GetPendingStartEvents(ctx context.Context) ([]*database.Event, error)
	GetSnapshotsByEvent(ctx context.Context, eventID int64) ([]*database.Snapshot, error)
	GetExpiringEvents(ctx context.Context) ([]*database.Event, error)
	GetAllActiveEvents(ctx context.Context) ([]*database.Event, error)
	GetGuild(ctx context.Context, guildID string) (*database.Guild, error)
}

type EventService interface {
	CompleteEvent(ctx context.Context, event *database.Event) error
	AutoRollover(ctx context.Context, guildID string, eventType string, startTime time.Time) (*services.StartEventResult, error)
}

type SnapshotService interface {
	UpdateSnapshotsForEvent(ctx context.Context, event *database.Event) ([]services.FailedAccountUpdate, error)
	CreateInitialSnapshots(ctx context.Context, eventID int64, guildID, metricName, metricType string) (int, error)
}

type LeaderboardService interface {
	UpdateWeeklyLeaderboard(ctx context.Context, guildID string, eventType string) error
	UpdateOverallLeaderboard(ctx context.Context, guildID string, eventType string) error
}
