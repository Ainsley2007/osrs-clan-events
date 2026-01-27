package services

import (
	"context"
	"testing"

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
			store := &fakeStore{
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

func errNoActiveEvent() error {
	// This error string matches Store.GetActiveEvent behavior
	// for missing active events, which the service treats as non-fatal.
	return &noActiveEventError{}
}

type noActiveEventError struct{}

func (e *noActiveEventError) Error() string {
	return "no active event found"
}
