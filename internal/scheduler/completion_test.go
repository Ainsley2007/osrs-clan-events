package scheduler

import (
	"context"
	"testing"
	"time"

	"osrs-events/internal/database"
	"osrs-events/internal/discord/services"
	"osrs-events/internal/osrs"
)

// mockCompletionStore records calls for completion tests.
type mockCompletionStore struct {
	getGuildFn      func(ctx context.Context, guildID string) (*database.Guild, error)
	deactivateCount int
}

func (m *mockCompletionStore) GetExpiredActiveEvents(context.Context) ([]*database.Event, error) {
	return nil, nil
}
func (m *mockCompletionStore) GetPendingStartEvents(context.Context) ([]*database.Event, error) {
	return nil, nil
}
func (m *mockCompletionStore) GetSnapshotsByEvent(context.Context, int64) ([]*database.Snapshot, error) {
	return nil, nil
}
func (m *mockCompletionStore) GetAllActiveEvents(context.Context) ([]*database.Event, error) {
	return nil, nil
}
func (m *mockCompletionStore) GetGuild(ctx context.Context, guildID string) (*database.Guild, error) {
	if m.getGuildFn != nil {
		return m.getGuildFn(ctx, guildID)
	}
	return &database.Guild{GuildID: guildID}, nil
}
func (m *mockCompletionStore) DeactivateEvent(context.Context, int64) error {
	m.deactivateCount++
	return nil
}
func (m *mockCompletionStore) UpsertMissingAccountNotificationFailure(context.Context, *database.MissingAccountNotification) error {
	return nil
}
func (m *mockCompletionStore) GetPendingMissingAccountNotifications(context.Context) ([]*database.MissingAccountNotification, error) {
	return nil, nil
}
func (m *mockCompletionStore) MarkMissingAccountNotificationDMSent(context.Context, int64, time.Time) error {
	return nil
}
func (m *mockCompletionStore) ResolveMissingAccountNotification(context.Context, int64, string, time.Time) error {
	return nil
}
func (m *mockCompletionStore) GetUnresolvedMissingAccountNotificationsByGuild(context.Context, string) ([]*database.MissingAccountNotification, error) {
	return nil, nil
}
func (m *mockCompletionStore) ShouldSendMissingAccountWeeklySummary(context.Context, string, string) (bool, error) {
	return false, nil
}
func (m *mockCompletionStore) MarkMissingAccountWeeklySummarySent(context.Context, string, string, time.Time) error {
	return nil
}

// mockCompletionSnapshotService returns configurable snapshot result and records CalculateAndAwardPoints calls.
type mockCompletionSnapshotService struct {
	updateResult *services.UpdateSnapshotsForEventsResult
	updateErr    error
	calcCount    int
}

func (m *mockCompletionSnapshotService) UpdateSnapshotsForEventsWithResult(_ context.Context, _ []*database.Event) (*services.UpdateSnapshotsForEventsResult, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	return m.updateResult, nil
}
func (m *mockCompletionSnapshotService) UpdateSnapshotsForEvent(context.Context, *database.Event) ([]services.FailedAccountUpdate, error) {
	return nil, nil
}
func (m *mockCompletionSnapshotService) UpdateSnapshotsForEvents(context.Context, []*database.Event) ([]services.FailedAccountUpdate, error) {
	return nil, nil
}
func (m *mockCompletionSnapshotService) CreateInitialSnapshots(context.Context, int64, string, string, string) (int, error) {
	return 0, nil
}
func (m *mockCompletionSnapshotService) CalculateAndAwardPoints(context.Context, *database.Event) error {
	m.calcCount++
	return nil
}

// mockCompletionEventService returns a valid rollover result.
type mockCompletionEventService struct{}

func (m *mockCompletionEventService) CompleteEvent(context.Context, *database.Event) error {
	return nil
}
func (m *mockCompletionEventService) CompleteEventWithoutSnapshotUpdate(context.Context, *database.Event) error {
	return nil
}
func (m *mockCompletionEventService) StartNewEvent(context.Context, string, string, time.Time) (*services.StartEventResult, error) {
	return nil, nil
}
func (m *mockCompletionEventService) StartNewEventFromRollover(_ context.Context, guildID, eventType string, startTime time.Time, _ map[int64]*osrs.PlayerStats) (*services.StartEventResult, error) {
	return &services.StartEventResult{
		Event:      &database.Event{ID: 2, GuildID: guildID, Type: eventType, StartTime: startTime, MetricJsonID: "Test"},
		MetricName: "Test",
	}, nil
}

// mockCompletionLeaderboardService is a no-op.
type mockCompletionLeaderboardService struct{}

func (m *mockCompletionLeaderboardService) UpdateWeeklyLeaderboard(context.Context, string, string) error {
	return nil
}
func (m *mockCompletionLeaderboardService) UpdateOverallLeaderboard(context.Context, string, string) error {
	return nil
}

// mockCompletionInitializerService is a no-op.
type mockCompletionInitializerService struct{}

func (m *mockCompletionInitializerService) RenameCategoryForEvent(context.Context, *database.Guild, string, *database.Event) error {
	return nil
}

func completionScheduler(store *mockCompletionStore, snapshot *mockCompletionSnapshotService) *Scheduler {
	return &Scheduler{
		store:              store,
		eventService:       &mockCompletionEventService{},
		snapshotService:    snapshot,
		leaderboardService: &mockCompletionLeaderboardService{},
		initializerService: &mockCompletionInitializerService{},
		session:            nil,
		clock:              realClock{},
	}
}

