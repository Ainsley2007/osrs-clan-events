package database

import (
	"context"
	"testing"
	"time"
)

func TestDeleteGuildRemovesEvents(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	guildID := "1342212870460411984"
	if err := store.SaveGuild(ctx, &Guild{GuildID: guildID}); err != nil {
		t.Fatalf("save guild: %v", err)
	}

	start := time.Now().UTC()
	event := &Event{
		GuildID: guildID, Type: "botw", WeekNumber: 1, MetricJsonID: "Vorkath",
		StartTime: start, EndTime: start.Add(7 * 24 * time.Hour), IsActive: true,
	}
	if err := store.CreateEvent(ctx, event); err != nil {
		t.Fatalf("create event: %v", err)
	}

	if err := store.DeleteGuild(ctx, guildID); err != nil {
		t.Fatalf("delete guild: %v", err)
	}

	events, err := store.GetAllActiveEvents(ctx)
	if err != nil {
		t.Fatalf("get active events: %v", err)
	}
	for _, e := range events {
		if e.GuildID == guildID {
			t.Fatalf("expected no events for deleted guild, found event %d", e.ID)
		}
	}
}

func TestPurgeOrphanedEvents(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	guildID := "orphan-guild"
	if err := store.SaveGuild(ctx, &Guild{GuildID: guildID}); err != nil {
		t.Fatalf("save guild: %v", err)
	}
	start := time.Now().UTC()
	event := &Event{
		GuildID: guildID, Type: "sotw", WeekNumber: 1, MetricJsonID: "Attack",
		StartTime: start, EndTime: start.Add(7 * 24 * time.Hour), IsActive: true,
	}
	if err := store.CreateEvent(ctx, event); err != nil {
		t.Fatalf("create event: %v", err)
	}

	// Simulate partial cleanup from before FK was enforced on every connection.
	if _, err := store.db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatalf("disable foreign keys: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `DELETE FROM guilds WHERE guild_id = ?`, guildID); err != nil {
		t.Fatalf("delete guild row only: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("re-enable foreign keys: %v", err)
	}

	n, err := store.PurgeOrphanedEvents(ctx)
	if err != nil {
		t.Fatalf("purge orphaned events: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 orphaned guild purged, got %d", n)
	}

	events, err := store.GetAllActiveEvents(ctx)
	if err != nil {
		t.Fatalf("get active events: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no active events after purge, got %d", len(events))
	}
}
