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

type mockHourlyStore struct {
	notifications   map[string]*database.MissingAccountNotification
	nextID          int64
	markDMSentCount int
}

func newMockHourlyStore() *mockHourlyStore {
	return &mockHourlyStore{
		notifications: make(map[string]*database.MissingAccountNotification),
		nextID:          1,
	}
}

func (m *mockHourlyStore) GetExpiredActiveEvents(context.Context) ([]*database.Event, error) {
	return nil, nil
}
func (m *mockHourlyStore) GetPendingStartEvents(context.Context) ([]*database.Event, error) {
	return nil, nil
}
func (m *mockHourlyStore) GetSnapshotsByEvent(context.Context, int64) ([]*database.Snapshot, error) {
	return nil, nil
}
func (m *mockHourlyStore) GetAllActiveEvents(context.Context) ([]*database.Event, error) {
	return nil, nil
}
func (m *mockHourlyStore) GetGuild(context.Context, string) (*database.Guild, error) {
	return nil, nil
}
func (m *mockHourlyStore) DeactivateEvent(context.Context, int64) error {
	return nil
}

func (m *mockHourlyStore) UpsertMissingAccountNotificationFailure(_ context.Context, notification *database.MissingAccountNotification) error {
	key := fmt.Sprintf("%d|%s", notification.AccountID, notification.GuildID)
	existing, ok := m.notifications[key]
	if ok && existing.ResolvedAt == nil {
		existing.LastFailedAt = notification.LastFailedAt
		existing.RSN = notification.RSN
		existing.DiscordUserID = notification.DiscordUserID
		return nil
	}

	copy := *notification
	copy.ID = m.nextID
	m.nextID++
	m.notifications[key] = &copy
	return nil
}

func (m *mockHourlyStore) GetPendingMissingAccountNotifications(context.Context) ([]*database.MissingAccountNotification, error) {
	var pending []*database.MissingAccountNotification
	for _, notification := range m.notifications {
		if notification.ResolvedAt == nil && notification.DMSentAt == nil {
			pending = append(pending, notification)
		}
	}
	return pending, nil
}

func (m *mockHourlyStore) MarkMissingAccountNotificationDMSent(_ context.Context, notificationID int64, sentAt time.Time) error {
	for _, notification := range m.notifications {
		if notification.ID == notificationID {
			t := sentAt
			notification.DMSentAt = &t
			m.markDMSentCount++
			return nil
		}
	}
	return nil
}

func (m *mockHourlyStore) ResolveMissingAccountNotification(_ context.Context, accountID int64, guildID string, resolvedAt time.Time) error {
	for _, notification := range m.notifications {
		if notification.AccountID == accountID && notification.GuildID == guildID && notification.ResolvedAt == nil {
			t := resolvedAt
			notification.ResolvedAt = &t
		}
	}
	return nil
}

func (m *mockHourlyStore) GetUnresolvedMissingAccountNotificationsByGuild(_ context.Context, guildID string) ([]*database.MissingAccountNotification, error) {
	var unresolved []*database.MissingAccountNotification
	for _, notification := range m.notifications {
		if notification.GuildID == guildID && notification.ResolvedAt == nil {
			unresolved = append(unresolved, notification)
		}
	}
	return unresolved, nil
}

func TestProcessMissingAccountNotifications_SendsDMOnlyOncePerUnresolvedState(t *testing.T) {
	store := newMockHourlyStore()
	s := &Scheduler{store: store, notifier: noopNotifier{}}
	now := time.Now().UTC()
	result := &services.UpdateSnapshotsForEventsResult{
		FailedUpdates: []services.FailedAccountUpdate{
			{
				AccountID:     1,
				DiscordUserID: "u1",
				RSN:           "MissingRSN",
				GuildID:       "g1",
				Error:         &osrs.PlayerNotFoundError{RSN: "MissingRSN"},
			},
		},
	}

	s.processMissingAccountNotifications(context.Background(), now, result)
	s.processMissingAccountNotifications(context.Background(), now.Add(time.Hour), result)

	if store.markDMSentCount != 1 {
		t.Fatalf("expected one DM sent mark, got %d", store.markDMSentCount)
	}
}

func TestProcessMissingAccountNotifications_ResolvesAfterSuccessfulFetch(t *testing.T) {
	store := newMockHourlyStore()
	s := &Scheduler{store: store, notifier: noopNotifier{}}
	now := time.Now().UTC()

	s.processMissingAccountNotifications(context.Background(), now, &services.UpdateSnapshotsForEventsResult{
		FailedUpdates: []services.FailedAccountUpdate{
			{
				AccountID:     2,
				DiscordUserID: "u2",
				RSN:           "OldRSN",
				GuildID:       "g1",
				Error:         &osrs.PlayerNotFoundError{RSN: "OldRSN"},
			},
		},
	})

	s.processMissingAccountNotifications(context.Background(), now.Add(time.Hour), &services.UpdateSnapshotsForEventsResult{
		SuccessfulPairs: []services.SuccessfulAccountUpdate{
			{AccountID: 2, GuildID: "g1"},
		},
	})

	unresolved, err := store.GetUnresolvedMissingAccountNotificationsByGuild(context.Background(), "g1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(unresolved) != 0 {
		t.Fatalf("expected no unresolved notifications, got %d", len(unresolved))
	}
}
