package services

import (
	"context"
	"testing"
	"time"

	"osrs-events/internal/database"
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
	// This error string matches Store.GetActiveEvent behavior
	// for missing active events, which the service treats as non-fatal.
	return &noActiveEventError{}
}

type noActiveEventError struct{}

func (e *noActiveEventError) Error() string {
	return "no active event found"
}
