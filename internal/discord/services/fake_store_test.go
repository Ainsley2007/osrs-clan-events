package services

import (
	"context"
	"fmt"
	"time"

	"osrs-events/internal/database"
)

type fakeEventStore struct {
	getActiveEventFn func(ctx context.Context, guildID, eventType string) (*database.Event, error)
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
func (f *fakeEventStore) CreateEvent(context.Context, *database.Event) error {
	return fmt.Errorf("not implemented")
}
func (f *fakeEventStore) DeactivateEvent(context.Context, int64) error {
	return fmt.Errorf("not implemented")
}

func utcNowPlus(minutes int) time.Time {
	return time.Now().UTC().Add(time.Duration(minutes) * time.Minute)
}
