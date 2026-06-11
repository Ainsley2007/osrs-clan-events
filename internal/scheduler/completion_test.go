package scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"osrs-events/internal/database"
	"osrs-events/internal/discord/services"
	"osrs-events/internal/osrs"
)

// mockCompletionStore records calls for completion tests.
type mockCompletionStore struct {
	getGuildFn       func(ctx context.Context, guildID string) (*database.Guild, error)
	deactivateCount  int
	deactivatedIDs   []int64
	upsert404Count   int
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
func (m *mockCompletionStore) DeactivateEvent(_ context.Context, id int64) error {
	m.deactivateCount++
	m.deactivatedIDs = append(m.deactivatedIDs, id)
	return nil
}
func (m *mockCompletionStore) UpsertMissingAccountNotificationFailure(context.Context, *database.MissingAccountNotification) error {
	m.upsert404Count++
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
func (m *mockCompletionSnapshotService) ValidateRolloverCommitReadiness(context.Context, []*database.Event, string) error {
	return nil
}

// mockCompletionEventService supports configurable PrepareRolloverEvent and records CommitRolloverEvent calls.
type mockCompletionEventService struct {
	prepareRolloverFn func(ctx context.Context, guildID, eventType string, startTime time.Time) (*services.PreparedRolloverEvent, error)
	commitFn          func(ctx context.Context, prepared *services.PreparedRolloverEvent, stats map[int64]*osrs.PlayerStats) error
	commitCount       int
}

func (m *mockCompletionEventService) CompleteEvent(context.Context, *database.Event) error {
	return nil
}
func (m *mockCompletionEventService) CompleteEventWithoutSnapshotUpdate(context.Context, *database.Event) error {
	return nil
}
func (m *mockCompletionEventService) StartNewEvent(context.Context, string, string, time.Time) (*services.StartEventResult, error) {
	return nil, nil
}
func (m *mockCompletionEventService) PrepareRolloverEvent(ctx context.Context, guildID, eventType string, startTime time.Time) (*services.PreparedRolloverEvent, error) {
	if m.prepareRolloverFn != nil {
		return m.prepareRolloverFn(ctx, guildID, eventType, startTime)
	}
	return &services.PreparedRolloverEvent{
		Event:      &database.Event{GuildID: guildID, Type: eventType, StartTime: startTime},
		MetricName: "Test",
	}, nil
}
func (m *mockCompletionEventService) CommitRolloverEvent(ctx context.Context, prepared *services.PreparedRolloverEvent, stats map[int64]*osrs.PlayerStats) error {
	m.commitCount++
	if m.commitFn != nil {
		return m.commitFn(ctx, prepared, stats)
	}
	return nil
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
	return completionSchedulerWithEventService(store, snapshot, &mockCompletionEventService{})
}

func completionSchedulerWithEventService(store *mockCompletionStore, snapshot *mockCompletionSnapshotService, es EventService) *Scheduler {
	return &Scheduler{
		store:              store,
		eventService:       es,
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
				{AccountID: 1, GuildID: "g1", RSN: "P1", Error: &osrs.PlayerNotFoundError{RSN: "P1"}},
				{AccountID: 2, GuildID: "g1", RSN: "P2", Error: &osrs.PlayerNotFoundError{RSN: "P2"}},
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
				{AccountID: 1, GuildID: "g1", RSN: "Missing", Error: &osrs.PlayerNotFoundError{RSN: "Missing"}},
			},
			SuccessfulPairs: []services.SuccessfulAccountUpdate{
				{AccountID: 2, GuildID: "g1", RSN: "Present1"},
				{AccountID: 3, GuildID: "g1", RSN: "Present2"},
			},
			TotalAccounts: 3,
		},
	}
	s := completionScheduler(store, snapshot)
	events := []*database.Event{
		{ID: 1, GuildID: "g1", Type: "botw", EndTime: time.Now().Add(-time.Hour)},
	}

	s.processEventCompletionsForEvents(events)

	if snapshot.calcCount != 1 {
		t.Errorf("CalculateAndAwardPoints called %d times, want 1 (>50%% success with partial 404 should proceed)", snapshot.calcCount)
	}
	if store.deactivateCount != 1 {
		t.Errorf("DeactivateEvent called %d times, want 1", store.deactivateCount)
	}
}