func TestProcessEventCompletionsForEvents_SkipsRolloverOnTransientFailure(t *testing.T) {
	store := &mockCompletionStore{}
	snapshot := &mockCompletionSnapshotService{
		updateResult: &services.UpdateSnapshotsForEventsResult{
			FailedUpdates: []services.FailedAccountUpdate{
				{RSN: "Player1", GuildID: "g1", Error: &osrs.RateLimitError{Message: "429"}},
			},
			TotalAccounts: 1,
		},
	}
	s := completionScheduler(store, snapshot)
	events := []*database.Event{
		{ID: 1, GuildID: "g1", Type: "botw", EndTime: time.Now().Add(-time.Hour)},
	}

	s.processEventCompletionsForEvents(events)

	if snapshot.calcCount != 0 {
		t.Errorf("CalculateAndAwardPoints called %d times, want 0 (transient failure should skip rollover)", snapshot.calcCount)
	}
	if store.deactivateCount != 0 {
		t.Errorf("DeactivateEvent called %d times, want 0", store.deactivateCount)
	}
}

func TestProcessEventCompletionsForEvents_SkipsRolloverOn5xx(t *testing.T) {
	store := &mockCompletionStore{}
	snapshot := &mockCompletionSnapshotService{
		updateResult: &services.UpdateSnapshotsForEventsResult{
			FailedUpdates: []services.FailedAccountUpdate{
				{RSN: "Player1", GuildID: "g1", Error: &osrs.APIError{StatusCode: 502, Message: "Bad Gateway"}},
			},
			TotalAccounts: 1,
		},
	}
	s := completionScheduler(store, snapshot)
	events := []*database.Event{
		{ID: 1, GuildID: "g1", Type: "botw", EndTime: time.Now().Add(-time.Hour)},
	}

	s.processEventCompletionsForEvents(events)

	if snapshot.calcCount != 0 {
		t.Errorf("CalculateAndAwardPoints called %d times, want 0 (5xx should skip rollover)", snapshot.calcCount)
	}
	if store.deactivateCount != 0 {
		t.Errorf("DeactivateEvent called %d times, want 0", store.deactivateCount)
	}
}

func TestProcessEventCompletionsForEvents_SkipsRolloverWhenAllAccounts404(t *testing.T) {
	store := &mockCompletionStore{}
	snapshot := &mockCompletionSnapshotService{
		updateResult: &services.UpdateSnapshotsForEventsResult{
			FailedUpdates: []services.FailedAccountUpdate{
				{RSN: "P1", GuildID: "g1", Error: &osrs.PlayerNotFoundError{RSN: "P1"}},
				{RSN: "P2", GuildID: "g1", Error: &osrs.PlayerNotFoundError{RSN: "P2"}},
			},
			TotalAccounts: 2,
		},
	}
	s := completionScheduler(store, snapshot)
	events := []*database.Event{
		{ID: 1, GuildID: "g1", Type: "botw", EndTime: time.Now().Add(-time.Hour)},
	}

	s.processEventCompletionsForEvents(events)

	if snapshot.calcCount != 0 {
		t.Errorf("CalculateAndAwardPoints called %d times, want 0 (404 for all accounts should skip rollover)", snapshot.calcCount)
	}
	if store.deactivateCount != 0 {
		t.Errorf("DeactivateEvent called %d times, want 0", store.deactivateCount)
	}
}

func TestProcessEventCompletionsForEvents_ProceedsWhenOnlySome404(t *testing.T) {
	store := &mockCompletionStore{}
	snapshot := &mockCompletionSnapshotService{
		updateResult: &services.UpdateSnapshotsForEventsResult{
			FailedUpdates: []services.FailedAccountUpdate{
				{RSN: "Missing", GuildID: "g1", Error: &osrs.PlayerNotFoundError{RSN: "Missing"}},
			},
			TotalAccounts: 2,
		},
	}
	s := completionScheduler(store, snapshot)
	events := []*database.Event{
		{ID: 1, GuildID: "g1", Type: "botw", EndTime: time.Now().Add(-time.Hour)},
	}

	s.processEventCompletionsForEvents(events)

	if snapshot.calcCount != 1 {
		t.Errorf("CalculateAndAwardPoints called %d times, want 1 (some 404 should proceed)", snapshot.calcCount)
	}
	if store.deactivateCount != 1 {
		t.Errorf("DeactivateEvent called %d times, want 1", store.deactivateCount)
	}
}

func TestProcessEventCompletionsForEvents_ProceedsWhenNoFailures(t *testing.T) {
	store := &mockCompletionStore{}
	snapshot := &mockCompletionSnapshotService{
		updateResult: &services.UpdateSnapshotsForEventsResult{
			FailedUpdates: nil,
			TotalAccounts: 2,
		},
	}
	s := completionScheduler(store, snapshot)
	events := []*database.Event{
		{ID: 1, GuildID: "g1", Type: "botw", EndTime: time.Now().Add(-time.Hour)},
	}

	s.processEventCompletionsForEvents(events)

	if snapshot.calcCount != 1 {
		t.Errorf("CalculateAndAwardPoints called %d times, want 1", snapshot.calcCount)
	}
	if store.deactivateCount != 1 {
		t.Errorf("DeactivateEvent called %d times, want 1", store.deactivateCount)
	}
}
