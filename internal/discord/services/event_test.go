package services

import (
	"context"
	"testing"
	"time"

	"osrs-events/internal/database"
	"osrs-events/internal/firebase"
	"osrs-events/internal/osrs"
)

func TestIsEventRunning(t *testing.T) {
	// This table verifies the event running status based on end time
	// and the expected behavior when no active event is found.
	tests := []struct {
		name           string
		getEvent       func() (*database.Event, error)
		expectedActive bool
		expectErr      bool
	}{
		{
			name: "no active event returns false without error",
			getEvent: func() (*database.Event, error) {
				return nil, errNoActiveEvent()
			},
			expectedActive: false,
			expectErr:      false,
		},
		{
			name: "future end time returns true",
			getEvent: func() (*database.Event, error) {
				return &database.Event{EndTime: utcNowPlus(10)}, nil
			},
			expectedActive: true,
			expectErr:      false,
		},
		{
			name: "past end time returns false",
			getEvent: func() (*database.Event, error) {
				return &database.Event{EndTime: utcNowPlus(-10)}, nil
			},
			expectedActive: false,
			expectErr:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeEventStore{
				getActiveEventFn: func(ctx context.Context, guildID, eventType string) (*database.Event, error) {
					return test.getEvent()
				},
			}
			service := NewEventService(store, nil, nil)

			active, err := service.IsEventRunning(context.Background(), "guild", "botw")
			if test.expectErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !test.expectErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if active != test.expectedActive {
				t.Fatalf("expected active=%v, got %v", test.expectedActive, active)
			}
		})
	}
}

func TestCreateEvent(t *testing.T) {
	// This test verifies that CreateEvent sets the end time to exactly 7 days after start time
	// and marks the event as active, ensuring each week is exactly 7 days.
	startTime := time.Date(2026, 1, 23, 10, 0, 0, 0, time.UTC)
	expectedEndTime := startTime.Add(7 * 24 * time.Hour)

	var createdEvent *database.Event
	store := &fakeEventStore{
		createEventFn: func(ctx context.Context, event *database.Event) error {
			createdEvent = event
			return nil
		},
	}

	service := NewEventService(store, nil, nil)
	event := &database.Event{
		GuildID:      "test-guild",
		Type:         "botw",
		WeekNumber:   1,
		MetricJsonID: "Vorkath",
		StartTime:    startTime,
	}

	err := service.CreateEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if createdEvent == nil {
		t.Fatalf("expected event to be created")
	}

	if !createdEvent.IsActive {
		t.Fatalf("expected event to be active")
	}

	if !createdEvent.EndTime.Equal(expectedEndTime) {
		t.Fatalf("expected end time %v, got %v", expectedEndTime, createdEvent.EndTime)
	}

	// Verify the duration is exactly 7 days (within 1 second tolerance for processing)
	duration := createdEvent.EndTime.Sub(createdEvent.StartTime)
	expectedDuration := 7 * 24 * time.Hour
	if duration < expectedDuration-time.Second || duration > expectedDuration+time.Second {
		t.Fatalf("expected duration of 7 days, got %v", duration)
	}
}

func errNoActiveEvent() error {
	return database.ErrNoActiveEvent
}

func TestStartNewEventFromRollover_callsCreateInitialSnapshotsForEventsFromStats(t *testing.T) {
	ctx := context.Background()
	startTime := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)

	var createdEvent *database.Event
	eventStore := &fakeEventStore{
		getActiveEventFn: func(ctx context.Context, guildID, eventType string) (*database.Event, error) {
			return nil, database.ErrNoActiveEvent
		},
		getAllEventsByGuildAndTypeFn: func(ctx context.Context, guildID, eventType string) ([]*database.Event, error) {
			return nil, nil
		},
		createEventFn: func(ctx context.Context, event *database.Event) error {
			event.ID = 99
			createdEvent = event
			return nil
		},
	}

	fakeSnapshot := &fakeSnapshotManagerForRollover{}
	configProvider := &fakeOSRSConfigProvider{}

	svc := NewEventService(eventStore, fakeSnapshot, configProvider)
	statsByAccountID := map[int64]*osrs.PlayerStats{
		1: {Skills: []osrs.Skill{{Name: "Vorkath", XP: 100}}},
	}

	result, err := svc.StartNewEventFromRollover(ctx, "guild1", "botw", startTime, statsByAccountID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if createdEvent == nil {
		t.Fatal("expected event to be created")
	}
	if result.Event != createdEvent {
		t.Fatal("expected result.Event to be the created event")
	}
	if result.MetricName != "Vorkath" {
		t.Fatalf("expected MetricName Vorkath, got %q", result.MetricName)
	}
	if fakeSnapshot.callCount != 1 {
		t.Fatalf("expected CreateInitialSnapshotsForEventsFromStats to be called once, got %d", fakeSnapshot.callCount)
	}
	if len(fakeSnapshot.lastEvents) != 1 || fakeSnapshot.lastEvents[0].ID != 99 {
		t.Fatalf("expected one event with ID 99, got %+v", fakeSnapshot.lastEvents)
	}
	if fakeSnapshot.lastStatsByAccountID == nil || len(fakeSnapshot.lastStatsByAccountID) != 1 {
		t.Fatalf("expected stats map with one entry, got %+v", fakeSnapshot.lastStatsByAccountID)
	}
}

func TestCountOccurrences(t *testing.T) {
	got := countOccurrences([]string{"Vorkath", "zulrah", "Vorkath", ""})
	if got["vorkath"] != 2 {
		t.Errorf("expected vorkath count 2, got %d", got["vorkath"])
	}
	if got["zulrah"] != 1 {
		t.Errorf("expected zulrah count 1, got %d", got["zulrah"])
	}
	if _, ok := got[""]; ok {
		t.Error("empty name should not be in map")
	}
}

func TestWeightedPickBoss(t *testing.T) {
	bosses := []firebase.BossConfig{
		{Name: "Vorkath"},
		{Name: "Zulrah"},
		{Name: "Nightmare"},
	}

	t.Run("no recent history gives uniform chance", func(t *testing.T) {
		got := weightedPickBoss(bosses, nil)
		if got == nil {
			t.Fatal("expected non-nil")
		}
		if got.Name != "Vorkath" && got.Name != "Zulrah" && got.Name != "Nightmare" {
			t.Errorf("unexpected name %s", got.Name)
		}
	})

	t.Run("recent names down-weight those options", func(t *testing.T) {
		recent := []string{"Vorkath", "Vorkath", "Zulrah"}
		seen := make(map[string]int)
		for i := 0; i < 200; i++ {
			b := weightedPickBoss(bosses, recent)
			seen[b.Name]++
		}
		if seen["Nightmare"] < 50 {
			t.Errorf("Nightmare (never in recent) should be picked often, got %d/200", seen["Nightmare"])
		}
		if seen["Vorkath"]+seen["Zulrah"]+seen["Nightmare"] != 200 {
			t.Error("expected 200 picks total")
		}
	})

	t.Run("single item returns it", func(t *testing.T) {
		one := []firebase.BossConfig{{Name: "Only"}}
		got := weightedPickBoss(one, []string{"Only", "Only"})
		if got == nil || got.Name != "Only" {
			t.Errorf("expected Only, got %v", got)
		}
	})
}
