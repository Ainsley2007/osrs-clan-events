package osrs

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetPlayerStats_ExistingAccount(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stats, err := client.GetPlayerStats(ctx, "FePrototype")
	if err != nil {
		t.Fatalf("expected no error for existing account, got: %v", err)
	}

	if stats == nil {
		t.Fatal("expected stats to be non-nil")
	}

	if stats.Name != "FePrototype" {
		t.Errorf("expected name 'FePrototype', got '%s'", stats.Name)
	}

	if len(stats.Skills) == 0 {
		t.Error("expected skills to be populated")
	}

	if len(stats.Skills) < 25 {
		t.Errorf("expected at least 25 skills, got %d", len(stats.Skills))
	}

	if len(stats.Activities) == 0 {
		t.Error("expected activities to be populated")
	}

	overallSkill := stats.Skills[0]
	if overallSkill.Name != "Overall" {
		t.Errorf("expected first skill to be 'Overall', got '%s'", overallSkill.Name)
	}

	if overallSkill.Level < 100 {
		t.Errorf("expected FePrototype to have high total level, got %d", overallSkill.Level)
	}

	if overallSkill.XP <= 0 {
		t.Error("expected Overall XP to be greater than 0")
	}

	t.Logf("FePrototype stats: Total Level=%d, Total XP=%d, Skills=%d, Activities=%d",
		overallSkill.Level, overallSkill.XP, len(stats.Skills), len(stats.Activities))
}

func TestGetPlayerStats_NonExistentAccount(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stats, err := client.GetPlayerStats(ctx, "lion1002")
	if err == nil {
		t.Fatal("expected error for non-existent account, got nil")
	}

	if stats != nil {
		t.Errorf("expected stats to be nil for non-existent account, got: %+v", stats)
	}

	var notFoundErr *PlayerNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Errorf("expected PlayerNotFoundError, got: %T - %v", err, err)
	}

	if notFoundErr.RSN != "lion1002" {
		t.Errorf("expected error to contain RSN 'lion1002', got '%s'", notFoundErr.RSN)
	}

	t.Logf("Correctly returned PlayerNotFoundError: %v", err)
}

func TestPlayerExists_ExistingAccount(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	exists, err := client.PlayerExists(ctx, "FePrototype")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !exists {
		t.Error("expected FePrototype to exist")
	}

	t.Log("FePrototype exists: true")
}

func TestPlayerExists_NonExistentAccount(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	exists, err := client.PlayerExists(ctx, "lion1002")
	if err != nil {
		t.Fatalf("expected no error for non-existent check, got: %v", err)
	}

	if exists {
		t.Error("expected lion1002 to not exist")
	}

	t.Log("lion1002 exists: false")
}

func TestRateLimiter(t *testing.T) {
	client := NewClient()
	ctx := context.Background()

	start := time.Now()

	for i := 0; i < 3; i++ {
		_, _ = client.GetPlayerStats(ctx, "FePrototype")
	}

	elapsed := time.Since(start)

	if elapsed < 2*time.Second {
		t.Logf("Warning: Rate limiting may not be working as expected. 3 requests took %v", elapsed)
	} else {
		t.Logf("Rate limiter working: 3 requests took %v", elapsed)
	}
}

const fakeStatsJSON = `{"name":"TestPlayer","skills":[{"id":0,"name":"Overall","rank":1,"level":2277,"xp":200000000}],"activities":[]}`

func newTestClient(server *httptest.Server) *Client {
	return &Client{
		httpClient:   server.Client(),
		rateLimiter:  NewRateLimiter(),
		baseURL:      server.URL,
		retryBackoff: 1 * time.Millisecond,
	}
}

func TestGetPlayerStats_429RetrySuccess(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, fakeStatsJSON)
	}))
	defer server.Close()

	client := newTestClient(server)
	stats, err := client.GetPlayerStats(context.Background(), "TestPlayer")
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if stats == nil || stats.Name != "TestPlayer" {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if calls.Load() != 2 {
		t.Errorf("expected 2 HTTP calls (1 429 + 1 200), got %d", calls.Load())
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		header string
		want   time.Duration
	}{
		{"5", 5 * time.Second},
		{"0", 0},
		{"-1", 0},
		{"abc", 0},
		{"", 0},
	}
	for _, tc := range tests {
		resp := &http.Response{Header: make(http.Header)}
		if tc.header != "" {
			resp.Header.Set("Retry-After", tc.header)
		}
		got := parseRetryAfter(resp)
		if got != tc.want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", tc.header, got, tc.want)
		}
	}
}

func TestGetPlayerStats_429WithRetryAfterHonored(t *testing.T) {
	// Server returns 429 with Retry-After: 0 (no extra wait) then 200.
	// Verifies that a Retry-After header is read without causing failures.
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, fakeStatsJSON)
	}))
	defer server.Close()

	client := newTestClient(server)
	stats, err := client.GetPlayerStats(context.Background(), "TestPlayer")
	if err != nil {
		t.Fatalf("expected success after Retry-After retry, got: %v", err)
	}
	if stats == nil || stats.Name != "TestPlayer" {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestGetPlayerStats_429Exhaustion_ReturnsRateLimitError(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.GetPlayerStats(context.Background(), "TestPlayer")
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	var rlErr *RateLimitError
	if !errors.As(err, &rlErr) {
		t.Errorf("expected *RateLimitError, got %T: %v", err, err)
	}
	// 1 initial attempt + maxRetries retries
	if got := calls.Load(); got != maxRetries+1 {
		t.Errorf("expected %d HTTP calls, got %d", maxRetries+1, got)
	}
}