func TestProcessEventCompletionsForEvents_DefersWhenTooFew404Successes(t *testing.T) {
	store := &mockCompletionStore{}
	var failedUpdates []services.FailedAccountUpdate
	for i := 2; i <= 50; i++ {
		failedUpdates = append(failedUpdates, services.FailedAccountUpdate{
			AccountID: int64(i),
			GuildID:   "g1",
			RSN:       fmt.Sprintf("Missing%d", i),
			Error:     &osrs.PlayerNotFoundError{RSN: fmt.Sprintf("Missing%d", i)},
		})
	}
	snapshot := &mockCompletionSnapshotService{
		updateResult: &services.UpdateSnapshotsForEventsResult{
			FailedUpdates: failedUpdates,
			SuccessfulPairs: []services.SuccessfulAccountUpdate{
				{AccountID: 1, GuildID: "g1", RSN: "OnlySuccess"},
			},
			TotalAccounts: 50,
		},
	}
	s := completionScheduler(store, snapshot)
	events := []*database.Event{
		{ID: 1, GuildID: "g1", Type: "botw", EndTime: time.Now().Add(-time.Hour)},
	}

	s.processEventCompletionsForEvents(events)

	if snapshot.calcCount != 0 {
		t.Errorf("CalculateAndAwardPoints called %d times, want 0 (1/50 success is likely API flake)", snapshot.calcCount)
	}
	if store.deactivateCount != 0 {
		t.Errorf("DeactivateEvent called %d times, want 0", store.deactivateCount)
	}
}

