package database

import (
	"context"
	"testing"
	"time"
)

func TestGetTotalGainedByParticipant(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	guildID := "guild1"
	// Create guild and participants so foreign keys are satisfied
	if err := store.SaveGuild(ctx, &Guild{GuildID: guildID}); err != nil {
		t.Fatalf("save guild: %v", err)
	}
	if err := store.SaveParticipant(ctx, &Participant{DiscordUserID: "user1", GuildID: guildID}); err != nil {
		t.Fatalf("save participant: %v", err)
	}
	if err := store.SaveParticipant(ctx, &Participant{DiscordUserID: "user2", GuildID: guildID}); err != nil {
		t.Fatalf("save participant: %v", err)
	}
	// Create accounts (linked to participants by discord_user_id)
	if err := store.SaveAccount(ctx, &Account{RSN: "P1", DiscordUserID: "user1"}); err != nil {
		t.Fatalf("save account 1: %v", err)
	}
	acc2 := &Account{RSN: "P2", DiscordUserID: "user1"}
	if err := store.SaveAccount(ctx, acc2); err != nil {
		t.Fatalf("save account 2: %v", err)
	}
	acc3 := &Account{RSN: "P3", DiscordUserID: "user2"}
	if err := store.SaveAccount(ctx, acc3); err != nil {
		t.Fatalf("save account 3: %v", err)
	}

	// Create two SOTW events
	start1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	event1 := &Event{
		GuildID: guildID, Type: "sotw", WeekNumber: 1, MetricJsonID: "Attack",
		StartTime: start1, EndTime: start1.Add(7*24*time.Hour), IsActive: false,
		PointsPerXP: 0.001, XPThreshold: 1000,
	}
	if err := store.CreateEvent(ctx, event1); err != nil {
		t.Fatalf("create event 1: %v", err)
	}
	event2 := &Event{
		GuildID: guildID, Type: "sotw", WeekNumber: 2, MetricJsonID: "Strength",
		StartTime: start1.Add(7*24*time.Hour), EndTime: start1.Add(14*24*time.Hour), IsActive: false,
		PointsPerXP: 0.001, XPThreshold: 1000,
	}
	if err := store.CreateEvent(ctx, event2); err != nil {
		t.Fatalf("create event 2: %v", err)
	}

	// Get account IDs (SaveAccount with AUTOINCREMENT sets ID)
	acc2, _ = store.GetAccountByRSN(ctx, "P2", "user1")
	acc3, _ = store.GetAccountByRSN(ctx, "P3", "user2")
	if acc2 == nil || acc3 == nil {
		t.Fatal("accounts not found")
	}

	// Snapshots: user1 has two accounts, each with gain in two events
	// Account 2: event1 gain 1000, event2 gain 2000 -> total 3000 for user1
	// Account 3: event1 gain 500, event2 gain 500 -> total 1000 for user2
	for _, row := range []struct {
		eventID      int64
		accountID    int64
		start, current int64
	}{
		{event1.ID, acc2.ID, 1000, 2000},
		{event2.ID, acc2.ID, 2000, 4000},
		{event1.ID, acc3.ID, 500, 1000},
		{event2.ID, acc3.ID, 1000, 1500},
	} {
		if err := store.CreateSnapshot(ctx, &Snapshot{
			EventID: row.eventID, AccountID: row.accountID,
			StartValue: row.start, CurrentValue: row.current,
		}); err != nil {
			t.Fatalf("create snapshot: %v", err)
		}
	}

	got, err := store.GetTotalGainedByParticipant(ctx, guildID, "sotw")
	if err != nil {
		t.Fatalf("GetTotalGainedByParticipant: %v", err)
	}
	// user1: (2000-1000)+(4000-2000) = 3000
	if got["user1"] != 3000 {
		t.Errorf("user1 total gained: want 3000, got %d", got["user1"])
	}
	// user2: (1000-500)+(1500-1000) = 1000
	if got["user2"] != 1000 {
		t.Errorf("user2 total gained: want 1000, got %d", got["user2"])
	}
}
