package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"osrs-events/internal/database"
	"osrs-events/internal/firebase"
)

func TestPrepareBotwEvent_UsesQueuedMetric(t *testing.T) {
	ctx := context.Background()
	startTime := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)

	store := &fakeEventStore{
		getActiveEventFn: func(context.Context, string, string) (*database.Event, error) {
			return nil, errNoActiveEvent()
		},
		getAllEventsByGuildAndTypeFn: func(context.Context, string, string) ([]*database.Event, error) {
			return nil, nil
		},
		metricQueues: map[string][]string{
			fakeEventStoreKey("guild1", "botw"): {"Maggot King"},
		},
	}
	config := &fakeOSRSConfigProvider{config: &firebase.OSRSConfig{
		Bosses: []firebase.BossConfig{{
			Name:          "Maggot King",
			BossesToTrack: []string{"Maggot King"},
			PointsPerKC:   1,
			ThresholdKC:   50,
		}},
	}}

	svc := NewEventService(store, nil, config, nil)
	event, metric, fromQueue, err := svc.prepareBotwEvent(ctx, "guild1", startTime)
	if err != nil {
		t.Fatalf("prepareBotwEvent: %v", err)
	}
	if !fromQueue || metric != "Maggot King" || event.MetricJsonID != "Maggot King" {
		t.Fatalf("got metric=%q fromQueue=%v", metric, fromQueue)
	}
	if head, _ := store.PeekMetricQueue(ctx, "guild1", "botw"); head != "Maggot King" {
		t.Fatal("queue head should remain until CreateEvent")
	}
}

func TestCreateEvent_ConsumesQueueOnSuccess(t *testing.T) {
	ctx := context.Background()
	store := &fakeEventStore{
		createEventFn: func(context.Context, *database.Event) error { return nil },
		metricQueues: map[string][]string{
			fakeEventStoreKey("guild1", "botw"): {"Maggot King"},
		},
	}
	svc := NewEventService(store, nil, nil, nil)
	event := &database.Event{GuildID: "guild1", Type: "botw", MetricJsonID: "Maggot King", StartTime: time.Now().UTC()}
	if err := svc.CreateEvent(ctx, event); err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	if head, _ := store.PeekMetricQueue(ctx, "guild1", "botw"); head != "" {
		t.Fatalf("queue should be empty, head=%q", head)
	}
}

func TestCreateEvent_DoesNotConsumeQueueOnFailure(t *testing.T) {
	ctx := context.Background()
	store := &fakeEventStore{
		createEventFn: func(context.Context, *database.Event) error { return errors.New("db down") },
		metricQueues: map[string][]string{
			fakeEventStoreKey("guild1", "sotw"): {"Fletching"},
		},
	}
	svc := NewEventService(store, nil, nil, nil)
	event := &database.Event{GuildID: "guild1", Type: "sotw", MetricJsonID: "Fletching", StartTime: time.Now().UTC()}
	if err := svc.CreateEvent(ctx, event); err == nil {
		t.Fatal("expected error")
	}
	if head, _ := store.PeekMetricQueue(ctx, "guild1", "sotw"); head != "Fletching" {
		t.Fatalf("queue should be unchanged, head=%q", head)
	}
}

func TestPickQueuedMetricName_SkipsStaleHead(t *testing.T) {
	ctx := context.Background()
	store := &fakeEventStore{
		metricQueues: map[string][]string{
			fakeEventStoreKey("guild1", "botw"): {"Removed Boss", "Vorkath"},
		},
	}
	config := &fakeOSRSConfigProvider{config: &firebase.OSRSConfig{
		Bosses: []firebase.BossConfig{{Name: "Vorkath", BossesToTrack: []string{"Vorkath"}, PointsPerKC: 1, ThresholdKC: 1}},
	}}
	svc := NewEventService(store, nil, config, nil)

	boss, ok, err := svc.pickQueuedBossConfig(ctx, "guild1", config.config.Bosses)
	if err != nil || !ok || boss.Name != "Vorkath" {
		t.Fatalf("pickQueuedBossConfig = %v ok=%v err=%v", boss, ok, err)
	}
	remaining, _ := store.ListMetricQueue(ctx, "guild1", "botw")
	if len(remaining) != 1 || remaining[0] != "Vorkath" {
		t.Fatalf("stale head popped during pick, remaining=%v", remaining)
	}
}
