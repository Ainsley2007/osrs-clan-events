package services

import (
	"context"
	"fmt"
	"time"

	"osrs-events/internal/database"
	"osrs-events/internal/firebase"
	"osrs-events/internal/osrs"
)

type fakeEventStore struct {
	getActiveEventFn             func(ctx context.Context, guildID, eventType string) (*database.Event, error)
	createEventFn                func(ctx context.Context, event *database.Event) error
	getAllEventsByGuildAndTypeFn func(ctx context.Context, guildID, eventType string) ([]*database.Event, error)
	deactivateEventFn            func(ctx context.Context, eventID int64) error
}

func (f *fakeEventStore) GetActiveEvent(ctx context.Context, guildID, eventType string) (*database.Event, error) {
	if f.getActiveEventFn != nil {
		return f.getActiveEventFn(ctx, guildID, eventType)
	}
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeEventStore) GetActiveEvents(context.Context, string, string) ([]*database.Event, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeEventStore) GetAllEventsByGuildAndType(ctx context.Context, guildID, eventType string) ([]*database.Event, error) {
	if f.getAllEventsByGuildAndTypeFn != nil {
		return f.getAllEventsByGuildAndTypeFn(ctx, guildID, eventType)
	}
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeEventStore) CreateEvent(ctx context.Context, event *database.Event) error {
	if f.createEventFn != nil {
		return f.createEventFn(ctx, event)
	}
	return fmt.Errorf("not implemented")
}
func (f *fakeEventStore) DeactivateEvent(ctx context.Context, eventID int64) error {
	if f.deactivateEventFn != nil {
		return f.deactivateEventFn(ctx, eventID)
	}
	return fmt.Errorf("not implemented")
}

func utcNowPlus(minutes int) time.Time {
	return time.Now().UTC().Add(time.Duration(minutes) * time.Minute)
}

// fakeSnapshotManagerForRollover records calls to CreateInitialSnapshotsForEventsFromStats for testing.
type fakeSnapshotManagerForRollover struct {
	createInitialSnapshotsForEventsFromStatsFn func(ctx context.Context, events []*database.Event, statsByAccountID map[int64]*osrs.PlayerStats) error
	callCount                                  int
	lastEvents                                 []*database.Event
	lastStatsByAccountID                       map[int64]*osrs.PlayerStats
}

func (f *fakeSnapshotManagerForRollover) CreateInitialSnapshotsForEventsFromStats(ctx context.Context, events []*database.Event, statsByAccountID map[int64]*osrs.PlayerStats) error {
	f.callCount++
	f.lastEvents = events
	f.lastStatsByAccountID = statsByAccountID
	if f.createInitialSnapshotsForEventsFromStatsFn != nil {
		return f.createInitialSnapshotsForEventsFromStatsFn(ctx, events, statsByAccountID)
	}
	return nil
}

func (f *fakeSnapshotManagerForRollover) CreateInitialSnapshotsWithResult(context.Context, int64, string, string, string) (*InitialSnapshotResult, error) {
	return nil, nil
}
func (f *fakeSnapshotManagerForRollover) UpdateSnapshotsForEvent(context.Context, *database.Event) ([]FailedAccountUpdate, error) {
	return nil, nil
}
func (f *fakeSnapshotManagerForRollover) CalculateAndAwardPoints(context.Context, *database.Event) error {
	return nil
}

type fakeOSRSConfigProvider struct {
	config *firebase.OSRSConfig
}

func (f *fakeOSRSConfigProvider) FetchOSRSConfig(context.Context) (*firebase.OSRSConfig, error) {
	if f.config != nil {
		return f.config, nil
	}
	return &firebase.OSRSConfig{
		Bosses: []firebase.BossConfig{{
			Name:          "Vorkath",
			BossesToTrack: []string{"Vorkath"},
			PointsPerKC:   1.0,
			ThresholdKC:   50,
		}},
		Skills: []firebase.SkillConfig{{
			Name:         "Attack",
			PointsPerXP:  0.001,
			XPThreshold:  1000,
		}},
	}, nil
}
