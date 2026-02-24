package scheduler

import (
	"context"
	"time"

	"osrs-events/internal/database"
	"osrs-events/internal/discord/services"
	"osrs-events/internal/osrs"
)

type Store interface {
	GetExpiredActiveEvents(ctx context.Context) ([]*database.Event, error)
	GetPendingStartEvents(ctx context.Context) ([]*database.Event, error)
	GetSnapshotsByEvent(ctx context.Context, eventID int64) ([]*database.Snapshot, error)
	GetAllActiveEvents(ctx context.Context) ([]*database.Event, error)
	GetGuild(ctx context.Context, guildID string) (*database.Guild, error)
	DeactivateEvent(ctx context.Context, eventID int64) error
}

type EventService interface {
	CompleteEvent(ctx context.Context, event *database.Event) error
	CompleteEventWithoutSnapshotUpdate(ctx context.Context, event *database.Event) error
	StartNewEvent(ctx context.Context, guildID string, eventType string, startTime time.Time) (*services.StartEventResult, error)
	StartNewEventFromRollover(ctx context.Context, guildID string, eventType string, startTime time.Time, statsByAccountID map[int64]*osrs.PlayerStats) (*services.StartEventResult, error)
}

type SnapshotService interface {
	UpdateSnapshotsForEvent(ctx context.Context, event *database.Event) ([]services.FailedAccountUpdate, error)
	UpdateSnapshotsForEvents(ctx context.Context, events []*database.Event) ([]services.FailedAccountUpdate, error)
	UpdateSnapshotsForEventsWithResult(ctx context.Context, events []*database.Event) (*services.UpdateSnapshotsForEventsResult, error)
	CreateInitialSnapshots(ctx context.Context, eventID int64, guildID, metricName, metricType string) (int, error)
	CalculateAndAwardPoints(ctx context.Context, event *database.Event) error
}

type LeaderboardService interface {
	UpdateWeeklyLeaderboard(ctx context.Context, guildID string, eventType string) error
	UpdateOverallLeaderboard(ctx context.Context, guildID string, eventType string) error
}

type InitializerService interface {
	RenameCategoryForEvent(ctx context.Context, guild *database.Guild, eventType string, event *database.Event) error
}
