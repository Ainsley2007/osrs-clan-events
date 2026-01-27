package services

import (
	"context"
	"fmt"
	"time"

	"osrs-events/internal/database"
)

type fakeStore struct {
	getActiveEventFn func(ctx context.Context, guildID, eventType string) (*database.Event, error)
}

func (f *fakeStore) SaveGuild(context.Context, *database.Guild) error {
	return fmt.Errorf("not implemented")
}
func (f *fakeStore) GetGuild(context.Context, string) (*database.Guild, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeStore) SaveParticipant(context.Context, *database.Participant) error {
	return fmt.Errorf("not implemented")
}
func (f *fakeStore) GetParticipant(context.Context, string, string) (*database.Participant, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeStore) DeleteParticipant(context.Context, string, string) error {
	return fmt.Errorf("not implemented")
}
func (f *fakeStore) SaveAccount(context.Context, *database.Account) error {
	return fmt.Errorf("not implemented")
}
func (f *fakeStore) GetAccount(context.Context, int64) (*database.Account, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeStore) GetAccountsByDiscordID(context.Context, string) ([]*database.Account, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeStore) GetAccountsByGuild(context.Context, string) ([]*database.Account, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeStore) GetAccountByRSN(context.Context, string, string) (*database.Account, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeStore) GetActiveAccounts(context.Context) ([]*database.Account, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeStore) DeleteAccount(context.Context, int64) error {
	return fmt.Errorf("not implemented")
}
func (f *fakeStore) UpdateAccountRSN(context.Context, int64, string) error {
	return fmt.Errorf("not implemented")
}
func (f *fakeStore) SaveEvent(context.Context, *database.Event) error {
	return fmt.Errorf("not implemented")
}
func (f *fakeStore) GetEvent(context.Context, int64) (*database.Event, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeStore) GetActiveEvent(ctx context.Context, guildID, eventType string) (*database.Event, error) {
	if f.getActiveEventFn != nil {
		return f.getActiveEventFn(ctx, guildID, eventType)
	}
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeStore) GetActiveEvents(context.Context, string, string) ([]*database.Event, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeStore) CreateEvent(context.Context, *database.Event) error {
	return fmt.Errorf("not implemented")
}
func (f *fakeStore) GetPendingStartEvents(context.Context) ([]*database.Event, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeStore) GetAllActiveEvents(context.Context) ([]*database.Event, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeStore) GetExpiringEvents(context.Context) ([]*database.Event, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeStore) GetStaleEvents(context.Context) ([]*database.Event, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeStore) DeactivateEvent(context.Context, int64) error {
	return fmt.Errorf("not implemented")
}
func (f *fakeStore) SaveSnapshot(context.Context, *database.Snapshot) error {
	return fmt.Errorf("not implemented")
}
func (f *fakeStore) CreateSnapshot(context.Context, *database.Snapshot) error {
	return fmt.Errorf("not implemented")
}
func (f *fakeStore) GetSnapshot(context.Context, int64, int64) (*database.Snapshot, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeStore) GetSnapshotsByEvent(context.Context, int64) ([]*database.Snapshot, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeStore) UpdateSnapshotCurrentValue(context.Context, int64, int64) error {
	return fmt.Errorf("not implemented")
}
func (f *fakeStore) GetSnapshotsWithAccounts(context.Context, int64) ([]*database.SnapshotWithAccount, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeStore) UpdateParticipantPoints(context.Context, []*database.ParticipantPointUpdate) error {
	return fmt.Errorf("not implemented")
}
func (f *fakeStore) Close() error {
	return nil
}

func utcNowPlus(minutes int) time.Time {
	return time.Now().UTC().Add(time.Duration(minutes) * time.Minute)
}