func TestProcessEventCompletionsForEvents_DefersWhenExactlyHalf404(t *testing.T) {
	store := &mockCompletionStore{}
	snapshot := &mockCompletionSnapshotService{
		updateResult: &services.UpdateSnapshotsForEventsResult{
			FailedUpdates: []services.FailedAccountUpdate{
				{AccountID: 1, GuildID: "g1", RSN: "Missing", Error: &osrs.PlayerNotFoundError{RSN: "Missing"}},
			},
			SuccessfulPairs: []services.SuccessfulAccountUpdate{
				{AccountID: 2, GuildID: "g1", RSN: "Present"},
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
		t.Errorf("CalculateAndAwardPoints called %d times, want 0 (50%% success is not enough)", snapshot.calcCount)
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

func TestProcessEventCompletionsForEvents_404UsesMissingAccountNotifications(t *testing.T) {
	store := &mockCompletionStore{}
	snapshot := &mockCompletionSnapshotService{
		updateResult: &services.UpdateSnapshotsForEventsResult{
			FailedUpdates: []services.FailedAccountUpdate{
				{AccountID: 1, GuildID: "g1", RSN: "Missing", Error: &osrs.PlayerNotFoundError{RSN: "Missing"}},
			},
			SuccessfulPairs: []services.SuccessfulAccountUpdate{
				{AccountID: 2, GuildID: "g1", RSN: "Present"},
			},
			TotalAccounts: 2,
		},
	}
	s := completionScheduler(store, snapshot)
	events := []*database.Event{
		{ID: 1, GuildID: "g1", Type: "botw", EndTime: time.Now().Add(-time.Hour)},
	}

	s.processEventCompletionsForEvents(events)

	if store.upsert404Count != 1 {
		t.Errorf("UpsertMissingAccountNotificationFailure called %d times, want 1", store.upsert404Count)
	}
}

func TestRolloverGuild_PhaseAFailureCausesNoDeactivateOrPoints(t *testing.T) {
	store := &mockCompletionStore{}
	snapshot := snapshotServiceWithNoFailures()
	es := &mockCompletionEventService{
		prepareRolloverFn: func(context.Context, string, string, time.Time) (*services.PreparedRolloverEvent, error) {
			return nil, fmt.Errorf("firebase config unavailable")
		},
	}
	s := completionSchedulerWithEventService(store, snapshot, es)

	events := []*database.Event{
		{ID: 1, GuildID: "g1", Type: "botw", EndTime: time.Now().Add(-time.Hour)},
		{ID: 2, GuildID: "g1", Type: "sotw", EndTime: time.Now().Add(-time.Hour)},
	}
	s.processEventCompletionsForEvents(events)

	if snapshot.calcCount != 0 {
		t.Errorf("CalculateAndAwardPoints called %d times, want 0 (Phase A failure must prevent any Phase B mutation)", snapshot.calcCount)
	}
	if store.deactivateCount != 0 {
		t.Errorf("DeactivateEvent called %d times, want 0 (Phase A failure must leave events untouched)", store.deactivateCount)
	}
}

func TestRolloverGuild_TwoExpiredEventsForOneGuildBothRollover(t *testing.T) {
	store := &mockCompletionStore{}
	snapshot := snapshotServiceWithNoFailures()
	es := &mockCompletionEventService{}
	s := completionSchedulerWithEventService(store, snapshot, es)

	events := []*database.Event{
		{ID: 1, GuildID: "g1", Type: "botw", EndTime: time.Now().Add(-time.Hour)},
		{ID: 2, GuildID: "g1", Type: "sotw", EndTime: time.Now().Add(-time.Hour)},
	}
	s.processEventCompletionsForEvents(events)

	if snapshot.calcCount != 2 {
		t.Errorf("CalculateAndAwardPoints called %d times, want 2 (one per event)", snapshot.calcCount)
	}
	if store.deactivateCount != 2 {
		t.Errorf("DeactivateEvent called %d times, want 2 (one per event)", store.deactivateCount)
	}
	if es.commitCount != 2 {
		t.Errorf("CommitRolloverEvent called %d times, want 2 (one per event)", es.commitCount)
	}
}

func TestProcessEventCompletionsForEvents_PerGuildIsolation(t *testing.T) {
	store := &mockCompletionStore{}
	snapshot := &mockCompletionSnapshotService{
		updateResult: &services.UpdateSnapshotsForEventsResult{
			// g1 has a transient (429) failure; g2 has a success only.
			FailedUpdates: []services.FailedAccountUpdate{
				{AccountID: 10, GuildID: "g1", RSN: "P1", Error: &osrs.RateLimitError{Message: "429"}},
			},
			SuccessfulPairs: []services.SuccessfulAccountUpdate{
				{AccountID: 20, GuildID: "g2", RSN: "P2"},
			},
			TotalAccounts: 2,
		},
	}
	s := completionScheduler(store, snapshot)

	events := []*database.Event{
		{ID: 1, GuildID: "g1", Type: "botw", EndTime: time.Now().Add(-time.Hour)},
		{ID: 2, GuildID: "g2", Type: "botw", EndTime: time.Now().Add(-time.Hour)},
	}
	s.processEventCompletionsForEvents(events)

	for _, id := range store.deactivatedIDs {
		if id == 1 {
			t.Errorf("event 1 (guild g1, transient failure) was deactivated - it should have been deferred")
		}
	}
	deactivatedG2 := false
	for _, id := range store.deactivatedIDs {
		if id == 2 {
			deactivatedG2 = true
		}
	}
	if !deactivatedG2 {
		t.Errorf("event 2 (guild g2, no failures) was not deactivated - it should have rolled over")
	}
}

func snapshotServiceWithNoFailures() *mockCompletionSnapshotService {
	return &mockCompletionSnapshotService{
		updateResult: &services.UpdateSnapshotsForEventsResult{
			FailedUpdates: nil,
			TotalAccounts: 1,
		},
	}
}

func TestRolloverGuild_CommitFailureAbortsBeforeSecondEvent(t *testing.T) {
	store := &mockCompletionStore{}
	snapshot := snapshotServiceWithNoFailures()
	es := &mockCompletionEventService{
		commitFn: func(_ context.Context, prepared *services.PreparedRolloverEvent, _ map[int64]*osrs.PlayerStats) error {
			if prepared.Event.Type == "botw" {
				return fmt.Errorf("db write failed")
			}
			return nil
		},
	}
	s := completionSchedulerWithEventService(store, snapshot, es)

	events := []*database.Event{
		{ID: 1, GuildID: "g1", Type: "botw", EndTime: time.Now().Add(-time.Hour)},
		{ID: 2, GuildID: "g1", Type: "sotw", EndTime: time.Now().Add(-time.Hour)},
	}
	s.processEventCompletionsForEvents(events)

	if snapshot.calcCount != 1 {
		t.Errorf("CalculateAndAwardPoints called %d times, want 1 (abort after first commit failure)", snapshot.calcCount)
	}
	if store.deactivateCount != 1 {
		t.Errorf("DeactivateEvent called %d times, want 1", store.deactivateCount)
	}
	if es.commitCount != 1 {
		t.Errorf("CommitRolloverEvent called %d times, want 1", es.commitCount)
	}
}
