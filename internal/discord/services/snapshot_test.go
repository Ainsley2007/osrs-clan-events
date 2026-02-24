package services

import (
	"context"
	"testing"

	"osrs-events/internal/database"
	"osrs-events/internal/osrs"
)

func TestCreateInitialSnapshotsForEventsFromStats(t *testing.T) {
	ctx := context.Background()
	event := &database.Event{
		ID:           100,
		GuildID:      "guild1",
		Type:         "sotw",
		MetricJsonID: "Attack",
		XPThreshold:  1000,
	}
	accounts := []*database.Account{
		{ID: 1, RSN: "P1", DiscordUserID: "u1"},
		{ID: 2, RSN: "P2", DiscordUserID: "u2"},
	}
	statsByAccountID := map[int64]*osrs.PlayerStats{
		1: {
			Skills: []osrs.Skill{{Name: "Attack", XP: 50000}},
		},
		2: {
			Skills: []osrs.Skill{{Name: "Attack", XP: 100000}},
		},
	}

	var created []*database.Snapshot
	store := &fakeSnapshotStoreForFromStats{
		getAccountsByGuildFn: func(ctx context.Context, guildID string) ([]*database.Account, error) {
			if guildID != "guild1" {
				return nil, nil
			}
			return accounts, nil
		},
		createSnapshotFn: func(ctx context.Context, snap *database.Snapshot) error {
			created = append(created, snap)
			return nil
		},
	}
	svc := NewSnapshotService(store, nil, nil)

	err := svc.CreateInitialSnapshotsForEventsFromStats(ctx, []*database.Event{event}, statsByAccountID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(created) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(created))
	}
	byAccount := make(map[int64]*database.Snapshot)
	for _, s := range created {
		byAccount[s.AccountID] = s
	}
	if s := byAccount[1]; s == nil || s.EventID != 100 || s.StartValue != 50000 || s.CurrentValue != 50000 {
		t.Fatalf("account 1 snapshot: got %+v", byAccount[1])
	}
	if s := byAccount[2]; s == nil || s.EventID != 100 || s.StartValue != 100000 || s.CurrentValue != 100000 {
		t.Fatalf("account 2 snapshot: got %+v", byAccount[2])
	}
}

func TestCreateInitialSnapshotsForEventsFromStats_skipsAccountWithoutStats(t *testing.T) {
	ctx := context.Background()
	event := &database.Event{
		ID:           100,
		GuildID:      "guild1",
		Type:         "sotw",
		MetricJsonID: "Attack",
	}
	accounts := []*database.Account{
		{ID: 1, RSN: "P1", DiscordUserID: "u1"},
	}
	// No stats for account 1
	statsByAccountID := map[int64]*osrs.PlayerStats{}

	var created int
	store := &fakeSnapshotStoreForFromStats{
		getAccountsByGuildFn: func(ctx context.Context, guildID string) ([]*database.Account, error) {
			return accounts, nil
		},
		createSnapshotFn: func(ctx context.Context, snap *database.Snapshot) error {
			created++
			return nil
		},
	}
	svc := NewSnapshotService(store, nil, nil)

	err := svc.CreateInitialSnapshotsForEventsFromStats(ctx, []*database.Event{event}, statsByAccountID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created != 0 {
		t.Fatalf("expected 0 snapshots when no stats, got %d", created)
	}
}

// fakeSnapshotStoreForFromStats implements the minimal SnapshotStore needed for CreateInitialSnapshotsForEventsFromStats.
type fakeSnapshotStoreForFromStats struct {
	getAccountsByGuildFn func(ctx context.Context, guildID string) ([]*database.Account, error)
	createSnapshotFn    func(ctx context.Context, snap *database.Snapshot) error
}

func (f *fakeSnapshotStoreForFromStats) GetAccountsByGuild(ctx context.Context, guildID string) ([]*database.Account, error) {
	if f.getAccountsByGuildFn != nil {
		return f.getAccountsByGuildFn(ctx, guildID)
	}
	return nil, nil
}

func (f *fakeSnapshotStoreForFromStats) CreateSnapshot(ctx context.Context, snap *database.Snapshot) error {
	if f.createSnapshotFn != nil {
		return f.createSnapshotFn(ctx, snap)
	}
	return nil
}

func (f *fakeSnapshotStoreForFromStats) GetEvent(context.Context, int64) (*database.Event, error) {
	return nil, nil
}
func (f *fakeSnapshotStoreForFromStats) GetSnapshot(context.Context, int64, int64) (*database.Snapshot, error) {
	return nil, nil
}
func (f *fakeSnapshotStoreForFromStats) UpdateSnapshotCurrentValue(context.Context, int64, int64) error {
	return nil
}
func (f *fakeSnapshotStoreForFromStats) GetSnapshotsByEvent(context.Context, int64) ([]*database.Snapshot, error) {
	return nil, nil
}
func (f *fakeSnapshotStoreForFromStats) GetAccount(context.Context, int64) (*database.Account, error) {
	return nil, nil
}
func (f *fakeSnapshotStoreForFromStats) GetSnapshotsWithAccounts(context.Context, int64) ([]*database.SnapshotWithAccount, error) {
	return nil, nil
}
func (f *fakeSnapshotStoreForFromStats) UpdateParticipantPoints(context.Context, []*database.ParticipantPointUpdate) error {
	return nil
}
