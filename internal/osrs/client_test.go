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

const fakeStatsJSON = `{"name":"TestPlayer","skills":[{"id":0,"name":"Overall","rank":1,"level":2277,"xp":200000000}],"activities":[{"id":0,"name":"Zulrah","rank":1,"score":100}]}`

func newTestClient(server *httptest.Server) *Client {
	return &Client{
		httpClient:   server.Client(),
		rateLimiter:  NewRateLimiter(),
		baseURL:      server.URL,
		retryBackoff: 1 * time.Millisecond,
	}
}

func TestGetPlayerStats_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("player"); got != "TestPlayer" {
			t.Errorf("unexpected player query: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, fakeStatsJSON)
	}))
	defer server.Close()

	client := newTestClient(server)
	stats, err := client.GetPlayerStats(context.Background(), "TestPlayer")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if stats == nil {
		t.Fatal("expected stats to be non-nil")
	}
	if stats.Name != "TestPlayer" {
		t.Errorf("expected name TestPlayer, got %q", stats.Name)
	}
	if len(stats.Skills) == 0 {
		t.Error("expected skills to be populated")
	}
	if stats.Skills[0].Name != "Overall" {
		t.Errorf("expected first skill Overall, got %q", stats.Skills[0].Name)
	}
	if len(stats.Activities) == 0 {
		t.Error("expected activities to be populated")
	}
}

func TestGetPlayerStats_NonExistentAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestClient(server)
	stats, err := client.GetPlayerStats(context.Background(), "UnknownPlayer")
	if err == nil {
		t.Fatal("expected error for non-existent account, got nil")
	}
	if stats != nil {
		t.Errorf("expected nil stats, got: %+v", stats)
	}

	var notFoundErr *PlayerNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected PlayerNotFoundError, got: %T - %v", err, err)
	}
	if notFoundErr.RSN != "UnknownPlayer" {
		t.Errorf("expected RSN UnknownPlayer, got %q", notFoundErr.RSN)
	}
}

func TestPlayerExists_ExistingAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, fakeStatsJSON)
	}))
	defer server.Close()

	client := newTestClient(server)
	exists, err := client.PlayerExists(context.Background(), "TestPlayer")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !exists {
		t.Error("expected player to exist")
	}
}

func TestPlayerExists_NonExistentAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestClient(server)
	exists, err := client.PlayerExists(context.Background(), "UnknownPlayer")
	if err != nil {
		t.Fatalf("expected no error for non-existent check, got: %v", err)
	}
	if exists {
		t.Error("expected player to not exist")
	}
}

func TestRateLimiter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, fakeStatsJSON)
	}))
	defer server.Close()

	client := newTestClient(server)
	start := time.Now()
	for i := 0; i < 6; i++ {
		if _, err := client.GetPlayerStats(context.Background(), "TestPlayer"); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}
	elapsed := time.Since(start)

	// Burst is 5; the 6th request should wait for the next token (~1s).
	if elapsed < 800*time.Millisecond {
		t.Errorf("expected rate limiting delay after burst, 6 requests took %v", elapsed)
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
	if got := calls.Load(); got != maxRetries+1 {
		t.Errorf("expected %d HTTP calls, got %d", maxRetries+1, got)
	}
}
