package services

import (
	"sync"
	"testing"
	"time"

	"osrs-events/internal/database"
)

func TestGuildWorkDebouncer_CoalescesBurstWork(t *testing.T) {
	var (
		mu    sync.Mutex
		calls []string
	)

	debouncer := newGuildWorkDebouncer(50*time.Millisecond, func(guildID string) {
		mu.Lock()
		calls = append(calls, guildID)
		mu.Unlock()
	})

	debouncer.schedule("g1")
	debouncer.schedule("g1")
	debouncer.schedule("g1")

	time.Sleep(120 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 1 {
		t.Fatalf("expected 1 debounced call, got %d (%v)", len(calls), calls)
	}
	if calls[0] != "g1" {
		t.Fatalf("unexpected guild id: %q", calls[0])
	}
}

func TestGuildWorkDebouncer_RefreshesAgainAfterQuietPeriod(t *testing.T) {
	var (
		mu    sync.Mutex
		calls int
	)

	debouncer := newGuildWorkDebouncer(30*time.Millisecond, func(string) {
		mu.Lock()
		calls++
		mu.Unlock()
	})

	debouncer.schedule("g1")
	time.Sleep(80 * time.Millisecond)

	debouncer.schedule("g1")
	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Fatalf("expected 2 debounced calls after quiet periods, got %d", calls)
	}
}

func TestIsPBSubmissionAlreadyReviewed(t *testing.T) {
	if !IsPBSubmissionAlreadyReviewed(database.ErrPBSubmissionNotPending) {
		t.Fatal("expected ErrPBSubmissionNotPending to match already reviewed helper")
	}
}
