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
	metricQueues                 map[string][]string
}

func fakeEventStoreKey(guildID, eventType string) string {
	return guildID + ":" + eventType
}

func (f *fakeEventStore) ListMetricQueue(_ context.Context, guildID, eventType string) ([]string, error) {
	if f.metricQueues == nil {
		return nil, nil
	}
	q := f.metricQueues[fakeEventStoreKey(guildID, eventType)]
	out := make([]string, len(q))
	copy(out, q)
	return out, nil
}

func (f *fakeEventStore) AppendMetricQueue(_ context.Context, guildID, eventType, metricName string) error {
	if f.metricQueues == nil {
		f.metricQueues = make(map[string][]string)
	}
	key := fakeEventStoreKey(guildID, eventType)
	f.metricQueues[key] = append(f.metricQueues[key], metricName)
	return nil
}

func (f *fakeEventStore) PeekMetricQueue(_ context.Context, guildID, eventType string) (string, error) {
	if f.metricQueues == nil {
		return "", nil
	}
	q := f.metricQueues[fakeEventStoreKey(guildID, eventType)]
	if len(q) == 0 {
		return "", nil
	}
	return q[0], nil
}

func (f *fakeEventStore) PopMetricQueue(_ context.Context, guildID, eventType string) (string, error) {
	if f.metricQueues == nil {
		return "", nil
	}
	key := fakeEventStoreKey(guildID, eventType)
	q := f.metricQueues[key]
	if len(q) == 0 {
		return "", nil
	}
	head := q[0]
	f.metricQueues[key] = q[1:]
	return head, nil
}

func (f *fakeEventStore) RemoveMetricQueueAt(_ context.Context, guildID, eventType string, position int) (string, error) {
	if position < 1 {
		return "", fmt.Errorf("position must be at least 1")
	}
	key := fakeEventStoreKey(guildID, eventType)
	q := f.metricQueues[key]
	if position > len(q) {
		return "", fmt.Errorf("no queue entry at position %d", position)
	}
	removed := q[position-1]
	f.metricQueues[key] = append(q[:position-1], q[position:]...)
	return removed, nil
}

func (f *fakeEventStore) ClearMetricQueue(_ context.Context, guildID, eventType string) (int, error) {
	key := fakeEventStoreKey(guildID, eventType)
	n := len(f.metricQueues[key])
	delete(f.metricQueues, key)
	return n, nil
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
func (f *fakeEventStore) DeactivateEvent(context.Context, int64) error {
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
