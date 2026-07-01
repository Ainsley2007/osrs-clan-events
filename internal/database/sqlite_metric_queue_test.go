package database

import (
	"context"
	"testing"
)

func TestMetricQueueFIFO(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	guildID := "guild-1"
	eventType := "botw"
	if err := store.SaveGuild(ctx, &Guild{GuildID: guildID}); err != nil {
		t.Fatalf("SaveGuild: %v", err)
	}

	for _, name := range []string{"Boss A", "Boss B", "Boss C"} {
		if err := store.AppendMetricQueue(ctx, guildID, eventType, name); err != nil {
			t.Fatalf("AppendMetricQueue %q: %v", name, err)
		}
	}

	got, err := store.ListMetricQueue(ctx, guildID, eventType)
	if err != nil {
		t.Fatalf("ListMetricQueue: %v", err)
	}
	want := []string{"Boss A", "Boss B", "Boss C"}
	if len(got) != len(want) {
		t.Fatalf("list len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("list[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	head, err := store.PeekMetricQueue(ctx, guildID, eventType)
	if err != nil || head != "Boss A" {
		t.Fatalf("PeekMetricQueue = %q, err = %v", head, err)
	}

	popped, err := store.PopMetricQueue(ctx, guildID, eventType)
	if err != nil || popped != "Boss A" {
		t.Fatalf("PopMetricQueue = %q, err = %v", popped, err)
	}

	remaining, err := store.ListMetricQueue(ctx, guildID, eventType)
	if err != nil {
		t.Fatalf("ListMetricQueue after pop: %v", err)
	}
	if len(remaining) != 2 || remaining[0] != "Boss B" {
		t.Fatalf("after pop: %v", remaining)
	}
}

func TestMetricQueueRemoveAtAndClear(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	guildID := "guild-2"
	if err := store.SaveGuild(ctx, &Guild{GuildID: guildID}); err != nil {
		t.Fatalf("SaveGuild: %v", err)
	}

	for _, name := range []string{"One", "Two", "Three"} {
		if err := store.AppendMetricQueue(ctx, guildID, "sotw", name); err != nil {
			t.Fatalf("AppendMetricQueue: %v", err)
		}
	}

	removed, err := store.RemoveMetricQueueAt(ctx, guildID, "sotw", 2)
	if err != nil || removed != "Two" {
		t.Fatalf("RemoveMetricQueueAt = %q, err = %v", removed, err)
	}

	list, err := store.ListMetricQueue(ctx, guildID, "sotw")
	if err != nil {
		t.Fatalf("ListMetricQueue: %v", err)
	}
	if len(list) != 2 || list[0] != "One" || list[1] != "Three" {
		t.Fatalf("after remove: %v", list)
	}

	n, err := store.ClearMetricQueue(ctx, guildID, "sotw")
	if err != nil || n != 2 {
		t.Fatalf("ClearMetricQueue = %d, err = %v", n, err)
	}

	empty, err := store.PopMetricQueue(ctx, guildID, "sotw")
	if err != nil || empty != "" {
		t.Fatalf("Pop empty queue = %q, err = %v", empty, err)
	}
}

func TestMetricQueueRemoveInvalidPosition(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	if err := store.SaveGuild(ctx, &Guild{GuildID: "g"}); err != nil {
		t.Fatalf("SaveGuild: %v", err)
	}

	if _, err := store.RemoveMetricQueueAt(ctx, "g", "botw", 1); err == nil {
		t.Fatal("expected error removing from empty queue")
	}
}
